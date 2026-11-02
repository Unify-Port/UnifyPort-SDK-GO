package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/util"
	"go.yaml.in/yaml/v3"
)

// boolean schema 的等价展开仅适配旧解析器，批准契约及命名 overlay 都保持原始字节。
func compatibleSchema(value any) any {
	if boolean, ok := value.(bool); ok {
		if boolean {
			return map[string]any{}
		}
		// false 与 object ∩ not({}) 等价；显式对象让可选字段保留 presence，避免 interface{} 被自定义 marshaler 输出成 null。
		return map[string]any{"type": "object", "additionalProperties": false, "not": map[string]any{}}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	// 上游生成器忽略 3.1 const；冗余 enum/type 保留方向、状态等字面量的 Go 类型。
	if literal, exists := object["const"]; exists {
		if text, ok := literal.(string); ok {
			if domain, hasEnum := object["enum"]; hasEnum {
				for _, candidate := range domain.([]any) {
					if candidate == text {
						object["enum"] = []any{text}
						break
					}
				}
			} else {
				object["enum"] = []any{text}
			}
			if _, hasType := object["type"]; !hasType {
				object["type"] = "string"
			}
		} else if _, ok := literal.(bool); ok {
			if _, hasType := object["type"]; !hasType {
				object["type"] = "boolean"
			}
		}
	}
	for _, keyword := range []string{"properties", "patternProperties", "$defs"} {
		if fields, ok := object[keyword].(map[string]any); ok {
			for name, field := range fields {
				fields[name] = compatibleSchema(field)
			}
		}
	}
	for _, keyword := range []string{"items", "not", "if", "then", "else", "contains"} {
		if field, exists := object[keyword]; exists {
			object[keyword] = compatibleSchema(field)
		}
	}
	if field, ok := object["additionalProperties"].(map[string]any); ok {
		object["additionalProperties"] = compatibleSchema(field)
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf", "prefixItems"} {
		if branches, ok := object[keyword].([]any); ok {
			for index, branch := range branches {
				branches[index] = compatibleSchema(branch)
			}
		}
	}
	return object
}

func compatibleContract(path string) (string, func(), error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return "", nil, err
	}
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	normalized := make(map[string]any)
	for name, schema := range schemas {
		normalized[name] = enrichIntersections(cloneSchema(schema), schemas)
	}
	document["components"].(map[string]any)["schemas"] = normalized
	// 只转换 schema 位置，避免把 example、required 和协议中的布尔值当作 schema。
	var visit func(map[string]any)
	visit = func(object map[string]any) {
		for key, value := range object {
			if key == "schemas" {
				if schemas, ok := value.(map[string]any); ok {
					for name, schema := range schemas {
						schemas[name] = compatibleSchema(schema)
					}
				}
			} else if key == "schema" {
				object[key] = compatibleSchema(value)
			} else if child, ok := value.(map[string]any); ok {
				visit(child)
			} else if children, ok := value.([]any); ok {
				for _, child := range children {
					if item, ok := child.(map[string]any); ok {
						visit(item)
					}
				}
			}
		}
	}
	visit(document)
	content, err = json.Marshal(document)
	if err != nil {
		return "", nil, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".codegen-*.json")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.Remove(temporary.Name()) }
	if _, err = temporary.Write(content); err != nil {
		_ = temporary.Close()
		cleanup()
		return "", nil, err
	}
	if err = temporary.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return temporary.Name(), cleanup, nil
}

// allOf 的重复约束在逻辑上等价；补齐分支中的共同字段，避免上游生成器浅合并时丢失嵌套字段。
func enrichIntersections(value any, schemas map[string]any) any {
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	resolve := func(item map[string]any) map[string]any {
		if ref, ok := item["$ref"].(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
			if resolved, ok := schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any); ok {
				return cloneSchema(resolved).(map[string]any)
			}
		}
		return item
	}
	var merge func(map[string]any, map[string]any) map[string]any
	merge = func(left, right map[string]any) map[string]any {
		result := make(map[string]any)
		for key, item := range resolve(left) {
			result[key] = item
		}
		for key, item := range resolve(right) {
			switch key {
			case "properties":
				fields := make(map[string]any)
				if previous, ok := result[key].(map[string]any); ok {
					for name, field := range previous {
						fields[name] = field
					}
				}
				for name, field := range item.(map[string]any) {
					previous, leftOK := fields[name].(map[string]any)
					current, rightOK := field.(map[string]any)
					if fields[name] == false || field == false {
						fields[name] = false
					} else if leftOK && rightOK {
						fields[name] = merge(previous, current)
					} else {
						fields[name] = field
					}
				}
				result[key] = fields
			case "required":
				combined, _ := result[key].([]any)
				combined = append([]any(nil), combined...)
				for _, name := range item.([]any) {
					found := false
					for _, previous := range combined {
						if previous == name {
							found = true
						}
					}
					if !found {
						combined = append(combined, name)
					}
				}
				result[key] = combined
			case "enum":
				if previous, ok := result[key].([]any); ok {
					var common []any
					for _, candidate := range previous {
						for _, other := range item.([]any) {
							if reflect.DeepEqual(candidate, other) {
								common = append(common, candidate)
								break
							}
						}
					}
					result[key] = common
				} else {
					result[key] = item
				}
			default:
				result[key] = item
			}
		}
		return result
	}
	if branches, ok := object["allOf"].([]any); ok {
		shared := make(map[string]any)
		for _, branch := range branches {
			if item, ok := branch.(map[string]any); ok {
				shared = merge(shared, item)
			}
		}
		for _, key := range []string{"properties", "required"} {
			if item, exists := object[key]; exists {
				shared = merge(shared, map[string]any{key: item})
			}
		}
		constraints := make(map[string]any)
		for _, key := range []string{"properties", "required"} {
			if value, exists := shared[key]; exists {
				constraints[key] = value
			}
		}
		if shared["type"] == "object" {
			constraints["type"] = "object"
		}
		// 保留原分支（尤其 additionalProperties:false），只附加它们已经蕴含的结构约束。
		if len(constraints) > 0 {
			object["allOf"] = append(branches, constraints)
		}

	}
	for key, item := range object {
		if key == "properties" {
			if fields, ok := item.(map[string]any); ok {
				for name, field := range fields {
					fields[name] = enrichIntersections(field, schemas)
				}
			}
		} else if key == "items" || key == "additionalProperties" {
			object[key] = enrichIntersections(item, schemas)
		} else if key == "allOf" || key == "oneOf" || key == "anyOf" {
			if branches, ok := item.([]any); ok {
				for index, branch := range branches {
					branches[index] = enrichIntersections(branch, schemas)
				}
			}
		}
	}
	return object
}

func loadCompatibleContract(path string) (*openapi3.T, error) {
	adapted, cleanup, err := compatibleContract(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return util.LoadSwagger(adapted)
}

func generateWire(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	adapted, cleanup, err := compatibleContract(filepath.Join(root, "contracts/device.openapi.yaml"))
	if err != nil {
		return err
	}
	defer cleanup()
	command := exec.Command("go", "tool", "oapi-codegen", "--config", filepath.Join(root, "codegen/oapi-codegen.yaml"), adapted)
	command.Dir = filepath.Join(root, "internal/generator")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("generate wire: %w", err)
	}
	return nil
}

// 引用的 schema 不可原地共享，否则某个专用事件的 required 会污染其他事件。
func cloneSchema(value any) any {
	content, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var copy any
	if err := json.Unmarshal(content, &copy); err != nil {
		panic(err)
	}
	return copy
}
