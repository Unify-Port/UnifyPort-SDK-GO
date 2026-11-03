package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

func generateWebhookFixtures(root string) ([]byte, error) {
	// YAML 节点保留声明顺序和原始时间字符串，避免 map 遍历或时间截断导致跨语言样例漂移。
	content, err := os.ReadFile(filepath.Join(root, "contracts/device.openapi.yaml"))
	if err != nil {
		return nil, err
	}
	var document struct {
		Webhooks yaml.Node `yaml:"webhooks"`
	}
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	var stringsOnly func(*yaml.Node)
	stringsOnly = func(node *yaml.Node) {
		if node.Tag == "!!timestamp" {
			node.Tag = "!!str"
		}
		for _, child := range node.Content {
			stringsOnly(child)
		}
	}
	stringsOnly(&document.Webhooks)
	type example struct {
		Name   string `json:"name"`
		Schema string `json:"schema"`
		Body   any    `json:"body"`
	}
	cases := []example{}
	for index := 0; index < len(document.Webhooks.Content); index += 2 {
		name := document.Webhooks.Content[index].Value
		var webhook struct {
			Post struct {
				RequestBody struct {
					Content map[string]struct {
						Schema struct {
							Ref string `yaml:"$ref"`
						} `yaml:"schema"`
						Examples yaml.Node `yaml:"examples"`
						Example  any       `yaml:"example"`
					} `yaml:"content"`
				} `yaml:"requestBody"`
			} `yaml:"post"`
		}
		if err := document.Webhooks.Content[index+1].Decode(&webhook); err != nil {
			return nil, err
		}
		media, ok := webhook.Post.RequestBody.Content["application/json"]
		if !ok {
			return nil, fmt.Errorf("webhook %s missing JSON content", name)
		}
		schema := filepath.Base(media.Schema.Ref)
		for offset := 0; offset < len(media.Examples.Content); offset += 2 {
			label := media.Examples.Content[offset].Value
			var wrapped struct {
				Value any `yaml:"value"`
			}
			if err := media.Examples.Content[offset+1].Decode(&wrapped); err != nil {
				return nil, err
			}
			if wrapped.Value != nil {
				cases = append(cases, example{name + "/" + label, schema, wrapped.Value})
			}
		}
		if media.Example != nil {
			cases = append(cases, example{name + "/example", schema, media.Example})
		}
	}
	encoded, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
