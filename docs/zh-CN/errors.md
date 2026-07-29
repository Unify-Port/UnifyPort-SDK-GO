# Error 处理

[English](../errors.md) | [简体中文](errors.md)

SDK 返回稳定 typed error，应用不需要解析 message 就能分支。使用 `errors.As` 判断类别，使用 `errors.Is`
判断 context cancellation/deadline 语义。

## Error 类别

| Type | 含义 | 安全公开字段 |
| --- | --- | --- |
| `ConfigurationError` | Client、call option、context 或 request 构造违反 SDK 边界 | `Field`、稳定 `Reason` |
| `NetworkError` | 未获得可用 HTTP response | `OperationID` |
| `TimeoutError` | 有限 SDK/caller deadline 到期 | `OperationID`、有效 `Timeout` |
| `CanceledError` | Caller context 被取消 | `OperationID` |
| `APIError` | Server 返回 non-success status | `StatusCode`、过滤后 `Code`、`RequestID`、`OperationID` |
| `ResponseParseError` | Response 超限、无法读取或不符合公开契约 | `StatusCode`、过滤后 `RequestID`、`OperationID` |
| `PaginationError` | Cursor invariant、page limit 或 pagination context 阻止安全继续 | `Reason`、`MaxPages` |

公开 error 不保留 raw response/header map/body、server message/details、API key、credential provider error、
provider payload 或底层 transport cause。Error message 由 SDK 生成，可以安全写入日志；应用行为仍应使用 type
和 field，不能解析 message text。

## 常见处理方式

```go
result, err := client.GetWorkspace(ctx)
if err != nil {
	var apiErr *unifyport.APIError
	var parseErr *unifyport.ResponseParseError

	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("workspace request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("workspace request timed out")
	case errors.As(err, &apiErr):
		return fmt.Errorf(
			"workspace API failure: status=%d code=%s request_id=%s",
			apiErr.StatusCode,
			apiErr.Code,
			apiErr.RequestID,
		)
	case errors.As(err, &parseErr):
		return fmt.Errorf("workspace response could not be parsed")
	default:
		return fmt.Errorf("workspace request failed")
	}
}

_ = result.Data
```

上面的 wrapper message 有意不在 operational logging 中使用 `%w`。应用若需要内部 error chain，必须先确认
目标 logger 不会序列化可能含 secret 的 argument、context、request 或 custom value。

## API error

`APIError.Code` 和 `RequestID` 经过字符 allowlist 和长度限制。如果 server body 没有安全 code，`Code` 回退为
`HTTP_<status>`。Server 的人类可读 message/details 会被忽略，因为它们可能回显 request input 或上游
provider content。

除非公开 Device API 契约记录了某个 `Code`，不要假设 code 列表 exhaustive。保留未知安全 code 供诊断，
业务分支优先使用契约已记录的 status/code 组合。

## Timeout 与 cancellation

`TimeoutError` 支持 `errors.Is(err, context.DeadlineExceeded)`；`CanceledError` 支持
`errors.Is(err, context.Canceled)`。这样既保留 Go 常用控制流，也不暴露原始 context cause。Timeout 不能证明
改变状态的 server operation 没有生效。

Timeout 覆盖 request build、credential lookup、HTTP/retry/body read 和 response parse。同步 build 或 parse
不会被强制抢占，因此可能在 deadline 后才返回；SDK 随后只返回 `TimeoutError`，并丢弃任何已解析成功值。

Pagination 使用自己的 `PaginationError` 表示 context 结果，同时支持相同 `errors.Is` 检查。

## Response 解析

`ResponseParseError` 覆盖 body-limit violation、读取失败、malformed payload 和不符合生成 parser 的 response。
如果存在安全 status/request ID 会保留，但不会保留 body 或 parser cause。应使用公开契约和 request ID 排查
upstream/API 兼容性，不能把 raw-body logging 当作调试捷径。

## 成功元数据

`Result[T]` 不是 error object。它通过 `Data`、`RequestID` 和 `ClientRequestID` 保留全部公开 response-envelope
字段，并提供 `StatusCode` 和 allowlist `HeaderSnapshot`。Snapshot 只包含规范化的诊断/content/rate-limit
header；通过 `Get`、`Values` 或 `Clone` 读取，不存在可达 raw response。

即使 transport header 已过滤，业务数据（包括一次性 secret）仍可能敏感。不要记录完整 `Result[T]`。
