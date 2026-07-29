# 契约维护

[English](../contract-maintenance.md) | [简体中文](contract-maintenance.md)

## Source of truth

| 关注点 | 权威文件 | 规则 |
| --- | --- | --- |
| Device API wire protocol | `contracts/device.openapi.yaml` | 唯一已批准公开 schema 输入 |
| 已批准 Go 命名 workaround | `codegen/go.overlay.yaml` | 精确 fail-closed allowlist；禁止 type/wire 变化 |
| Operation 安全性 | `policy/operation-policies.json` | 显式、语言中立分类 |
| Node baseline | `parity/source.json` | 只记录公开 release/commit 和契约 digest |
| 派生覆盖信息 | 生成的 parity manifest | 禁止手工维护 |

不得根据未公开实现、生产观察结果或其他私有仓库推测字段、status、副作用、认证规则或 provider 行为。若已批准
公开材料不完整，应保持现有契约，并在生成文件之外记录待确认问题。

## 前置条件

- Go 1.25 或 1.26 支持线中的最新安全 patch；
- `internal/generator/go.mod` 固定的 generator 版本，以及 root `go:generate` 入口；
- 本次变化对应的已批准公开 API schema 和 release note；
- 已识别并理解工作树中的现有修改；
- 已明确受影响行为的兼容性和安全负责人。

生成成功只能证明工具可以处理文档，不能证明协议或 operation 安全分类正确。

## 标准更新流程

### 1. 审查公开变化

确认：

- 稳定且唯一的 `operationId`、method、path、tag 和认证；
- path/query/header/body parameter 及其 required/nullable 规则；
- 所有公开成功/错误 status、content type、response header 和 empty-body 情况；
- schema format、enum、range、union 和 additional-property 行为；
- 副作用、idempotency、secret 生命周期、destructive 行为和兼容性；
- release note 说明的新增、弃用和 breaking change。

### 2. 更新仓库内契约

只能依据该批准材料修改 `contracts/device.openapi.yaml`。Provider 账号路由仍保留在 Device API 中。`204`、
非 JSON response、redirect 和 integer range 应按协议真实定义建模，不能强制套入统一 JSON 形状。

### 3. 保持 overlay fail-closed

`codegen/go.overlay.yaml` 不是通用 extension surface。Validator 当前只接受一个 action：

| 属性 | 唯一允许值 |
| --- | --- |
| `target` | `$.components.responses.RuntimeActionResult` |
| update key | `x-go-name` |
| update value | `RuntimeActionResponse` |

它只解决 response/schema component 同名冲突，不改变 contract。代码生成前，validator 会拒绝 `x-go-type`、
其他 update key/value、其他 target、多于一个 action、未知 YAML field 或多个 document。

未来若确需新增 workaround，必须在同一个 pull request 中完成显式 contract/security review，并同步更新
validator/test。它不能改变 method、path、operation ID、认证、parameter location、request/response 字段、
required、nullability、status、media type、schema constraint、协议描述或 Go type semantics。

### 4. 分类 operation 行为

只有审查每个受影响 operation 后才能更新 `policy/operation-policies.json`。每个新 operation 从严格默认值开始：
不重试、不假设 secret 属性，也不增加暴露面。

至少分类：

- `mutability`：read、write 或 destructive；
- `retryable`：只有已证明 idempotency 和失败安全性时才为 true；
- 分别标记 `secretInput` 和 `secretOutput`；
- parity 报告需要保留的其他跨语言 policy 字段。

MCP 分类可以留在语言中立 parity 数据中，但不会在 Go 仓库创建 MCP runtime 或公开 tool surface。

### 5. 生成并审查漂移

```bash
go generate ./...
git status --short
git diff --exit-code
```

审查生成 model type、operation name、method/path 映射、request requiredness、response status、policy 元数据和
manifest 数量。主动更新时，`git diff --exit-code` 报告变化是预期行为；提交或对比目标生成物之后，再生成一次
必须保持干净。

禁止手改生成物。应修复契约、policy、overlay、template 或 generator。
Generation 通过隔离的 `internal/generator` module 执行。Root runtime module 必须继续保持无 generator-only
dependency，且不嵌入 OpenAPI 文档。

### 6. 增加边界测试和文档

根据变化覆盖：

- request path/query/body 序列化和成功 response 解析；
- 公开 error、empty body、malformed/non-JSON body 和 oversized body；
- credential scope 和 redirect 拒绝；
- timeout、cancellation、retry 分类、`Retry-After` 和 backoff 上限；
- 同步 build/parse deadline 检查，以及预算到期后在 credential/HTTP 前拒绝继续；
- 在 credential 或 network work 之前执行生成的 `dependentRequired` guard；
- 精确 `int64`/`uint64` 边界和 no-content result；
- missing/repeated cursor 和最大页数 pagination failure；
- 稳定 Go method 命名和 example 编译。

Canonical 英文文档和完整简体中文版本必须同步更新。

### 7. 运行 release gate

```bash
go generate ./...
git diff --exit-code
go mod tidy -diff
go -C internal/generator mod tidy -diff
unformatted="$(gofmt -l $(git ls-files '*.go'))"
test -z "${unformatted}"
go test ./...
go -C internal/generator test ./...
go test -race ./...
go -C internal/generator test -race ./...
go vet ./...
go -C internal/generator vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go -C internal/generator run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
go -C internal/generator run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
bash scripts/check-public-test.sh
bash scripts/check-public-docs.sh
bash scripts/check-public.sh
git diff --check
```

生成后检查 `git status --short`，以发现 untracked 输出。CI 覆盖 Go 1.25 和 1.26 支持线中的最新可用 patch。
Nested test suite 包含 clean external consumer：真实 import/compile root package，并拒绝 generator-only module
进入依赖图。真实 staging/provider E2E 结果仍是独立 release gate。

## 兼容性审查

至少把以下变化视为潜在 breaking change：

- 删除或重命名 operation ID、公开 Go method、type、field 或 error；
- 改变 Go initialism 归一化或 request struct 布局；
- 增加 required input、缩窄 type，或改变成功 status/media type；
- 改变 integer signedness/range、no-content 表达或可见 result 元数据；
- 改变认证、redirect、retry、timeout、response-size 或 pagination 默认值；
- 暴露 generated/raw transport type，导致调用方开始依赖；
- 提高最低 Go 版本。

Optional response field 仍可能影响 exhaustive business logic，必须写入文档。

## Node 同步

公开 Node SDK 是行为基线，仓库内 OpenAPI 契约仍是唯一 wire 输入。更新 baseline 时：

1. 验证公开 Node release 和仓库 revision；
2. 对比完整契约 digest 和 operation policy，而不是抽查文件；
3. 只复制已批准公开契约/policy 变化；
4. 在 `parity/source.json` 记录公开 provenance，不得写本地路径或 private branch；
5. 重新生成并审查完整 Go diff；
6. 在 [Node parity](node-parity.md) 记录有意的生态差异；
7. 运行所有离线 gate，并单独记录外部 E2E 状态。

## 诊断生成漂移

Clean-generation 检查失败时：

1. 确认 Go，以及 `internal/generator/go.mod` 中的 generator 版本；
2. 运行生成并检查所有 tracked/untracked 变化；
3. 检查契约、policy、overlay、排序、换行、timestamp 和意外绝对路径；
4. 修复输入或 generator，再生成；
5. 重新运行完整 gate。

不得只为让 CI 变绿而提交无法解释的 generated diff。
