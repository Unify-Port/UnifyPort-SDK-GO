package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"sort"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
)

func buildRequestArguments(source operationSource, goMethod string, requestBody *bodyManifest, wire wireIndex) ([]requestArgumentManifest, error) {
	wireArguments, ok := wire.RequestArguments[goMethod]
	if !ok {
		return nil, fmt.Errorf("wire request arguments missing for %s", goMethod)
	}
	result := make([]requestArgumentManifest, 0, len(wireArguments))
	for _, argument := range wireArguments {
		item := requestArgumentManifest{
			Field:    exportIdentifier(argument.Name),
			WireName: argument.Name,
			WireType: argument.Type,
		}
		switch argument.Name {
		case "body":
			if requestBody == nil {
				return nil, errors.New("wire has body argument but contract requestBody is missing")
			}
			item.Kind = "body"
			item.Required = requestBody.Required
			item.GoType = goMethod + "RequestBody"
			if !item.Required {
				item.GoType = "*" + item.GoType
			}
		case "params":
			item.Kind = "params"
			item.Required = hasRequiredNonPathParameter(source.PathItem, source.Operation)
			item.GoType = goMethod + "Params"
			if !item.Required {
				item.GoType = "*" + item.GoType
			}
		default:
			item.Kind = "path"
			item.Required = true
			item.GoType = strings.TrimPrefix(argument.Type, "*")
		}
		result = append(result, item)
	}
	return result, nil
}

func hasRequiredNonPathParameter(pathItem *openapi3.PathItem, operation *openapi3.Operation) bool {
	parameters := append(append(openapi3.Parameters{}, pathItem.Parameters...), operation.Parameters...)
	for _, parameterRef := range parameters {
		if parameterRef == nil || parameterRef.Value == nil {
			continue
		}
		parameter := parameterRef.Value
		if parameter.In != openapi3.ParameterInPath && parameter.Required {
			return true
		}
	}
	return false
}

func buildDependentRequirements(operation *openapi3.Operation, arguments []requestArgumentManifest, wire wireIndex) ([]dependentRequirement, error) {
	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return nil, nil
	}
	mediaType := operation.RequestBody.Value.Content["application/json"]
	if mediaType == nil || mediaType.Schema == nil {
		return nil, nil
	}

	dependencies, err := collectDependentRequired(mediaType.Schema)
	if err != nil {
		return nil, err
	}
	if len(dependencies) == 0 {
		return nil, nil
	}
	bodyWireType := ""
	for _, argument := range arguments {
		if argument.Kind == "body" {
			bodyWireType = baseNamedType(argument.WireType)
			break
		}
	}
	if bodyWireType == "" {
		return nil, errors.New("dependentRequired exists but the wire body argument is missing")
	}
	fields, err := resolveWireJSONFields(bodyWireType, wire, make(map[string]struct{}))
	if err != nil {
		return nil, err
	}

	result := make([]dependentRequirement, 0)
	for _, sourceJSONField := range sortedKeys(dependencies) {
		sourceField, ok := fields[sourceJSONField]
		if !ok {
			return nil, fmt.Errorf("dependentRequired source %q has no wire JSON field on %s", sourceJSONField, bodyWireType)
		}
		if !sourceField.Pointer {
			return nil, fmt.Errorf("dependentRequired source %q on %s is not presence-aware", sourceJSONField, bodyWireType)
		}
		for _, requiredJSONField := range sortedKeys(dependencies[sourceJSONField]) {
			requiredField, exists := fields[requiredJSONField]
			if !exists {
				return nil, fmt.Errorf("dependentRequired target %q has no wire JSON field on %s", requiredJSONField, bodyWireType)
			}
			if !requiredField.Pointer {
				return nil, fmt.Errorf("dependentRequired target %q on %s is not presence-aware", requiredJSONField, bodyWireType)
			}
			result = append(result, dependentRequirement{
				IfJSONField:       sourceJSONField,
				IfGoField:         sourceField.GoName,
				RequiresJSONField: requiredJSONField,
				RequiresGoField:   requiredField.GoName,
			})
		}
	}
	return result, nil
}

func collectDependentRequired(root *openapi3.SchemaRef) (map[string]map[string]struct{}, error) {
	result := make(map[string]map[string]struct{})
	type visitKey struct {
		schema      *openapi3.Schema
		conditional bool
	}
	seen := make(map[visitKey]struct{})
	var visit func(*openapi3.SchemaRef, bool) error
	visit = func(ref *openapi3.SchemaRef, conditional bool) error {
		if ref == nil || ref.Value == nil {
			return nil
		}
		schema := ref.Value
		key := visitKey{schema: schema, conditional: conditional}
		if _, exists := seen[key]; exists {
			return nil
		}
		seen[key] = struct{}{}
		if conditional && len(schema.DependentRequired) > 0 {
			return errors.New("dependentRequired inside oneOf/anyOf is branch-conditional and cannot become an unconditional facade guard")
		}
		for source, requirements := range schema.DependentRequired {
			if result[source] == nil {
				result[source] = make(map[string]struct{})
			}
			for _, requirement := range requirements {
				result[source][requirement] = struct{}{}
			}
		}
		// allOf 规则无条件继承；oneOf/anyOf 分支规则无法提升为 facade 的无条件 guard。
		for _, child := range schema.AllOf {
			if err := visit(child, conditional); err != nil {
				return err
			}
		}
		for _, child := range append(append(openapi3.SchemaRefs{}, schema.OneOf...), schema.AnyOf...) {
			if err := visit(child, true); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root, false); err != nil {
		return nil, err
	}
	return result, nil
}

func resolveWireJSONFields(typeName string, wire wireIndex, seen map[string]struct{}) (map[string]wireJSONField, error) {
	if _, duplicate := seen[typeName]; duplicate {
		return nil, fmt.Errorf("wire type alias cycle at %q", typeName)
	}
	seen[typeName] = struct{}{}
	expression, ok := wire.TypeDeclarations[typeName]
	if !ok {
		return nil, fmt.Errorf("wire type declaration missing for %q", typeName)
	}
	switch typed := expression.(type) {
	case *ast.StructType:
		return wire.JSONFields[typeName], nil
	case *ast.Ident:
		return resolveWireJSONFields(typed.Name, wire, seen)
	default:
		return nil, fmt.Errorf("wire body type %q does not resolve to a struct", typeName)
	}
}

func successDataGoType(operation *openapi3.Operation, wire wireIndex) (string, error) {
	if operation.Responses == nil {
		return "", errors.New("responses are missing")
	}
	dataType := ""
	hasEmpty := false
	for _, status := range operation.Responses.Keys() {
		if !isSuccessStatus(status) {
			continue
		}
		responseRef := operation.Responses.Value(status)
		if responseRef == nil || responseRef.Value == nil {
			return "", fmt.Errorf("success response %q is unresolved", status)
		}
		current := ""
		if len(responseRef.Value.Content) == 0 {
			hasEmpty = true
			continue
		}
		if len(responseRef.Value.Content) > 0 {
			mediaType := responseRef.Value.Content["application/json"]
			if mediaType == nil || mediaType.Schema == nil {
				return "", fmt.Errorf("success response %q must use application/json", status)
			}
			dataSchema, err := findEnvelopeDataSchema(mediaType.Schema)
			if err != nil {
				return "", fmt.Errorf("success response %q: %w", status, err)
			}
			current, err = schemaGoType(dataSchema, wire)
			if err != nil {
				current, err = responseDataWireType(operation.OperationID, status, wire)
			}
			if err != nil {
				return "", fmt.Errorf("success response %q data: %w", status, err)
			}
		}
		if dataType != "" && dataType != current {
			return "", fmt.Errorf("success responses use incompatible data types %q and %q", dataType, current)
		}
		dataType = current
	}
	if dataType == "" && !hasEmpty {
		return "", errors.New("no success response data type")
	}
	// 混合 202/204 用指针保留空响应，不能把异步删除误报为已完成。
	if hasEmpty && dataType != "" {
		return "*" + dataType, nil
	}
	if dataType == "" {
		return "NoContent", nil
	}
	return dataType, nil
}

func findEnvelopeDataSchema(root *openapi3.SchemaRef) (*openapi3.SchemaRef, error) {
	seen := make(map[*openapi3.SchemaRef]struct{})
	found := make([]*openapi3.SchemaRef, 0, 1)
	var visit func(*openapi3.SchemaRef)
	visit = func(ref *openapi3.SchemaRef) {
		if ref == nil || ref.Value == nil {
			return
		}
		if _, exists := seen[ref]; exists {
			return
		}
		seen[ref] = struct{}{}
		if data, ok := ref.Value.Properties["data"]; ok {
			found = append(found, data)
		}
		for _, child := range append(append(append(openapi3.SchemaRefs{}, ref.Value.AllOf...), ref.Value.OneOf...), ref.Value.AnyOf...) {
			visit(child)
		}
	}
	visit(root)
	if len(found) != 1 {
		return nil, fmt.Errorf("response envelope must define exactly one data property, found %d", len(found))
	}
	return found[0], nil
}

func schemaGoType(ref *openapi3.SchemaRef, wire wireIndex) (string, error) {
	if ref == nil {
		return "", errors.New("schema is missing")
	}
	if schemaName := localSchemaName(ref.Ref); schemaName != "" {
		goType, ok := wire.SchemaTypes[schemaName]
		if !ok {
			return "", fmt.Errorf("component schema %q has no Go type", schemaName)
		}
		return goType, nil
	}
	if ref.Value == nil || ref.Value.Type == nil {
		return "", errors.New("inline data schema has no supported type")
	}
	schema := ref.Value
	switch {
	case schema.Type.Is("array"):
		itemType, err := schemaGoType(schema.Items, wire)
		if err != nil {
			return "", err
		}
		return "[]" + itemType, nil
	case schema.Type.Is("boolean"):
		return "bool", nil
	case schema.Type.Is("integer"):
		switch schema.Format {
		case "int32":
			return "int32", nil
		case "int64":
			return "int64", nil
		default:
			return "int", nil
		}
	case schema.Type.Is("number"):
		if schema.Format == "float" {
			return "float32", nil
		}
		return "float64", nil
	case schema.Type.Is("string"):
		if schema.Format != "" {
			return "", fmt.Errorf("string format %q needs an explicit public mapping", schema.Format)
		}
		return "string", nil
	case schema.Type.Is("object"):
		return "map[string]interface{}", nil
	default:
		return "", fmt.Errorf("unsupported inline data schema type %v", schema.Type.Slice())
	}
}

func generateModels(value *manifest) ([]byte, error) {
	aliases := make(map[string]string)
	queue := make([]string, 0)
	addAlias := func(wireType, publicType string) error {
		if wireType == "" || publicType == "" {
			return errors.New("empty model alias")
		}
		if previous, exists := aliases[wireType]; exists && previous != publicType {
			return fmt.Errorf("wire type %q maps to both %q and %q", wireType, previous, publicType)
		}
		if _, exists := aliases[wireType]; !exists {
			queue = append(queue, wireType)
		}
		aliases[wireType] = publicType
		return nil
	}

	for _, schema := range value.Schemas {
		if err := addAlias(schema.GoType, schema.GoType); err != nil {
			return nil, err
		}
	}
	for _, operation := range value.Operations {
		if name := baseNamedType(operation.DataGoType); name != "NoContent" {
			if _, exists := value.wire.TypeDeclarations[name]; exists {
				if err := addAlias(name, name); err != nil {
					return nil, err
				}
			}
		}
		for _, argument := range operation.RequestArguments {
			wireType := baseNamedType(argument.WireType)
			switch argument.Kind {
			case "body":
				if err := addAlias(wireType, operation.GoMethod+"RequestBody"); err != nil {
					return nil, err
				}
			case "params", "path":
				// 内置类型不属于 wire 声明，不能生成遮蔽 string 等预声明标识符的 alias。
				if _, exists := value.wire.TypeDeclarations[wireType]; !exists {
					continue
				}
				if err := addAlias(wireType, wireType); err != nil {
					return nil, err
				}
			}
		}
	}

	// 从公开根类型递归补齐字段依赖，避免 alias 的字段类型落回不可导入的 internal/wire。
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, dependency := range value.wire.TypeDependencies[current] {
			if _, exists := aliases[dependency]; exists {
				continue
			}
			if err := addAlias(dependency, dependency); err != nil {
				return nil, err
			}
		}
	}

	publicOwners := make(map[string]string, len(aliases))
	for wireType, publicType := range aliases {
		if previous, duplicate := publicOwners[publicType]; duplicate && previous != wireType {
			return nil, fmt.Errorf("public type %q maps to both wire types %q and %q", publicType, previous, wireType)
		}
		publicOwners[publicType] = wireType
	}
	publicTypes := make([]string, 0, len(publicOwners))
	for publicType := range publicOwners {
		publicTypes = append(publicTypes, publicType)
	}
	sort.Strings(publicTypes)

	var output strings.Builder
	output.WriteString("// Code generated by go generate; DO NOT EDIT.\n")
	output.WriteString("// 公开 alias 保持契约模型与 internal wire 完全一致，同时不暴露 raw client。\n\n")
	output.WriteString("package unifyport\n\n")
	output.WriteString("import wire \"github.com/Unify-Port/UnifyPort-SDK-GO/internal/wire\"\n\n")
	for _, publicType := range publicTypes {
		wireType := publicOwners[publicType]
		fmt.Fprintf(&output, "// %s 是由公开 OpenAPI 契约生成的类型别名。\n", publicType)
		fmt.Fprintf(&output, "type %s = wire.%s\n\n", publicType, wireType)
	}

	constantOwners := make(map[string]enumConstant)
	for _, constant := range value.wire.EnumConstants {
		publicType, exposed := aliases[constant.TypeName]
		if !exposed {
			continue
		}
		publicName := publicEnumConstantName(publicType, constant.TypeName, constant.Name)
		if previous, duplicate := constantOwners[publicName]; duplicate && previous != constant {
			return nil, fmt.Errorf("public enum constant %q is ambiguous", publicName)
		}
		constantOwners[publicName] = constant
	}
	constantNames := make([]string, 0, len(constantOwners))
	for name := range constantOwners {
		constantNames = append(constantNames, name)
	}
	sort.Strings(constantNames)
	if len(constantNames) > 0 {
		output.WriteString("const (\n")
		for _, publicName := range constantNames {
			constant := constantOwners[publicName]
			publicType := aliases[constant.TypeName]
			fmt.Fprintf(&output, "\t// %s 是 %s 的受支持枚举值。\n", publicName, publicType)
			fmt.Fprintf(&output, "\t%s %s = wire.%s\n", publicName, publicType, constant.Name)
		}
		output.WriteString(")\n")
	}
	return format.Source([]byte(output.String()))
}

func generateOperations(value *manifest) ([]byte, error) {
	operationIDs := make([]string, 0, len(value.Operations))
	for operationID := range value.Operations {
		operationIDs = append(operationIDs, operationID)
	}
	sort.Strings(operationIDs)

	var output strings.Builder
	output.WriteString("// Code generated by go generate; DO NOT EDIT.\n")
	output.WriteString("// operation metadata、request builder 与 response parser 共用同一批准契约和安全策略。\n\n")
	output.WriteString("package unifyport\n\n")
	output.WriteString("import (\n")
	output.WriteString("\t\"context\"\n")
	output.WriteString("\t\"errors\"\n")
	output.WriteString("\t\"net/http\"\n\n")
	output.WriteString("\twire \"github.com/Unify-Port/UnifyPort-SDK-GO/internal/wire\"\n")
	output.WriteString(")\n\n")
	output.WriteString("// Mutability 描述 operation 对远端资源的影响。\n")
	output.WriteString("type Mutability string\n\n")
	output.WriteString("const (\n")
	output.WriteString("\t// MutabilityRead 表示只读 operation。\n\tMutabilityRead Mutability = \"read\"\n")
	output.WriteString("\t// MutabilityWrite 表示会改变远端状态的 operation。\n\tMutabilityWrite Mutability = \"write\"\n")
	output.WriteString("\t// MutabilityDestructive 表示删除、撤回或移除类 operation。\n\tMutabilityDestructive Mutability = \"destructive\"\n")
	output.WriteString(")\n\n")
	output.WriteString("// MCPExposure 记录 Node 基线对模型工具面的显式审核结论，不包含任何 MCP schema。\n")
	output.WriteString("type MCPExposure string\n\n")
	output.WriteString("const (\n")
	output.WriteString("\t// MCPExposureNever 表示 operation 不进入模型工具面。\n\tMCPExposureNever MCPExposure = \"never\"\n")
	output.WriteString("\t// MCPExposureRead 表示只读模型工具暴露。\n\tMCPExposureRead MCPExposure = \"read\"\n")
	output.WriteString("\t// MCPExposureWrite 表示写入模型工具暴露。\n\tMCPExposureWrite MCPExposure = \"write\"\n")
	output.WriteString("\t// MCPExposureDestructive 表示破坏性模型工具暴露。\n\tMCPExposureDestructive MCPExposure = \"destructive\"\n")
	output.WriteString(")\n\n")
	output.WriteString("// Operation 是公开、只读语义的 operation metadata 值。\n")
	output.WriteString("type Operation struct {\n")
	output.WriteString("\tID string\n\tMethod string\n\tPath string\n\tTag string\n\tSummary string\n")
	output.WriteString("\tMutability Mutability\n\tRetryable bool\n\tSecretInput bool\n\tSecretOutput bool\n\tMCPExposure MCPExposure\n")
	output.WriteString("}\n\n")
	output.WriteString("var errUnexpectedSuccessResponse = errors.New(\"unexpected successful response\")\n\n")

	for _, operationID := range operationIDs {
		operation := value.Operations[operationID]
		fmt.Fprintf(&output, "var operation%s = Operation{\n", operation.GoMethod)
		fmt.Fprintf(&output, "\tID: %q, Method: %q, Path: %q, Tag: %q, Summary: %q,\n", operation.OperationID, operation.Method, operation.Path, operation.Tag, operation.Summary)
		fmt.Fprintf(&output, "\tMutability: Mutability%s, Retryable: %t, SecretInput: %t, SecretOutput: %t, MCPExposure: MCPExposure%s,\n", exportIdentifier(operation.Mutability), operation.Retryable, operation.SecretInput, operation.SecretOutput, exportIdentifier(operation.MCPExposure))
		output.WriteString("}\n\n")
	}
	output.WriteString("var operationCatalog = [...]Operation{\n")
	for _, operationID := range operationIDs {
		fmt.Fprintf(&output, "\toperation%s,\n", value.Operations[operationID].GoMethod)
	}
	output.WriteString("}\n\n")
	output.WriteString("var operationIndex = func() map[string]Operation {\n")
	output.WriteString("\tresult := make(map[string]Operation, len(operationCatalog))\n")
	output.WriteString("\tfor _, operation := range operationCatalog { result[operation.ID] = operation }\n")
	output.WriteString("\treturn result\n")
	output.WriteString("}()\n\n")
	output.WriteString("// Operations 返回 operation metadata 的独立切片副本。\n")
	output.WriteString("func Operations() []Operation { return append([]Operation(nil), operationCatalog[:]...) }\n\n")
	output.WriteString("// LookupOperation 按原始 operationId 返回 metadata 值。\n")
	output.WriteString("func LookupOperation(id string) (Operation, bool) { operation, ok := operationIndex[id]; return operation, ok }\n\n")

	for _, operationID := range operationIDs {
		operation := value.Operations[operationID]
		writeOperationFacade(&output, operation)
	}
	return format.Source([]byte(output.String()))
}

func writeOperationFacade(output *strings.Builder, operation operationManifest) {
	if operation.InputGoType != "" {
		fmt.Fprintf(output, "// %s 汇集 %s 所需的 path、query/header 与 body 输入。\n", operation.InputGoType, operation.GoMethod)
		fmt.Fprintf(output, "type %s struct {\n", operation.InputGoType)
		for _, argument := range operation.RequestArguments {
			switch argument.Kind {
			case "path":
				fmt.Fprintf(output, "\t// %s 对应必填 path parameter。\n", argument.Field)
			case "params":
				fmt.Fprintf(output, "\t// Params 保留生成器对 query/header 参数的 required 约束。\n")
			case "body":
				fmt.Fprintf(output, "\t// Body 对应 application/json request body。\n")
			}
			fmt.Fprintf(output, "\t%s %s\n", argument.Field, argument.GoType)
		}
		output.WriteString("}\n\n")
	}

	summary := strings.Join(strings.Fields(operation.Summary), " ")
	fmt.Fprintf(output, "// %s %s。\n", operation.GoMethod, strings.TrimSuffix(summary, "。"))
	fmt.Fprintf(output, "func (c *Client) %s(ctx context.Context", operation.GoMethod)
	if operation.InputGoType != "" {
		fmt.Fprintf(output, ", input %s", operation.InputGoType)
	}
	output.WriteString(", options ...CallOption) (Result[")
	output.WriteString(operation.DataGoType)
	output.WriteString("], error) {\n")
	writeDependentRequirementGuards(output, operation)
	fmt.Fprintf(output, "\treturn executeOperation(ctx, c, operation%s, func(baseURL string) (*http.Request, error) {\n", operation.GoMethod)
	writeRequestBuilder(output, operation)
	fmt.Fprintf(output, "\t}, parse%sResponse, options...)\n", operation.GoMethod)
	output.WriteString("}\n\n")

	fmt.Fprintf(output, "func parse%sResponse(response *http.Response) (operationData[%s], error) {\n", operation.GoMethod, operation.DataGoType)
	fmt.Fprintf(output, "\tvar zero operationData[%s]\n", operation.DataGoType)
	fmt.Fprintf(output, "\tparsed, err := wire.Parse%sResponse(response)\n", operation.GoMethod)
	output.WriteString("\tif err != nil { return zero, err }\n")
	for _, status := range operation.SuccessStatuses {
		if operation.HasSuccessContent && !containsStatus(operation.NoContentStatuses, status) {
			fmt.Fprintf(output, "\tif parsed.JSON%s != nil {\n", status)
			fmt.Fprintf(output, "\t\tpayload := parsed.JSON%s\n", status)
			dataExpression := "payload.Data"
			if strings.HasPrefix(operation.DataGoType, "*") {
				dataExpression = "&payload.Data"
			}
			fmt.Fprintf(output, "\t\treturn operationData[%s]{Data: %s, RequestID: payload.RequestID, ClientRequestID: payload.ClientRequestID}, nil\n", operation.DataGoType, dataExpression)
			output.WriteString("\t}\n")
		} else {
			if operation.DataGoType == "NoContent" {
				fmt.Fprintf(output, "\tif parsed.StatusCode() == %s { return operationData[NoContent]{Data: NoContent{}}, nil }\n", status)
			} else {
				fmt.Fprintf(output, "\tif parsed.StatusCode() == %s { return operationData[%s]{}, nil }\n", status, operation.DataGoType)
			}
		}
	}
	output.WriteString("\treturn zero, errUnexpectedSuccessResponse\n")
	output.WriteString("}\n\n")
}

func writeDependentRequirementGuards(output *strings.Builder, operation operationManifest) {
	if len(operation.DependentRequirements) == 0 {
		return
	}
	bodyOptional := operation.RequestBody != nil && !operation.RequestBody.Required
	output.WriteString("\t// OpenAPI dependentRequired 必须先于 credential 与网络访问执行，避免无效请求触发外部副作用。\n")
	for _, requirement := range operation.DependentRequirements {
		condition := fmt.Sprintf("input.Body.%s != nil && input.Body.%s == nil", requirement.IfGoField, requirement.RequiresGoField)
		if bodyOptional {
			condition = "input.Body != nil && " + condition
		}
		fmt.Fprintf(output, "\tif %s {\n", condition)
		fmt.Fprintf(output, "\t\treturn Result[%s]{}, &ConfigurationError{Field: %q, Reason: %q}\n",
			operation.DataGoType,
			operation.InputGoType+".Body",
			requirement.RequiresJSONField+" is required when "+requirement.IfJSONField+" is provided",
		)
		output.WriteString("\t}\n")
	}
}

func writeRequestBuilder(output *strings.Builder, operation operationManifest) {
	optionalBody := false
	for _, argument := range operation.RequestArguments {
		if argument.Kind == "body" && !argument.Required {
			optionalBody = true
			break
		}
	}
	if optionalBody {
		output.WriteString("\t\tif input.Body == nil {\n")
		fmt.Fprintf(output, "\t\t\treturn wire.New%sRequestWithBody(baseURL", operation.GoMethod)
		for _, argument := range operation.RequestArguments {
			if argument.Kind == "body" {
				continue
			}
			fmt.Fprintf(output, ", %s", requestArgumentExpression(argument))
		}
		output.WriteString(", \"application/json\", nil)\n")
		output.WriteString("\t\t}\n")
	}
	fmt.Fprintf(output, "\t\treturn wire.New%sRequest(baseURL", operation.GoMethod)
	for _, argument := range operation.RequestArguments {
		expression := requestArgumentExpression(argument)
		if argument.Kind == "body" && !argument.Required {
			expression = "*input." + argument.Field
		}
		fmt.Fprintf(output, ", %s", expression)
	}
	output.WriteString(")\n")
}

func requestArgumentExpression(argument requestArgumentManifest) string {
	if argument.Kind == "params" && argument.Required {
		return "&input." + argument.Field
	}
	return "input." + argument.Field
}

func baseNamedType(expression string) string {
	return strings.TrimLeft(expression, "*[]")
}

func exportIdentifier(value string) string {
	if value == "" {
		return ""
	}
	characters := []rune(value)
	characters[0] = unicode.ToUpper(characters[0])
	return string(characters)
}

func publicEnumConstantName(publicType, wireType, wireName string) string {
	if strings.HasPrefix(wireName, wireType) {
		return wireName
	}
	return publicType + wireName
}

// 每个成功状态独立判断内容，防止混合响应引用不存在的 JSON204 字段。
func noContentStatuses(operation *openapi3.Operation) []string {
	var statuses []string
	for _, status := range operation.Responses.Keys() {
		if isSuccessStatus(status) && len(operation.Responses.Value(status).Value.Content) == 0 {
			statuses = append(statuses, status)
		}
	}
	sort.Strings(statuses)
	return statuses
}
func containsStatus(statuses []string, value string) bool {
	for _, status := range statuses {
		if status == value {
			return true
		}
	}
	return false
}

// 内联 union 使用生成器真实标识符，不根据命名惯例猜测或降级成 map。
func responseDataWireType(operationID, status string, wire wireIndex) (string, error) {
	var resolve func(ast.Expr) ast.Expr
	resolve = func(expression ast.Expr) ast.Expr {
		switch typed := expression.(type) {
		case *ast.StarExpr:
			return resolve(typed.X)
		case *ast.Ident:
			if next, ok := wire.TypeDeclarations[typed.Name]; ok {
				return resolve(next)
			}
		}
		return expression
	}
	response, ok := resolve(wire.TypeDeclarations[wire.GoMethods[foldIdentifier(operationID)]+"Response"]).(*ast.StructType)
	if !ok {
		return "", errors.New("response type is not a struct")
	}
	for _, field := range response.Fields.List {
		if len(field.Names) == 1 && field.Names[0].Name == "JSON"+status {
			payload, ok := resolve(field.Type).(*ast.StructType)
			if !ok {
				break
			}
			for _, data := range payload.Fields.List {
				if len(data.Names) == 1 && data.Names[0].Name == "Data" {
					var buffer bytes.Buffer
					if err := printer.Fprint(&buffer, token.NewFileSet(), data.Type); err != nil {
						return "", err
					}
					return buffer.String(), nil
				}
			}
		}
	}
	return "", errors.New("response data wire type is missing")
}
