# 入门

[English](../getting-started.md) | [简体中文](getting-started.md)

本指南创建 Device API client，调用一个生成 operation，并在不把 credential 或业务 payload 写入日志的前提下
处理结果。

## 要求

- Go 1.25 或 1.26 支持线中的最新安全 patch；
- Device API base URL；
- 具有所需 workspace 最小权限的 Device API key。

应用代码应使用 tagged release：

```bash
go get github.com/Unify-Port/UnifyPort-SDK-GO@latest
```

为了可复现 build，应提交 `go.mod` 和 `go.sum`，并用已审查 release 版本替代 `@latest`。默认 branch 是开发
状态，不代表 release 保证。

## 配置 credential

通过 runtime environment 或 secret manager 提供值：

```bash
export UNIFYPORT_DEVICE_API_BASE_URL="<device-api-base-url>"
export UNIFYPORT_DEVICE_API_KEY="<api-key>"
```

上面的 placeholder 不是可用 credential。避免把真实 secret 输入共享 shell history；优先使用部署平台的
secret-injection 机制。

## 创建 client 并调用 operation

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func main() {
	client, err := unifyport.NewClient(unifyport.Config{
		BaseURL: requiredEnv("UNIFYPORT_DEVICE_API_BASE_URL"),
		APIKeyProvider: unifyport.StaticAPIKey(
			requiredEnv("UNIFYPORT_DEVICE_API_KEY"),
		),
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := client.GetWorkspace(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("status=%d request_id=%s\n", result.StatusCode, result.RequestID)
	_ = result.Data
}

func requiredEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		log.Fatalf("missing required environment variable: %s", name)
	}
	return value
}
```

完整示例位于 [`examples/getting-started`](../../examples/getting-started/main.go)，只输出过滤后的诊断信息，不输出
workspace data 或 header。

## 理解 `Result[T]`

每个 operation 返回 `Result[T]` 和 error：

| 字段 | 含义 |
| --- | --- |
| `Data` | Operation typed business data；`204` 时为 `NoContent` |
| `StatusCode` | 成功 HTTP status |
| `RequestID` | 过滤后的公开 envelope/header request ID |
| `ClientRequestID` | 过滤后的公开 response-envelope client request ID |
| `Headers` | Allowlist、规范化的诊断/content/rate-limit snapshot |

这些字段共同保留当前公开成功 envelope 的全部字段，并增加安全 transport 元数据。不暴露 raw
`http.Response`、body 或无限制 header map。

不要记录完整 result。即使 transport 元数据安全，某些 operation 仍会返回一次性 credential 或业务数据。

## 调用带输入的 operation

生成 method 使用两种形式之一：

```go
result, err := client.GetWorkspace(ctx, options...)
result, err := client.OperationName(ctx, input, options...)
```

每个包含 parameter 或 request body 的 operation 都有一个专用 `...Input` struct，避免契约新增 optional field
时改变位置参数。先通过 [API coverage](api-coverage.md)找到 Go method，再查看生成声明：

```bash
go doc github.com/Unify-Port/UnifyPort-SDK-GO.Client
go doc github.com/Unify-Port/UnifyPort-SDK-GO
```

`parity/manifest.gen.json` 也记录 wire request/response 映射和引用 schema。协议字段描述和约束仍以 OpenAPI
contract 为权威。

## 收紧单次调用

```go
result, err := client.GetWorkspace(
	ctx,
	unifyport.WithTimeout(5*time.Second),
	unifyport.WithMaxRetries(1),
)
```

Call option 只能收紧 client 预算。应用要求单次 attempt 时使用 `WithoutRetry()`。Call option 不能让 unsafe
operation 获得 retry。

默认值和硬上限见[配置](configuration.md)和 [Retry 与 timeout](retry.md)。

## 处理 error

```go
var apiErr *unifyport.APIError
switch {
case errors.Is(err, context.Canceled):
	// The caller canceled the operation.
case errors.Is(err, context.DeadlineExceeded):
	// The caller or SDK deadline expired.
case errors.As(err, &apiErr):
	// Use only filtered status/code/request ID fields.
default:
	// Return a generic application error; do not dump requests or responses.
}
```

所有 typed category 和安全日志规则见 [Error 处理](errors.md)。

## Pagination

不同资源的 list-response 形状不同，因此要把生成 list operation 适配到 `PageFetcher[T]`。`Pager[T]` 随后在
不 prefetch 的情况下执行 context cancellation、missing/repeated cursor 检测和有限 page limit。参见
[Cursor pagination](pagination.md)和离线 [`examples/pagination`](../../examples/pagination/main.go)。

## 生产检查表

- 从 secret manager 或受保护 runtime environment 加载 API key。
- 使用 HTTPS；loopback test 之外保持关闭 `AllowInsecureHTTP`。
- 设置符合业务操作的 caller context deadline。
- 不要在 write 或 authorization operation 外再套通用 retry。
- 只记录 operation ID、过滤后的 status/code/request ID 和已批准应用上下文。
- 默认不记录 `Result.Data`、request struct、header、provider error 或 credential。
- 固定已审查 SDK release，并在消费应用中运行 `govulncheck`。

离线编译不能证明账号已授权或真实 provider workflow 可用。真实 E2E 应在受控 staging/provider 环境单独执行，
并把 credential 和数据留在本仓库之外。
