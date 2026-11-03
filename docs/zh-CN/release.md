# 发布流程

[English](../release.md) | [简体中文](release.md)


当前源码准备目标为 `v0.2.0`，尚未发布；详见[发布说明](release-notes.md)和[迁移指南](migration.md)。Module 版本来自已审查 tag，不是 `go.mod` 版本字段。下文原有 `v0.1.0` 示例说明发布流程，执行时使用已批准版本。本 candidate 的受控 staging/provider E2E 门禁仍待完成，正式发布前必须通过，准备阶段不得宣称已通过。

## Release 模型

本仓库通过经过审查的 Git tag 发布 Go module，不上传独立 binary，也不把源码复制到其他 registry。Tag 遵循
Semantic Versioning：

- `v0.1.0-alpha.1`：第一个公开 pre-release candidate；
- `v0.1.0`：完成 staging/provider 验证后的首次 release；
- 后续版本：根据 public facade 和 contract 的兼容性影响确定版本号。

在 `v1.0.0` 前也应避免不必要的 API 变化，并明确记录每个 breaking public change。创建/推送 tag 和创建
GitHub Release 都属于外部操作，需要维护者明确批准。

## Release gate

源码 release candidate 需要为每个 gate 提供证据：

| Gate | 证据 |
| --- | --- |
| Contract | 已批准公开 schema/release note 和已审查 digest |
| Coverage | 生成 manifest 覆盖每个 operation 和 schema |
| Generation | 第二次 `go generate ./...` 后没有 tracked/untracked 漂移 |
| Runtime | Unit/integration test 覆盖认证、URL、redirect、timeout、retry、limit、error、integer 和 pagination |
| Consumer 隔离 | Clean external module 真实 import/compile SDK，且依赖图排除 generator-only module |
| Compatibility | 已审查 public API 和 Node-parity 差异 |
| Documentation | 英文 canonical 与完整简体中文文档一致 |
| Public boundary | 自动扫描和人工 diff 不包含 secret 或私有 provenance |
| Supply chain | Vet、静态分析、漏洞扫描、action/tool 固定版本审查 |
| Real API | 受控 staging/provider E2E 结果单独记录 |

Alpha 源码 candidate 可以暂时保持 real-API gate 为 pending，但 `v0.1.0` 会被该 gate 阻断。Health response 或
unit test 不能证明账号已授权或 provider workflow 成功。

## 发布前验证

在干净工作树中使用最新受支持 Go 1.25 patch 执行，并通过 CI 再覆盖最新受支持 Go 1.26 patch：

```bash
go mod download
go -C internal/generator mod download
go mod tidy -diff
go -C internal/generator mod tidy -diff
unformatted="$(gofmt -l $(git ls-files '*.go'))"
test -z "${unformatted}"
go generate ./...
git diff --exit-code
git status --short
go test ./...
go -C internal/generator test ./...
go test -race ./...
go -C internal/generator test -race ./...
go vet ./...
go -C internal/generator vet ./...
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
staticcheck ./...
(cd internal/generator && staticcheck ./...)
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
govulncheck ./...
(cd internal/generator && govulncheck ./...)
bash scripts/check-public-test.sh
bash scripts/check-public-docs.sh
bash scripts/check-public.sh
git diff --check
```

打 tag 前 `git status --short` 必须为空。即使 scanner 通过，也要人工检查全部 tracked file、generated artifact、
example、test fixture 和 GitHub workflow 变化。

## 版本审查

批准版本前：

1. 确认目标 tag 和兼容性级别；
2. 确认 root `go.mod` 使用公开 module path 和支持的 Go baseline，同时 generator toolchain dependency
   只由 `internal/generator/go.mod` 固定；
3. 验证 contract digest 和 parity source 只使用公开 provenance；
4. 审查生成 operation/schema 数量和安全 policy 数量；
5. 为 contract 或行为变化更新面向 release 的文档；
6. 确认不存在临时 replace directive、本地路径、debug flag、credential 或 test-only export；
7. 确认 CI 针对精确 release commit 通过。

Release command、workflow secret name（已记录的 SDK 环境变量除外）、release note 或验证输出中不得放置真实
credential。

## Tag 与 GitHub Release

获得明确批准后，由授权维护者创建已经审查的 tag，并只 push 该 tag。Release note 应说明：

- 公开 operation/schema 或行为变化；
- 兼容性影响和迁移说明；
- 安全边界变化；
- 支持的 Go 版本；
- 离线 gate 状态，以及独立 staging/provider E2E 状态；
- 不包含内部实现或客户细节的已知限制。

除非确有单独审查过的制品需求，不附加 GitHub 标准 archive 之外的 source bundle。不得附加 test log、`.env`、
coverage profile、generated debug output 或 E2E 数据。

## Consumer 验证

打 tag 前先运行自动化本地隔离 gate。它使用 `GOWORK=off` 创建临时 module，写入真实 import root package 的
源码并编译，同时拒绝 generator-only dependency 进入 consumer graph：

```bash
go -C internal/generator test -run TestCleanConsumerCompilesWithoutCodegenDependencies -v
```

Tag 公开后，再从新的临时目录独立验证 released module。Source file 是必要证据：在空 module 中执行
`go test ./...` 不会编译 SDK，不能作为 release 证明。

```bash
consumer_dir="$(mktemp -d)"
cd "${consumer_dir}"
GOWORK=off go mod init example.com/unifyport-release-check
GOWORK=off go get github.com/Unify-Port/UnifyPort-SDK-GO@v0.1.0
cat > consumer.go <<'EOF'
package consumer

import (
	"context"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func compilePublicSDK(ctx context.Context, client *unifyport.Client) {
	_, _ = client.GetWorkspace(ctx)
	_ = unifyport.ProviderNameLine
}
EOF
GOWORK=off go mod tidy
GOWORK=off go test ./...
modules="$(GOWORK=off go list -m -f '{{.Path}}' all)"
printf '%s\n' "${modules}"
for forbidden in \
  github.com/getkin/kin-openapi \
  github.com/oapi-codegen/oapi-codegen/v2 \
  github.com/speakeasy-api/openapi \
  go.yaml.in/yaml/v3; do
  if grep -Fxq "${forbidden}" <<<"${modules}"; then
    printf 'unexpected generator dependency: %s\n' "${forbidden}" >&2
    exit 1
  fi
done
```

请用实际 release 版本替换 `v0.1.0`。Consumer check 不使用 API key，也不调用 live service，只证明 released
module 可解析、public package 可编译，以及 generator dependency 隔离。

## 发布后验证

- 确认 tag 和 GitHub Release 指向已审查 commit；
- 确认 `go list -m` 通过公开 module ecosystem 解析到目标版本；
- 在 clean consumer module 编译文档示例；
- 检查公开 archive 是否存在意外文件；
- 记录受控 staging/provider E2E 结果，但不包含 payload 或 identifier；
- 对 deferred gate 创建 follow-up issue，不能把未完成事项描述为完成。

若已发布 release 暴露 secret 或私有数据，应立即按私下安全报告流程处理。仅删除 tag 不能移除公共 module proxy
已经镜像的内容。
