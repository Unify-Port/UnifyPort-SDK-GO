package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
)

func TestSchemaCompatibilityPreservesValidationAndSources(t *testing.T) {
	root := filepath.Join("..", "..")
	content, err := os.ReadFile(filepath.Join(root, "contracts/device.openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	before := cloneSchema(schemas)
	for _, schema := range schemas {
		_ = enrichIntersections(cloneSchema(schema), schemas)
	}
	if !reflect.DeepEqual(before, cloneSchema(schemas)) {
		t.Fatal("normalization mutated shared source schemas")
	}
	for name, schema := range schemas {
		schemas[name] = compatibleSchema(schema)
	}
	originalJSON, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	original, err := openapi3.NewLoader().LoadFromData(originalJSON)
	if err != nil {
		t.Fatal(err)
	}
	adapted, err := loadCompatibleContract(filepath.Join(root, "contracts/device.openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join(root, "testdata/webhook-examples.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string         `json:"name"`
		Schema string         `json:"schema"`
		Body   map[string]any `json:"body"`
	}
	if err := json.Unmarshal(fixture, &cases); err != nil {
		t.Fatal(err)
	}
	for _, example := range cases {
		t.Run(example.Name, func(t *testing.T) {
			originalSchema := original.Components.Schemas[example.Schema].Value
			adaptedSchema := adapted.Components.Schemas[example.Schema].Value
			if err := originalSchema.VisitJSON(example.Body); err != nil {
				t.Fatalf("public example invalid before adaptation: %v", err)
			}
			check := func(payload any) {
				left := originalSchema.VisitJSON(payload) == nil
				right := adaptedSchema.VisitJSON(payload) == nil
				if left != right {
					t.Fatalf("schema adaptation changed acceptance (original=%t adapted=%t)", left, right)
				}
			}
			check(example.Body)
			// 按路径删除和替换每个字段，检查封闭对象、必填、状态分支及 false/zero 的边界。
			var visit func(map[string]any, map[string]any)
			visit = func(rootValue, current map[string]any) {
				for key, value := range current {
					delete(current, key)
					check(rootValue)
					current[key] = value
					for _, replacement := range []any{nil, false, float64(0), "", []any{}, map[string]any{}} {
						current[key] = replacement
						check(rootValue)
					}
					current[key] = value
					if child, ok := value.(map[string]any); ok {
						visit(rootValue, child)
					}
				}
			}
			copy := cloneSchema(example.Body).(map[string]any)
			visit(copy, copy)
		})
	}
}
