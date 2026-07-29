# Retry 与 timeout 行为

[English](../retry.md) | [简体中文](retry.md)

## 默认保守

自动 retry 由 operation policy 决定，不根据 HTTP method 猜测。生成 manifest 当前标记 19 个 retryable
operation；[API coverage](api-coverage.md)列出了它们。所有未知或新增 operation 在安全审查完成前默认关闭
retry。

写操作、destructive action、账号授权提交和返回一次性 secret 的 operation，不会仅因相似 request 看似
idempotent 就自动变为 retryable。

## 默认预算

| 设置 | 默认值 | 硬行为 |
| --- | ---: | --- |
| 逻辑 operation timeout | 30 秒 | 有限 deadline 预算；单次调用只能收紧；同步阶段不可抢占 |
| 额外 attempt | 2 | `RetryConfig.MaxRetries` 必须在 0 到 10 之间 |
| Base delay | 200 ms | 非负，且受 maximum delay 限制 |
| Maximum delay | 5 秒 | 限制 jitter 和 `Retry-After` |

`MaxRetries` 表示首次 request 之后的 attempt 数。默认值 2 表示 allowlist operation 最多总共尝试 3 次。
通过非 nil 且 `MaxRetries: 0` 的 `RetryConfig` 全局关闭 retry，或用 `WithoutRetry()` 关闭单次调用 retry。

## 总 timeout 边界

有效 timeout 从 `executeOperation` 执行入口启动。同步 request build/marshal、credential lookup、所有 HTTP
attempt、retry wait、有界 response 读取和 response parse 共用一个 deadline；进入 transport 不会重置它。

同步 request build 和 parse 无法在不泄漏 goroutine work 的前提下安全抢占，因此 SDK 让它们同步返回后立即
检查同一 deadline。若预算已过期，只返回 `TimeoutError`，不返回成功结果；若 request build 已耗尽预算，
credential provider 和 HTTP 都不会执行。

## Retryable failure

Allowlist operation 可以重试：

- 获得可用 HTTP response 之前的 transport failure；
- HTTP `408`、`429`、`502`、`503` 或 `504`。

其他 HTTP status 不自动重放。Retry response 只在有限上限内 drain，随后关闭。

## Backoff

合法 `Retry-After` delta 或 HTTP date 会被遵守，但受 `RetryConfig.MaxDelay` 限制；否则 runtime 在零到指数
增长 cap 之间使用 full jitter。Cap 本身不会超过配置 maximum delay。

这可以减少同步 retry traffic，同时让可感知 context 的 credential、HTTP、response read 和 retry wait 共用
同一 deadline。Context 已取消或不再允许下一次 attempt 时，runtime 不继续 sleep。

## Credential 与 body 行为

API-key provider 在 attempt 开始前只执行一次，所有 attempt 复用同一个值。只有能够安全重建 request body 时
才会重试。Operation input、credential、body content 和 raw transport error 都不会进入 retry error 或诊断
信息。

## Call option

```go
result, err := client.GetWorkspace(
	ctx,
	unifyport.WithTimeout(4*time.Second),
	unifyport.WithMaxRetries(1),
)
```

有效值始终取 client/call 中更严格的一个。若生成 policy 是 `retryable: false`，`WithMaxRetries` 不能为该
operation 开启 retry。

`context.Context` cancellation 优先于 SDK retry。Caller deadline 短于 `WithTimeout` 时也优先生效。

## 应用建议

- 不要用通用 retry library 包装 non-idempotent SDK call。
- 不要假设 timeout 意味着 server 没有应用写操作。
- 使用返回 request ID 做支持关联，不使用 raw body 或 credential。
- 公开协议未来增加 idempotency key 时，必须先更新 contract、policy、test 和 docs，再为 operation 开启 retry。
