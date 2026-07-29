# Error handling

[English](errors.md) | [简体中文](zh-CN/errors.md)

The SDK returns stable typed errors so applications can branch without parsing messages. Use
`errors.As` for a category and `errors.Is` for context cancellation/deadline semantics.

## Error categories

| Type | Meaning | Safe public fields |
| --- | --- | --- |
| `ConfigurationError` | Client, call option, context, or request construction violates an SDK boundary | `Field`, stable `Reason` |
| `NetworkError` | No usable HTTP response was obtained | `OperationID` |
| `TimeoutError` | The finite SDK/caller deadline expired | `OperationID`, effective `Timeout` |
| `CanceledError` | Caller context was canceled | `OperationID` |
| `APIError` | The server returned a non-success status | `StatusCode`, filtered `Code`, `RequestID`, `OperationID` |
| `ResponseParseError` | A response was oversized, unreadable, or incompatible with the public contract | `StatusCode`, filtered `RequestID`, `OperationID` |
| `PaginationError` | Cursor invariants, page limits, or pagination context prevented safe continuation | `Reason`, `MaxPages` |

No public error retains a raw response, header map, body, server message/details, API key, credential
provider error, provider payload, or underlying transport cause. Error messages are SDK-authored and
stable enough for logs, but application behavior must use types and fields rather than message text.

## Typical handling

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

The wrapper messages above deliberately do not use `%w` for operational logging. If your application
needs an internal error chain, verify that the destination logger does not serialize arguments,
contexts, requests, or custom values containing secrets.

## API errors

`APIError.Code` and `RequestID` pass character allowlists and length limits. If the server body does not
contain a safe code, `Code` falls back to `HTTP_<status>`. The server's human-readable message and
details are ignored because they can echo request input or upstream provider content.

Do not treat `Code` as exhaustive unless the public Device API contract documents that value. Preserve
an unknown safe code for diagnostics and branch primarily on documented status/code combinations.

## Timeout and cancellation

`TimeoutError` supports `errors.Is(err, context.DeadlineExceeded)`. `CanceledError` supports
`errors.Is(err, context.Canceled)`. This preserves idiomatic control flow without exposing the original
context cause. A timeout does not prove that a state-changing server operation was not applied.

The timeout covers request build, credential lookup, HTTP/retry/body reading, and response parsing.
Synchronous build or parse work may finish after the deadline because it is not forcefully preempted,
but the SDK then returns `TimeoutError` and discards any parsed success value.

Pagination wraps its own context outcome in `PaginationError` while supporting the same `errors.Is`
checks.

## Response parsing

`ResponseParseError` covers body-limit violations, read failures, malformed payloads, and responses
that do not match the generated parser. It retains a safe status/request ID when available but never
the body or parser cause. Check upstream/API compatibility using the public contract and request ID;
do not add raw-body logging as a debugging shortcut.

## Successful metadata

`Result[T]` is not an error object. It preserves every public response-envelope field through `Data`,
`RequestID`, and `ClientRequestID`, plus `StatusCode` and an allowlisted `HeaderSnapshot`. The snapshot
contains only normalized diagnostic/content/rate-limit headers. Use `Get`, `Values`, or `Clone`; no raw
response remains reachable.

Business data, including a one-time secret, can still be sensitive even though transport headers are
filtered. Never log an entire `Result[T]`.
