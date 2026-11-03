# Node/Go parity

[English](../node-parity.md) | [简体中文](node-parity.md)

## Baseline

`parity/source.json` 记录本 Go SDK 采用的精确公开 Node SDK baseline 和 contract digest。当前 baseline 为：

| 项目 | 值 |
| --- | --- |
| Node package | `@unifyport/sdk-node` `0.4.0` |
| 公开 Node repository | `https://github.com/Unify-Port/UnifyPort-SDK-Nodejs.git` |
| 公开 Node commit | `ce60499d7169c860ed3f64a686e04bff44316f35` |
| Contract SHA-256 | `8190a34dc8ed60476f79e00b37d9dea418082886724ee6d64efa389f440adffc` |
| Operation | 78 |
| Component schema | 225 |
| Resource tag | 13 |

仓库内契约和生成 manifest 是权威信息。本文数量是验收预期；baseline 变化时必须重新生成。

Baseline kind 为 `approved-source`。表中的包版本是该 commit 的源码元数据，不表示 npm 该版本已发布本次能力。

## 覆盖预期

Go SDK 必须保留 workspace、API key、provider、account、account authorization、account runtime、
conversation、contact、group、message 和 webhook endpoint 的全部 78 个公开 operation。不能因为路由属于
provider 账号或安全策略保守而遗漏 operation。

Policy baseline 包含：

| 分类 | 预期数量 |
| --- | ---: |
| 显式 retryable operation | 19 |
| Destructive operation | 8 |
| Secret-input operation | 9 |
| Secret-output operation | 16 |

每个 operation 的 method、path、tag、Go method、request/response type、retry flag、mutability 和 secret flag
应由生成 parity manifest 枚举，不能只依赖本文数量。

## Module 映射

| Node SDK 概念 | Go SDK 概念 | Parity 规则 |
| --- | --- | --- |
| `contracts/device.openapi.yaml` | `contracts/device.openapi.yaml` | 精确已批准公开快照 |
| `UnifyPortDeviceClient` | `Client` | 同样 78 个 operation ID，使用 Go 风格名称 |
| 生成 schema | 生成 public model 和 `internal/wire` model | 225 个 schema 全部可追踪 |
| 生成 operation catalog | 生成 Go operation 和 parity manifest | Method/path/policy 精确一致 |
| `DeviceClientConfig` | `Config` | 相同安全默认值和有限上限 |
| `RetryConfig` | `RetryConfig` | 相同默认值与硬上限意图 |
| Request execution option | Go call option 与 `context.Context` | 只能收紧/关闭预算 |
| `ApiResult<T>` | `Result[T]` | 全部 envelope 字段和安全 response 元数据 |
| `runtime.ts` | Go 安全 transport/runtime | 相同认证、retry、limit 和脱敏语义 |
| Node error class | Typed Go error | 语义类别逐一对应 |
| `paginateCursor` | `Pager[T]` | 相同 cursor 安全和默认页数上限 |
| Operation policy | `policy/operation-policies.json` | 相同 retry/mutability/secret 决策 |

## 对齐行为

- `X-Api-Key` 只能来自 client 配置，每个逻辑 operation 只解析一次。
- Credential 保留在配置的 origin/path prefix 内，且永远不跨 redirect。
- 默认 HTTPS；只有显式 loopback 开发环境可以使用 HTTP。
- 只有 19 个 allowlist operation 自动重试。
- Retryable failure 包括 transport error 和 `408`、`429`、`502`、`503`、`504`，使用有上限的
  `Retry-After` 或 full jitter。
- 单次调用可以取消、关闭或收紧 timeout/retry，不能扩大全局限制。
- 共享 timeout 从 `executeOperation` 执行入口启动，覆盖 build、credential、HTTP/retry/read 和 parse 且不会
  重置；同步 build/parse 可能在 deadline 后才完成，但只能返回 `TimeoutError`，不能成功。
- Body 读取实际执行配置字节限制，不信任 `Content-Length`。
- 公开 error 只暴露过滤后的 status/code/request ID/operation ID，不包含 raw body/header、上游 detail 或
  cause。
- Pagination 拒绝 missing/repeated cursor、遵守 cancellation，并执行相同的默认最大页数。
- 两种 SDK 都不提供可绕过 operation policy 的通用 request 方法。

## 有意的 Go 差异

| 差异 | 原因与影响 |
| --- | --- |
| 使用 `Client` 而非 `UnifyPortDeviceClient` | Module 本身只服务 Device API；短名称符合 Go 习惯。 |
| `ListAPIKeys`、`CheckAccountQRAuth` 等名称 | Go initialism 与 lower-camel TypeScript 不同，manifest 仍保留 operation-ID 映射。 |
| 使用 `context.Context` 而非 `AbortSignal` | Go 标准 cancellation/deadline 传递方式。 |
| 每个 operation 使用独立 request struct | 避免契约新增 optional input 时产生位置参数变更。 |
| Envelope 字段扁平化到 `Result[T]` | `Data`、request identifier、status 和过滤 header 以 Go 习惯保留每个公开 envelope 字段。 |
| Schema 允许时使用精确 Go `int64`/`uint64` | Go 没有 JavaScript safe-integer 限制；调用方仍不能缩窄数值。 |
| 显式 `NoContent` result | Go 没有 `undefined`；`204` 必须与空 JSON 值区分。 |
| 安全 response 元数据而非 raw `Response` | 避免常规 result/error 日志展开认证或 server header。 |
| 支持 `errors.As`/`errors.Is` 的 typed Go error | 以 Go 习惯检查错误，同时不暴露 raw cause。 |
| 不预取的 `Pager[T]` | Context-aware iteration，不产生隐藏 goroutine 或泄漏工作。 |
| 隔离的 nested generator module 且不嵌入 spec | Code-generation tool 不进入公开 runtime dependency graph。 |
| 不包含 MCP runtime | MCP 属于 Node workspace；保留的 policy 字段只作为 parity 元数据。 |

这些是生态适配，不代表可以改变 wire contract 或安全语义。公开 response metadata schema 若新增字段，
generation/parity check 必须扩展 `Result[T]`；静默丢弃字段属于 parity failure。

## 更新 parity

按[契约维护](contract-maintenance.md)执行。只有 contract digest、operation policy、生成 manifest、public facade、
测试和两种语言文档一致，parity 更新才算完成。Generator 变绿本身不充分，真实 provider E2E 仍是独立
release gate。
