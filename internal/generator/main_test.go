package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestBuildManifestMatchesApprovedBaseline(t *testing.T) {
	t.Parallel()
	root := filepath.Clean(filepath.Join("..", ".."))
	result, err := buildManifest(root)
	if err != nil {
		t.Fatalf("buildManifest() error = %v", err)
	}

	if result.OperationCount != 78 {
		t.Fatalf("OperationCount = %d, want 78", result.OperationCount)
	}
	// FB 接收模型增加覆盖量，不应扩大 HTTP operation 或放宽安全策略。
	if result.SchemaCount != 225 {
		t.Fatalf("SchemaCount = %d, want 225", result.SchemaCount)
	}
	wantCounts := policyCounts{Retryable: 19, Destructive: 8, SecretInput: 9, SecretOutput: 16}
	if result.PolicyCounts != wantCounts {
		t.Fatalf("PolicyCounts = %#v, want %#v", result.PolicyCounts, wantCounts)
	}

	qrOperation := result.Operations["checkAccountQrAuth"]
	if qrOperation.GoMethod != "CheckAccountQRAuth" {
		t.Fatalf("checkAccountQrAuth GoMethod = %q, want CheckAccountQRAuth", qrOperation.GoMethod)
	}
	if qrOperation.RequestFunction != "NewCheckAccountQRAuthRequest" || qrOperation.ParseResponseFunction != "ParseCheckAccountQRAuthResponse" {
		t.Fatalf("checkAccountQrAuth wire mapping is incomplete: %#v", qrOperation)
	}
	if got := result.Schemas["ContactIdRequest"].GoType; got != "ContactIDRequest" {
		t.Fatalf("ContactIdRequest GoType = %q, want ContactIDRequest", got)
	}
	if got := result.Schemas["RuntimeActionResult"].GoType; got != "RuntimeActionResult" {
		t.Fatalf("RuntimeActionResult GoType = %q, want RuntimeActionResult", got)
	}
	if got := result.Operations["deleteAccount"].DataGoType; got != "*AccountOperation" {
		t.Fatalf("deleteAccount DataGoType = %q, want *AccountOperation", got)
	}
	if got := result.Operations["getContact"].RequestArguments[1].GoType; got != "GetContactParams" {
		t.Fatalf("getContact Params GoType = %q, want required value", got)
	}
	if got := result.Operations["listContacts"].RequestArguments[1].GoType; got != "*ListContactsParams" {
		t.Fatalf("listContacts Params GoType = %q, want optional pointer", got)
	}
	if got := result.Operations["cancelAccountAuth"].RequestArguments[1].GoType; got != "*CancelAccountAuthRequestBody" {
		t.Fatalf("cancelAccountAuth Body GoType = %q, want optional pointer", got)
	}
	readRequirements := result.Operations["markConversationRead"].DependentRequirements
	// Telegram 只接受消息边界，WhatsApp 成对约束不能提升为所有 provider 的 guard。
	if len(readRequirements) != 0 {
		t.Fatalf("unexpected provider-independent read guard: %#v", readRequirements)
	}

	models, err := generateModels(result)
	if err != nil {
		t.Fatalf("generateModels() error = %v", err)
	}
	if !strings.Contains(string(models), "ProviderNameLine ProviderName = wire.ProviderNameLine") || !strings.Contains(string(models), "GroupJoinRequestsUpdateRequestActionApprove") {
		t.Fatal("generated enum constants do not use stable type-prefixed names")
	}
	if regexp.MustCompile(`(?m)^\s*(Line|Start)\s`).Match(models) {
		t.Fatal("generated public enum constants expose an unprefixed short name")
	}
	operations, err := generateOperations(result)
	if err != nil {
		t.Fatalf("generateOperations() error = %v", err)
	}
	if count := len(regexp.MustCompile(`(?m)^func \(c \*Client\) [A-Za-z0-9]+\(`).FindAll(operations, -1)); count != 78 {
		t.Fatalf("generated Client method count = %d, want 78", count)
	}
	if strings.Contains(string(operations), `Reason: "up_to_message_sender_id is required when up_to_message_id is provided"`) {
		t.Fatal("generated facade rejects the legal Telegram read boundary")
	}
}

func TestGeneratedManifestIsCurrent(t *testing.T) {
	// check 模式禁止测试覆盖生成物，只验证仓库中已提交内容完全可复现。
	root := filepath.Clean(filepath.Join("..", ".."))
	if err := run(root, true); err != nil {
		t.Fatalf("run(check=true) error = %v", err)
	}
}

func TestNamingOverlayRejectsWireSemanticUpdates(t *testing.T) {
	tests := map[string]string{
		"standard field": "  - target: $.paths\n    description: must fail\n    update:\n      paths: {}\n",
		"type override":  "  - target: $.components.responses.RuntimeActionResult\n    description: must fail\n    update:\n      x-go-type: string\n",
		"extra target":   "  - target: $.components.responses.RuntimeActionResult\n    description: approved\n    update:\n      x-go-name: RuntimeActionResponse\n  - target: $.components.schemas.Account\n    description: must fail\n    update:\n      x-go-name: RenamedAccount\n",
	}
	for name, actions := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(overlayRelativePath))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			content := []byte("overlay: 1.0.0\ninfo:\n  title: invalid\n  version: 0.1.0\nactions:\n" + actions)
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			// Go extension 同样可改变 requiredness 或公开类型，未在精确白名单中就必须失败。
			if err := validateNamingOverlay(root); err == nil {
				t.Fatal("validateNamingOverlay() accepted an unapproved update")
			}
		})
	}
}

func TestCollectDependentRequiredRejectsConditionalBranches(t *testing.T) {
	for _, branch := range []string{"oneOf", "anyOf"} {
		t.Run(branch, func(t *testing.T) {
			conditional := &openapi3.SchemaRef{Value: &openapi3.Schema{
				DependentRequired: map[string][]string{"source": {"target"}},
			}}
			schema := &openapi3.Schema{}
			if branch == "oneOf" {
				schema.OneOf = openapi3.SchemaRefs{conditional}
			} else {
				schema.AnyOf = openapi3.SchemaRefs{conditional}
			}
			// 分支条件不同，生成器必须拒绝把局部规则错误提升为公开方法的全局约束。
			if _, err := collectDependentRequired(&openapi3.SchemaRef{Value: schema}); err == nil {
				t.Fatalf("collectDependentRequired() accepted %s branch constraint", branch)
			}
		})
	}
}
