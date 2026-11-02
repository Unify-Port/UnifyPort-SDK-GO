// Command generator 根据批准契约、显式策略和 wire 生成物构建 parity manifest。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	manifestRelativePath   = "parity/manifest.gen.json"
	modelsRelativePath     = "models.gen.go"
	operationsRelativePath = "operations.gen.go"
	overlayRelativePath    = "codegen/go.overlay.yaml"
	policyRelativePath     = "policy/operation-policies.json"
	sourceRelativePath     = "parity/source.json"
	wireRelativePath       = "internal/wire/device.gen.go"
)

var modelCommentPattern = regexp.MustCompile(`(?m)^// ([A-Za-z_][A-Za-z0-9_]*) defines model for ([A-Za-z_][A-Za-z0-9_]*)\.$`)

type sourceFile struct {
	SchemaVersion int          `json:"schemaVersion"`
	NodeSDK       nodeSDK      `json:"nodeSdk"`
	Contract      contractInfo `json:"contract"`
}

type nodeSDK struct {
	Package      string `json:"package"`
	Version      string `json:"version"`
	Repository   string `json:"repository"`
	Commit       string `json:"commit"`
	BaselineKind string `json:"baselineKind,omitempty"`
}

type contractInfo struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type policyFile struct {
	SchemaVersion       int               `json:"schemaVersion"`
	Defaults            policyDefaults    `json:"defaults"`
	RetryAllowlist      []string          `json:"retryAllowlist"`
	MutabilityOverrides map[string]string `json:"mutabilityOverrides"`
	SecretInput         []string          `json:"secretInput"`
	SecretOutput        []string          `json:"secretOutput"`
	MCPExposure         map[string]string `json:"mcpExposure"`
}

type policyDefaults struct {
	Retryable    bool   `json:"retryable"`
	SecretInput  bool   `json:"secretInput"`
	SecretOutput bool   `json:"secretOutput"`
	MCPExposure  string `json:"mcpExposure"`
}

type manifest struct {
	SchemaVersion  int                          `json:"schemaVersion"`
	NodeSDK        nodeSDK                      `json:"nodeSdk"`
	ContractSHA256 string                       `json:"contractSha256"`
	OperationCount int                          `json:"operationCount"`
	SchemaCount    int                          `json:"schemaCount"`
	PolicyCounts   policyCounts                 `json:"policyCounts"`
	Operations     map[string]operationManifest `json:"operations"`
	Schemas        map[string]schemaManifest    `json:"schemas"`
	wire           wireIndex
}

type policyCounts struct {
	Retryable    int `json:"retryable"`
	Destructive  int `json:"destructive"`
	SecretInput  int `json:"secretInput"`
	SecretOutput int `json:"secretOutput"`
}

type operationManifest struct {
	OperationID           string                    `json:"operationId"`
	GoMethod              string                    `json:"goMethod"`
	RequestFunction       string                    `json:"requestFunction"`
	ResponseType          string                    `json:"responseType"`
	ParseResponseFunction string                    `json:"parseResponseFunction"`
	Method                string                    `json:"method"`
	Path                  string                    `json:"path"`
	Tag                   string                    `json:"tag"`
	Summary               string                    `json:"summary"`
	AuthRequired          bool                      `json:"authRequired"`
	Mutability            string                    `json:"mutability"`
	Retryable             bool                      `json:"retryable"`
	SecretInput           bool                      `json:"secretInput"`
	SecretOutput          bool                      `json:"secretOutput"`
	MCPExposure           string                    `json:"mcpExposure"`
	Parameters            []parameterManifest       `json:"parameters"`
	ParamsGoType          string                    `json:"paramsGoType,omitempty"`
	RequestBody           *bodyManifest             `json:"requestBody,omitempty"`
	NoContentStatuses     []string                  `json:"noContentStatuses,omitempty"`
	SuccessStatuses       []string                  `json:"successStatuses"`
	ResponseSchemas       []string                  `json:"responseSchemas"`
	HasSuccessContent     bool                      `json:"hasSuccessContent"`
	DataGoType            string                    `json:"dataGoType"`
	InputGoType           string                    `json:"inputGoType,omitempty"`
	RequestArguments      []requestArgumentManifest `json:"requestArguments"`
	DependentRequirements []dependentRequirement    `json:"dependentRequirements,omitempty"`
}

type dependentRequirement struct {
	IfJSONField       string `json:"ifJsonField"`
	IfGoField         string `json:"ifGoField"`
	RequiresJSONField string `json:"requiresJsonField"`
	RequiresGoField   string `json:"requiresGoField"`
}

type requestArgumentManifest struct {
	Field    string `json:"field"`
	Kind     string `json:"kind"`
	GoType   string `json:"goType"`
	WireName string `json:"wireName"`
	WireType string `json:"wireType"`
	Required bool   `json:"required"`
}

type parameterManifest struct {
	Name       string   `json:"name"`
	In         string   `json:"in"`
	Required   bool     `json:"required"`
	SchemaRefs []string `json:"schemaRefs"`
}

type bodyManifest struct {
	Required   bool     `json:"required"`
	GoType     string   `json:"goType,omitempty"`
	SchemaRefs []string `json:"schemaRefs"`
}

type schemaManifest struct {
	GoType string `json:"goType"`
}

type wireIndex struct {
	GoMethods        map[string]string
	RequestFunctions map[string]struct{}
	ParseFunctions   map[string]struct{}
	SchemaTypes      map[string]string
	RequestArguments map[string][]wireParameter
	TypeDeclarations map[string]ast.Expr
	TypeDependencies map[string][]string
	JSONFields       map[string]map[string]wireJSONField
	EnumConstants    []enumConstant
}

type wireJSONField struct {
	GoName  string
	Pointer bool
}

type wireParameter struct {
	Name string
	Type string
}

type enumConstant struct {
	Name     string
	TypeName string
}

type operationSource struct {
	Method    string
	Path      string
	PathItem  *openapi3.PathItem
	Operation *openapi3.Operation
}

func main() {
	root := flag.String("root", ".", "repository root")
	check := flag.Bool("check", false, "verify generated output without writing")
	validateOverlayOnly := flag.Bool("validate-overlay", false, "validate the Go-only overlay and exit")
	generateWireOnly := flag.Bool("generate-wire", false, "generate wire through equivalent boolean schemas")
	flag.Parse()
	if *generateWireOnly {
		if err := generateWire(*root); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if *validateOverlayOnly {
		if err := validateNamingOverlay(*root); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "generator: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*root, *check); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "generator: %v\n", err)
		os.Exit(1)
	}
}

func run(root string, check bool) error {
	manifestValue, err := buildManifest(root)
	if err != nil {
		return err
	}

	content, err := json.MarshalIndent(manifestValue, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	content = append(content, '\n')
	models, err := generateModels(manifestValue)
	if err != nil {
		return err
	}
	operations, err := generateOperations(manifestValue)
	if err != nil {
		return err
	}
	artifacts := map[string][]byte{
		manifestRelativePath:   content,
		modelsRelativePath:     models,
		operationsRelativePath: operations,
	}
	paths := make([]string, 0, len(artifacts))
	for relative := range artifacts {
		paths = append(paths, relative)
	}
	sort.Strings(paths)

	for _, relative := range paths {
		target := filepath.Join(root, filepath.FromSlash(relative))
		if check {
			actual, readErr := os.ReadFile(target)
			if readErr != nil {
				return fmt.Errorf("read %s: %w", relative, readErr)
			}
			if !bytes.Equal(actual, artifacts[relative]) {
				return fmt.Errorf("%s is stale; run go generate ./... to refresh it", relative)
			}
			continue
		}
		if err := writeFileAtomically(target, artifacts[relative]); err != nil {
			return fmt.Errorf("write %s: %w", relative, err)
		}
	}
	return nil
}

func buildManifest(root string) (*manifest, error) {
	if err := validateNamingOverlay(root); err != nil {
		return nil, err
	}
	source, err := decodeJSONFile[sourceFile](filepath.Join(root, filepath.FromSlash(sourceRelativePath)))
	if err != nil {
		return nil, err
	}
	if err := validateSource(source); err != nil {
		return nil, err
	}

	contractPath := filepath.Join(root, filepath.FromSlash(source.Contract.Path))
	contractBytes, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, fmt.Errorf("read contract: %w", err)
	}
	contractHash := sha256.Sum256(contractBytes)
	contractSHA := hex.EncodeToString(contractHash[:])
	if contractSHA != source.Contract.SHA256 {
		return nil, fmt.Errorf("contract hash differs from %s; review and update the Node baseline", sourceRelativePath)
	}

	policy, err := decodeJSONFile[policyFile](filepath.Join(root, filepath.FromSlash(policyRelativePath)))
	if err != nil {
		return nil, err
	}
	if err := validatePolicyDefaults(policy); err != nil {
		return nil, err
	}

	document, err := loadCompatibleContract(contractPath)
	if err != nil {
		return nil, fmt.Errorf("load contract: %w", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate contract: %w", err)
	}

	wire, err := loadWireIndex(filepath.Join(root, filepath.FromSlash(wireRelativePath)), document.Components.Schemas)
	if err != nil {
		return nil, err
	}
	operationSources, err := collectOperationSources(document)
	if err != nil {
		return nil, err
	}
	knownOperations := make(map[string]struct{}, len(operationSources))
	for operationID := range operationSources {
		knownOperations[operationID] = struct{}{}
	}
	if err := validatePolicyReferences(policy, knownOperations); err != nil {
		return nil, err
	}

	operations := make(map[string]operationManifest, len(operationSources))
	counts := policyCounts{}
	for operationID, source := range operationSources {
		operation, buildErr := buildOperationManifest(document, operationID, source, policy, wire)
		if buildErr != nil {
			return nil, buildErr
		}
		operations[operationID] = operation
		if operation.Retryable {
			counts.Retryable++
		}
		if operation.Mutability == "destructive" {
			counts.Destructive++
		}
		if operation.SecretInput {
			counts.SecretInput++
		}
		if operation.SecretOutput {
			counts.SecretOutput++
		}
	}

	schemas := make(map[string]schemaManifest, len(document.Components.Schemas))
	for schemaName := range document.Components.Schemas {
		goType, ok := wire.SchemaTypes[schemaName]
		if !ok {
			return nil, fmt.Errorf("wire model missing for component schema %q", schemaName)
		}
		schemas[schemaName] = schemaManifest{GoType: goType}
	}

	return &manifest{
		SchemaVersion:  1,
		NodeSDK:        source.NodeSDK,
		ContractSHA256: contractSHA,
		OperationCount: len(operations),
		SchemaCount:    len(schemas),
		PolicyCounts:   counts,
		Operations:     operations,
		Schemas:        schemas,
		wire:           wire,
	}, nil
}

func buildOperationManifest(document *openapi3.T, operationID string, source operationSource, policy policyFile, wire wireIndex) (operationManifest, error) {
	goMethod, ok := wire.GoMethods[foldIdentifier(operationID)]
	if !ok {
		return operationManifest{}, fmt.Errorf("wire Parse response missing for operation %q", operationID)
	}
	requestFunction := "New" + goMethod + "Request"
	parseFunction := "Parse" + goMethod + "Response"
	if _, ok := wire.RequestFunctions[requestFunction]; !ok {
		return operationManifest{}, fmt.Errorf("wire request constructor missing for operation %q", operationID)
	}
	if _, ok := wire.ParseFunctions[parseFunction]; !ok {
		return operationManifest{}, fmt.Errorf("wire response parser missing for operation %q", operationID)
	}

	parameters, hasParams := collectParameters(source.PathItem, source.Operation)
	requestBody := collectRequestBody(source.Operation, goMethod)
	statuses, responseSchemas, hasContent, err := collectSuccessResponses(source.Operation)
	if err != nil {
		return operationManifest{}, fmt.Errorf("operation %q: %w", operationID, err)
	}
	requestArguments, err := buildRequestArguments(source, goMethod, requestBody, wire)
	if err != nil {
		return operationManifest{}, fmt.Errorf("operation %q: %w", operationID, err)
	}
	dependentRequirements, err := buildDependentRequirements(source.Operation, requestArguments, wire)
	if err != nil {
		return operationManifest{}, fmt.Errorf("operation %q: %w", operationID, err)
	}
	dataGoType, err := successDataGoType(source.Operation, wire)
	if err != nil {
		return operationManifest{}, fmt.Errorf("operation %q: %w", operationID, err)
	}
	mutability := defaultMutability(source.Method)
	if override, exists := policy.MutabilityOverrides[operationID]; exists {
		mutability = override
	}
	retryable := contains(policy.RetryAllowlist, operationID)
	secretInput := contains(policy.SecretInput, operationID)
	secretOutput := contains(policy.SecretOutput, operationID)
	mcpExposure := policy.Defaults.MCPExposure
	if exposure, exists := policy.MCPExposure[operationID]; exists {
		if exposure != mutability {
			return operationManifest{}, fmt.Errorf("operation %q MCP exposure %q differs from mutability %q", operationID, exposure, mutability)
		}
		mcpExposure = exposure
	}
	// 敏感输入或输出一律不进入模型工具面，防止策略文件未来误放宽。
	if secretInput || secretOutput {
		mcpExposure = "never"
	}

	tag := ""
	if len(source.Operation.Tags) > 0 {
		tag = source.Operation.Tags[0]
	}
	if tag == "" {
		return operationManifest{}, fmt.Errorf("operation %q has no tag", operationID)
	}

	result := operationManifest{
		OperationID:           operationID,
		GoMethod:              goMethod,
		RequestFunction:       requestFunction,
		ResponseType:          goMethod + "Response",
		ParseResponseFunction: parseFunction,
		Method:                source.Method,
		Path:                  source.Path,
		Tag:                   tag,
		Summary:               source.Operation.Summary,
		AuthRequired:          securityRequired(document, source.Operation),
		Mutability:            mutability,
		Retryable:             retryable,
		SecretInput:           secretInput,
		SecretOutput:          secretOutput,
		MCPExposure:           mcpExposure,
		Parameters:            parameters,
		RequestBody:           requestBody,
		NoContentStatuses:     noContentStatuses(source.Operation),
		SuccessStatuses:       statuses,
		ResponseSchemas:       responseSchemas,
		HasSuccessContent:     hasContent,
		DataGoType:            dataGoType,
		RequestArguments:      requestArguments,
		DependentRequirements: dependentRequirements,
	}
	if len(requestArguments) > 0 {
		result.InputGoType = goMethod + "Input"
	}
	if hasParams {
		result.ParamsGoType = goMethod + "Params"
	}
	return result, nil
}

func collectOperationSources(document *openapi3.T) (map[string]operationSource, error) {
	result := make(map[string]operationSource)
	paths := document.Paths.Keys()
	sort.Strings(paths)
	for _, path := range paths {
		pathItem := document.Paths.Value(path)
		if pathItem == nil {
			return nil, fmt.Errorf("path %q has no path item", path)
		}
		methods := make([]string, 0, len(pathItem.Operations()))
		for method := range pathItem.Operations() {
			methods = append(methods, method)
		}
		sort.Strings(methods)
		for _, method := range methods {
			operation := pathItem.Operations()[method]
			if operation.OperationID == "" {
				return nil, fmt.Errorf("%s %s has no operationId", method, path)
			}
			if _, duplicate := result[operation.OperationID]; duplicate {
				return nil, fmt.Errorf("duplicate operationId %q", operation.OperationID)
			}
			result[operation.OperationID] = operationSource{
				Method:    strings.ToUpper(method),
				Path:      path,
				PathItem:  pathItem,
				Operation: operation,
			}
		}
	}
	return result, nil
}

func collectParameters(pathItem *openapi3.PathItem, operation *openapi3.Operation) ([]parameterManifest, bool) {
	refs := append(append(openapi3.Parameters{}, pathItem.Parameters...), operation.Parameters...)
	parameters := make([]parameterManifest, 0, len(refs))
	hasParamsGoType := false
	for _, parameterRef := range refs {
		if parameterRef == nil || parameterRef.Value == nil {
			continue
		}
		parameter := parameterRef.Value
		parameters = append(parameters, parameterManifest{
			Name:       parameter.Name,
			In:         parameter.In,
			Required:   parameter.Required,
			SchemaRefs: collectSchemaRefs(parameter.Schema),
		})
		if parameter.In != openapi3.ParameterInPath {
			hasParamsGoType = true
		}
	}
	sort.Slice(parameters, func(i, j int) bool {
		if parameters[i].In == parameters[j].In {
			return parameters[i].Name < parameters[j].Name
		}
		return parameters[i].In < parameters[j].In
	})
	return parameters, hasParamsGoType
}

func collectRequestBody(operation *openapi3.Operation, goMethod string) *bodyManifest {
	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return nil
	}
	body := operation.RequestBody.Value
	result := &bodyManifest{
		Required:   body.Required,
		SchemaRefs: collectContentSchemaRefs(body.Content),
	}
	if _, ok := body.Content["application/json"]; ok {
		result.GoType = goMethod + "JSONRequestBody"
	}
	return result
}

func collectSuccessResponses(operation *openapi3.Operation) ([]string, []string, bool, error) {
	if operation.Responses == nil {
		return nil, nil, false, errors.New("responses are missing")
	}
	statuses := make([]string, 0)
	refSet := make(map[string]struct{})
	hasContent := false
	for _, status := range operation.Responses.Keys() {
		if !isSuccessStatus(status) {
			continue
		}
		responseRef := operation.Responses.Value(status)
		if responseRef == nil || responseRef.Value == nil {
			return nil, nil, false, fmt.Errorf("success response %q is unresolved", status)
		}
		statuses = append(statuses, status)
		if len(responseRef.Value.Content) > 0 {
			hasContent = true
		}
		for _, schemaRef := range collectContentSchemaRefs(responseRef.Value.Content) {
			refSet[schemaRef] = struct{}{}
		}
	}
	if len(statuses) == 0 {
		return nil, nil, false, errors.New("no success response is declared")
	}
	sort.Strings(statuses)
	return statuses, sortedKeys(refSet), hasContent, nil
}

func collectContentSchemaRefs(content openapi3.Content) []string {
	refs := make(map[string]struct{})
	for _, mediaType := range content {
		if mediaType == nil {
			continue
		}
		for _, name := range collectSchemaRefs(mediaType.Schema) {
			refs[name] = struct{}{}
		}
	}
	return sortedKeys(refs)
}

func collectSchemaRefs(root *openapi3.SchemaRef) []string {
	refs := make(map[string]struct{})
	seen := make(map[*openapi3.SchemaRef]struct{})
	var visit func(*openapi3.SchemaRef)
	visit = func(ref *openapi3.SchemaRef) {
		if ref == nil {
			return
		}
		if _, exists := seen[ref]; exists {
			return
		}
		seen[ref] = struct{}{}
		if name := localSchemaName(ref.Ref); name != "" {
			refs[name] = struct{}{}
			return
		}
		if ref.Value == nil {
			return
		}
		schema := ref.Value
		for _, child := range append(append(append(openapi3.SchemaRefs{}, schema.OneOf...), schema.AnyOf...), schema.AllOf...) {
			visit(child)
		}
		visit(schema.Not)
		visit(schema.Items)
		visit(schema.Contains)
		visit(schema.PropertyNames)
		visit(schema.If)
		visit(schema.Then)
		visit(schema.Else)
		visit(schema.ContentSchema)
		for _, collection := range []openapi3.Schemas{schema.Properties, schema.PatternProperties, schema.DependentSchemas, schema.Defs} {
			for _, child := range collection {
				visit(child)
			}
		}
	}
	visit(root)
	return sortedKeys(refs)
}

func loadWireIndex(path string, componentSchemas openapi3.Schemas) (wireIndex, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return wireIndex{}, fmt.Errorf("read wire output: %w", err)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, source, 0)
	if err != nil {
		return wireIndex{}, fmt.Errorf("parse wire output: %w", err)
	}

	index := wireIndex{
		GoMethods:        make(map[string]string),
		RequestFunctions: make(map[string]struct{}),
		ParseFunctions:   make(map[string]struct{}),
		SchemaTypes:      make(map[string]string),
		RequestArguments: make(map[string][]wireParameter),
		TypeDeclarations: make(map[string]ast.Expr),
		TypeDependencies: make(map[string][]string),
		JSONFields:       make(map[string]map[string]wireJSONField),
	}
	typeNamesByFold := make(map[string][]string)
	for _, declaration := range file.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok {
			switch general.Tok {
			case token.TYPE:
				for _, specification := range general.Specs {
					typeSpecification, ok := specification.(*ast.TypeSpec)
					if !ok {
						continue
					}
					name := typeSpecification.Name.Name
					typeNamesByFold[foldIdentifier(name)] = append(typeNamesByFold[foldIdentifier(name)], name)
					index.TypeDeclarations[name] = typeSpecification.Type
				}
			case token.CONST:
				for _, specification := range general.Specs {
					valueSpecification, ok := specification.(*ast.ValueSpec)
					if !ok {
						continue
					}
					typeIdentifier, ok := valueSpecification.Type.(*ast.Ident)
					if !ok {
						continue
					}
					for _, name := range valueSpecification.Names {
						index.EnumConstants = append(index.EnumConstants, enumConstant{Name: name.Name, TypeName: typeIdentifier.Name})
					}
				}
			}
		}
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil {
			continue
		}
		name := function.Name.Name
		if strings.HasPrefix(name, "New") && strings.HasSuffix(name, "Request") {
			index.RequestFunctions[name] = struct{}{}
			goMethod := strings.TrimSuffix(strings.TrimPrefix(name, "New"), "Request")
			arguments := make([]wireParameter, 0)
			for _, field := range function.Type.Params.List {
				var typeBuffer bytes.Buffer
				if err := printer.Fprint(&typeBuffer, fileSet, field.Type); err != nil {
					return wireIndex{}, fmt.Errorf("render %s parameter type: %w", name, err)
				}
				for _, parameterName := range field.Names {
					if parameterName.Name == "server" {
						continue
					}
					arguments = append(arguments, wireParameter{Name: parameterName.Name, Type: typeBuffer.String()})
				}
			}
			index.RequestArguments[goMethod] = arguments
		}
		if strings.HasPrefix(name, "Parse") && strings.HasSuffix(name, "Response") {
			index.ParseFunctions[name] = struct{}{}
			goMethod := strings.TrimSuffix(strings.TrimPrefix(name, "Parse"), "Response")
			key := foldIdentifier(goMethod)
			if previous, duplicate := index.GoMethods[key]; duplicate && previous != goMethod {
				return wireIndex{}, fmt.Errorf("wire methods %q and %q have the same folded identifier", previous, goMethod)
			}
			index.GoMethods[key] = goMethod
		}
	}
	for typeName, expression := range index.TypeDeclarations {
		dependencies := make(map[string]struct{})
		ast.Inspect(expression, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if !ok || identifier.Name == typeName {
				return true
			}
			if _, local := index.TypeDeclarations[identifier.Name]; local {
				dependencies[identifier.Name] = struct{}{}
			}
			return true
		})
		index.TypeDependencies[typeName] = sortedKeys(dependencies)
		structure, ok := expression.(*ast.StructType)
		if !ok {
			continue
		}
		fields := make(map[string]wireJSONField)
		for _, field := range structure.Fields.List {
			if len(field.Names) != 1 || field.Tag == nil {
				continue
			}
			tag, unquoteErr := strconv.Unquote(field.Tag.Value)
			if unquoteErr != nil {
				return wireIndex{}, fmt.Errorf("decode %s.%s struct tag: %w", typeName, field.Names[0].Name, unquoteErr)
			}
			jsonName := strings.Split(reflect.StructTag(tag).Get("json"), ",")[0]
			if jsonName == "" || jsonName == "-" {
				continue
			}
			if _, duplicate := fields[jsonName]; duplicate {
				return wireIndex{}, fmt.Errorf("wire type %q has duplicate JSON field %q", typeName, jsonName)
			}
			_, pointer := field.Type.(*ast.StarExpr)
			fields[jsonName] = wireJSONField{GoName: field.Names[0].Name, Pointer: pointer}
		}
		index.JSONFields[typeName] = fields
	}
	sort.Slice(index.EnumConstants, func(i, j int) bool {
		if index.EnumConstants[i].TypeName == index.EnumConstants[j].TypeName {
			return index.EnumConstants[i].Name < index.EnumConstants[j].Name
		}
		return index.EnumConstants[i].TypeName < index.EnumConstants[j].TypeName
	})

	// 直接读取生成注释可获得真实 Go 类型名，并覆盖 ID、API、QR 等 initialism。
	schemaCandidates := make(map[string][]string)
	for _, match := range modelCommentPattern.FindAllSubmatch(source, -1) {
		goType := string(match[1])
		schemaName := string(match[2])
		if _, isComponent := componentSchemas[schemaName]; !isComponent {
			continue
		}
		schemaCandidates[schemaName] = append(schemaCandidates[schemaName], goType)
	}
	for schemaName := range componentSchemas {
		matching := uniqueStrings(typeNamesByFold[foldIdentifier(schemaName)])
		if len(matching) == 0 {
			matching = uniqueStrings(schemaCandidates[schemaName])
		}
		if len(matching) != 1 {
			return wireIndex{}, fmt.Errorf("component schema %q has no unambiguous Go type", schemaName)
		}
		// response component 可能复用 schema 名；优先选择折叠后与 schema 本身一致的模型类型。
		index.SchemaTypes[schemaName] = matching[0]
	}
	return index, nil
}

func validateSource(source sourceFile) error {
	// 源码基线不代表同版本 npm 包已发布，保留旧 release 记录的兼容读取。
	if source.NodeSDK.BaselineKind != "" && source.NodeSDK.BaselineKind != "approved-source" {
		return fmt.Errorf("unsupported Node baseline kind")
	}
	if source.SchemaVersion != 1 {
		return fmt.Errorf("%s schemaVersion must be 1", sourceRelativePath)
	}
	if source.NodeSDK.Package == "" || source.NodeSDK.Version == "" {
		return fmt.Errorf("%s Node package and version are required", sourceRelativePath)
	}
	repository, err := url.Parse(source.NodeSDK.Repository)
	if err != nil || repository.Scheme != "https" || repository.Host != "github.com" || repository.User != nil || repository.RawQuery != "" || repository.Fragment != "" {
		return fmt.Errorf("%s Node repository must be a public GitHub HTTPS URL", sourceRelativePath)
	}
	if matched, _ := regexp.MatchString(`^[0-9a-f]{40}$`, source.NodeSDK.Commit); !matched {
		return fmt.Errorf("%s Node commit must be a full lowercase SHA-1", sourceRelativePath)
	}
	if !filepath.IsLocal(filepath.FromSlash(source.Contract.Path)) || filepath.ToSlash(filepath.Clean(source.Contract.Path)) != source.Contract.Path {
		return fmt.Errorf("%s contract path must be a clean repository-relative path", sourceRelativePath)
	}
	if matched, _ := regexp.MatchString(`^[0-9a-f]{64}$`, source.Contract.SHA256); !matched {
		return fmt.Errorf("%s contract sha256 must be lowercase hexadecimal", sourceRelativePath)
	}
	return nil
}

func validatePolicyDefaults(policy policyFile) error {
	if policy.SchemaVersion != 1 {
		return fmt.Errorf("%s schemaVersion must be 1", policyRelativePath)
	}
	// 新 operation 保持禁用重试、无敏感推断且不暴露 MCP，必须显式审查后再放宽。
	if policy.Defaults.Retryable || policy.Defaults.SecretInput || policy.Defaults.SecretOutput || policy.Defaults.MCPExposure != "never" {
		return fmt.Errorf("%s defaults must remain fail-closed", policyRelativePath)
	}
	return nil
}

func validatePolicyReferences(policy policyFile, operations map[string]struct{}) error {
	lists := map[string][]string{
		"retryAllowlist": policy.RetryAllowlist,
		"secretInput":    policy.SecretInput,
		"secretOutput":   policy.SecretOutput,
	}
	for field, values := range lists {
		seen := make(map[string]struct{}, len(values))
		for _, operationID := range values {
			if _, duplicate := seen[operationID]; duplicate {
				return fmt.Errorf("%s contains duplicate operation %q", field, operationID)
			}
			seen[operationID] = struct{}{}
			if _, exists := operations[operationID]; !exists {
				return fmt.Errorf("%s references unknown operation %q", field, operationID)
			}
		}
	}
	for operationID, mutability := range policy.MutabilityOverrides {
		if _, exists := operations[operationID]; !exists {
			return fmt.Errorf("mutabilityOverrides references unknown operation %q", operationID)
		}
		if mutability != "read" && mutability != "write" && mutability != "destructive" {
			return fmt.Errorf("operation %q has invalid mutability %q", operationID, mutability)
		}
	}
	for operationID, exposure := range policy.MCPExposure {
		if _, exists := operations[operationID]; !exists {
			return fmt.Errorf("mcpExposure references unknown operation %q", operationID)
		}
		if exposure != "read" && exposure != "write" && exposure != "destructive" && exposure != "never" {
			return fmt.Errorf("operation %q has invalid MCP exposure %q", operationID, exposure)
		}
	}
	return nil
}

func securityRequired(document *openapi3.T, operation *openapi3.Operation) bool {
	if operation.Security != nil {
		return len(*operation.Security) > 0
	}
	return len(document.Security) > 0
}

func defaultMutability(method string) string {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return "read"
	case "DELETE":
		return "destructive"
	default:
		return "write"
	}
}

func localSchemaName(ref string) string {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	return strings.TrimPrefix(ref, prefix)
}

func isSuccessStatus(status string) bool {
	return len(status) == 3 && status[0] == '2' && ((status[1] >= '0' && status[1] <= '9') || status[1] == 'X') && ((status[2] >= '0' && status[2] <= '9') || status[2] == 'X')
}

func foldIdentifier(value string) string {
	var result strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') {
			result.WriteString(strings.ToLower(string(character)))
		}
	}
	return result.String()
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortedKeys[V any](values map[string]V) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func decodeJSONFile[T any](path string) (T, error) {
	var value T
	content, err := os.ReadFile(path)
	if err != nil {
		return value, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return value, fmt.Errorf("decode %s: trailing JSON value", filepath.Base(path))
		}
		return value, fmt.Errorf("decode %s trailing content: %w", filepath.Base(path), err)
	}
	return value, nil
}

func writeFileAtomically(target string, content []byte) error {
	actual, err := os.ReadFile(target)
	if err == nil && bytes.Equal(actual, content) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read generated output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create generated output directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".manifest-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary manifest: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary manifest mode: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary manifest: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary manifest: %w", err)
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return fmt.Errorf("replace generated manifest: %w", err)
	}
	return nil
}
