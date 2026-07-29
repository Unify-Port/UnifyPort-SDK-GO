package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanConsumerCompilesWithoutCodegenDependencies(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	consumer := t.TempDir()
	module := fmt.Sprintf(`module example.com/unifyport-consumer

go 1.25.0

require github.com/Unify-Port/UnifyPort-SDK-GO v0.0.0

replace github.com/Unify-Port/UnifyPort-SDK-GO => %s
`, filepath.ToSlash(root))
	if err := os.WriteFile(filepath.Join(consumer, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatalf("write consumer go.mod: %v", err)
	}
	// 真实 import 和符号引用可防止空 module 让依赖隔离门禁出现假阳性。
	source := `package consumer

import (
	"context"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func compilePublicSDK(ctx context.Context, client *unifyport.Client) {
	_, _ = client.GetWorkspace(ctx)
	_ = unifyport.ProviderNameLine
}
`
	if err := os.WriteFile(filepath.Join(consumer, "consumer.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write consumer source: %v", err)
	}

	runConsumerGo(t, consumer, "mod", "tidy")
	runConsumerGo(t, consumer, "test", "./...")
	graph := runConsumerGo(t, consumer, "list", "-m", "-f", "{{.Path}}", "all")
	modules := make(map[string]struct{})
	for _, modulePath := range strings.Fields(graph) {
		modules[modulePath] = struct{}{}
	}
	t.Logf("compiled clean consumer module graph contains %d modules", len(modules))
	for _, forbidden := range []string{
		"github.com/getkin/kin-openapi",
		"github.com/oapi-codegen/oapi-codegen/v2",
		"github.com/speakeasy-api/openapi",
		"go.yaml.in/yaml/v3",
	} {
		if _, exists := modules[forbidden]; exists {
			t.Errorf("consumer module graph unexpectedly contains codegen dependency %q", forbidden)
		}
	}
}

func runConsumerGo(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("go", arguments...)
	command.Dir = directory
	// 临时 consumer 必须脱离开发者可能配置的 workspace，才能模拟公开模块使用方。
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}
