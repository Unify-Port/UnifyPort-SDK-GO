# 配置

[English](../configuration.md) | [简体中文](configuration.md)

`NewClient(Config)` 返回 client 前会校验所有安全敏感配置。除 required field 外，零值采用文档定义的默认值。

## Config 字段

| 字段 | 必填 | 默认值 | 用途与边界 |
| --- | --- | --- | --- |
| `BaseURL` | 是 | 无 | Device API 绝对 base URL；不能包含 user info、query 或 fragment |
| `APIKeyProvider` | 是 | 无 | 每个逻辑 operation 只解析一次 `X-Api-Key` |
| `HTTPClient` | 否 | 安全默认 client | 复用调用方 connection/TLS 设置，redirect、cookie 和 timeout 行为由 SDK 控制 |
| `Timeout` | 否 | 30 秒 | `executeOperation` 共享预算，覆盖 request build、credential lookup、attempt、backoff、response 读取和解析 |
| `MaxResponseBytes` | 否 | 8 MiB | 实际 response 读取上限；硬上限为 64 MiB |
| `Retry` | 否 | 重试 2 次、200 ms base、5 s 最大 delay | 只对显式标记 retryable 的 operation 生效 |
| `AllowInsecureHTTP` | 否 | `false` | 只有显式 loopback 开发 URL 可以使用 HTTP |

缺少 `BaseURL`/`APIKeyProvider`、负 duration/count 或 response limit 超过硬上限时返回
`ConfigurationError`。Configuration error 使用稳定 message，不回显 key 或完整不可信 URL。

预算从 `executeOperation` 执行入口启动，后续所有阶段沿用同一个 deadline。Request build/marshal 和 response
parse 同步执行，不会被强制中断；它们返回后会立即检查预算，若已过期则返回 `TimeoutError`，绝不返回成功结果。

## 安全构造

```go
package main

import (
	"fmt"
	"os"
	"time"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func requiredEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		panic(fmt.Sprintf("missing required environment variable: %s", name))
	}
	return value
}

func newClient() (*unifyport.Client, error) {
	return unifyport.NewClient(unifyport.Config{
		BaseURL: requiredEnv("UNIFYPORT_DEVICE_API_BASE_URL"),
		APIKeyProvider: unifyport.StaticAPIKey(
			requiredEnv("UNIFYPORT_DEVICE_API_KEY"),
		),
		Timeout: 20 * time.Second,
	})
}

func main() {
	client, err := newClient()
	if err != nil {
		panic(err)
	}
	_ = client
}
```

示例在 runtime 读取 credential，不把真实 API key 放入源码或 build output。`StaticAPIKey` 把值保存在 provider
closure 内，但应用仍不能打印配置或 closure environment。

## 轮换或托管 credential

Secret manager 提供当前值时，使用 `APIKeyProvider`：

```go
provider := func(ctx context.Context) (string, error) {
	return secretStore.CurrentDeviceAPIKey(ctx)
}
```

Callback 接收 operation context，必须遵守 cancellation。每个逻辑 operation 调用一次，而不是每个 retry
attempt 调用一次。不要记录 provider error，也不要用包含 secret-manager path 或值的信息包装它；SDK 有意不
暴露 callback cause。

## Base URL 规则

可接受的生产格式：

```text
https://api.example.com/device
```

Client 保留配置的 path prefix，并拒绝超出该范围的生成 request。包含 credential、query、fragment、不支持
scheme 或非 loopback HTTP host 的 URL 均无效。

只对本地 test server：

```go
unifyport.Config{
	BaseURL:           "http://127.0.0.1:8080",
	APIKeyProvider:    unifyport.StaticAPIKey("synthetic-test-key"),
	AllowInsecureHTTP: true,
}
```

远端 host 或生产部署绝不能开启 `AllowInsecureHTTP`。

## 自定义 HTTP client

可选 `*http.Client` 可以提供自定义 `Transport`、proxy、TLS 配置或 connection pool。SDK 会 clone client，并
接管单次调用安全行为：

- 保持禁止 redirect；
- 不使用 cookie jar；
- SDK operation timeout 仍是权威上限；
- 认证仍由 SDK 管理。

不要使用会改写目标、捕获 header/body、注入 credential 或内部执行 redirect 的自定义 `RoundTripper`。Test
double 必须使用合成数据。

## 单次调用收紧

每个生成 method 接受零个或多个 `CallOption`：

```go
result, err := client.GetWorkspace(
	ctx,
	unifyport.WithTimeout(5*time.Second),
	unifyport.WithMaxRetries(1),
)
```

可用 option：

- `WithTimeout(duration)`：取 call 和 client timeout 的较小值；
- `WithMaxRetries(count)`：取 call 和 client retry 次数的较小值；
- `WithoutRetry()`：关闭本次调用 retry。

Option 不能让 non-retryable operation 获得重试、延长全局 timeout、增加全局 retry 次数、改变 response
limit、替换认证或改变目标 URL。
