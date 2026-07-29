# Retry and timeout behavior

[English](retry.md) | [简体中文](zh-CN/retry.md)

## Conservative by default

Automatic retry is an operation-policy decision, not an HTTP-method heuristic. The generated manifest
currently marks 19 operations retryable; [API coverage](api-coverage.md) lists them. All unknown or new
operations start with retry disabled until safety is reviewed.

Writes, destructive actions, account-authorization submissions, and operations returning one-time
secrets are not made retryable merely because a similar request appears idempotent.

## Default budgets

| Setting | Default | Hard behavior |
| --- | ---: | --- |
| Logical-operation timeout | 30 seconds | Finite deadline budget; a call may only tighten it; synchronous stages are non-preemptible |
| Additional attempts | 2 | `RetryConfig.MaxRetries` must be between 0 and 10 |
| Base delay | 200 ms | Non-negative and bounded by maximum delay |
| Maximum delay | 5 seconds | Caps jitter and `Retry-After` |

`MaxRetries` means attempts after the initial request. A default value of 2 therefore permits at most
3 total attempts for an allowlisted operation. Set a non-nil `RetryConfig` with `MaxRetries: 0` to
disable retry globally, or use `WithoutRetry()` for one call.

## Total timeout boundary

The effective timeout starts at the `executeOperation` execution entry. One deadline covers
synchronous request construction and marshaling, credential lookup, every HTTP attempt, retry waits,
bounded response reading, and response parsing; entering the transport never resets that deadline.

Synchronous request construction and parsing cannot be safely preempted without leaking work into a
goroutine. The SDK therefore lets either stage return, checks the same deadline immediately, and
returns `TimeoutError` instead of a successful result if the budget expired. If request construction
consumes the budget, neither the credential provider nor HTTP is invoked.

## Retryable failures

An allowlisted operation may retry:

- a transport failure before a usable HTTP response;
- HTTP `408`, `429`, `502`, `503`, or `504`.

Other HTTP statuses are returned without automatic replay. A retry response is drained only through a
finite limit and then closed.

## Backoff

A valid `Retry-After` delta or HTTP date is honored but capped by `RetryConfig.MaxDelay`. Otherwise the
runtime uses full jitter between zero and an exponentially increasing cap. The cap itself never exceeds
the configured maximum delay.

This reduces synchronized retry traffic while keeping context-aware credential, HTTP, response-read,
and retry-wait work inside the shared deadline. The runtime does not sleep after context cancellation
or when the next attempt would no longer be allowed.

## Credential and body behavior

The API-key provider runs once before attempts begin, and the same value is reused. A request body is
retried only when it can be recreated safely. Operation input, credentials, body content, and raw
transport errors are never added to retry errors or diagnostics.

## Call options

```go
result, err := client.GetWorkspace(
	ctx,
	unifyport.WithTimeout(4*time.Second),
	unifyport.WithMaxRetries(1),
)
```

The effective value is always the stricter client/call value. `WithMaxRetries` cannot enable retry for
an operation whose generated policy says `retryable: false`.

`context.Context` cancellation takes precedence over SDK retry. A caller deadline shorter than
`WithTimeout` also wins.

## Application guidance

- Do not wrap non-idempotent SDK calls in a generic retry library.
- Do not assume a timeout means the server did not apply a write.
- Use returned request IDs for support correlation, never raw bodies or credentials.
- When a public protocol later adds an idempotency key, update the contract, policy, tests, and docs
  before enabling retry for that operation.
