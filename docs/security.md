# Security boundaries

[English](security.md) | [简体中文](zh-CN/security.md)

## Threat model

The SDK is designed to reduce these risks:

- API keys, provider authorization codes/passwords/session payloads, webhook secrets, or one-time
  outputs entering source, logs, errors, fixtures, or public artifacts;
- credentials being sent to an unintended origin/path or forwarded across a redirect;
- automatic retry repeating a state-changing or secret-bearing operation;
- unbounded responses, retry delays, or pagination consuming process resources;
- raw upstream error content or transport causes entering telemetry;
- integer narrowing causing a caller to target the wrong resource;
- generated or raw clients bypassing reviewed operation policy.

The SDK does not replace application secret management, API-key rotation, authorization policy, or
real-provider validation.

## Configuration and URL confinement

Client construction rejects a base URL that is relative, contains user information, query, or
fragment, or uses an unsupported scheme. HTTPS is required by default. Plain HTTP can be enabled only
explicitly for loopback development.

The runtime normalizes the base path and ensures every generated target remains within the same
scheme, host, effective port, and path prefix. Operation input cannot supply a base URL or arbitrary
path. Redirect following is disabled so an authentication header cannot be replayed to another target.

Do not enable the loopback exception in production. Do not put credentials in a URL, including test
URLs.

## Credential lifecycle

`X-Api-Key` comes only from the configured `APIKeyProvider`. The provider is resolved once per logical
operation; retries reuse the result so a rotating callback cannot change identity halfway through a
call. A provider error becomes a stable configuration/credential error without retaining its message
or cause.

Credentials must never appear in:

- operation request structs or call options;
- `String`, Go-syntax formatting, JSON, errors, or logs;
- test snapshots, examples, `.env` files, or generated artifacts;
- URL query strings, telemetry attributes, or retry diagnostics.

Applications should load keys from a secret manager or runtime environment and scope each key to the
minimum required workspace permissions.

## Retry, timeout, and cancellation

Retries require an explicit `retryable` policy. Transport failures and `408`, `429`, `502`, `503`, or
`504` are temporary only for allowlisted operations. The runtime honors a valid bounded `Retry-After`;
otherwise it uses capped full jitter. Attempts and wait durations always have finite limits.

Writes, destructive actions, account-authorization submissions, and operations returning a one-time
secret are not retried automatically. An HTTP method alone is never proof of idempotency.

Every call has a finite SDK deadline budget and accepts `context.Context`. Cancellation promptly stops
context-aware credential, HTTP, backoff, and response-read work and prevents another attempt. Per-call
options may tighten but not extend configured budgets.

The deadline starts at the `executeOperation` execution entry and is shared by synchronous request
build/marshal, credential lookup, HTTP/retry, bounded body reading, and parsing. Build and parse are
kept synchronous rather than moved into leak-prone goroutines; after either completes, an expired
deadline returns a sanitized `TimeoutError` and cannot produce success.

## Response and error handling

The runtime limits bytes while reading instead of trusting `Content-Length`. Oversized, truncated,
empty, non-JSON, and malformed responses are handled without copying the raw body into an error.

Public API errors may expose only:

- HTTP status;
- an allowlisted, length-limited API `code`;
- an allowlisted, length-limited request ID;
- the stable operation ID;
- a generic SDK-authored message.

They do not retain the raw response, headers, body, server message/details, provider payload, or an
underlying cause. Network, timeout, and cancellation errors follow the same no-cause rule while
remaining distinguishable with typed errors and `errors.Is` where appropriate.

Successful responses can contain business data or one-time secrets. Treat the returned envelope as
sensitive, avoid `%+v` logging, and copy only explicitly approved fields into telemetry.

## Secret operations

The language-neutral policy marks secret input and output independently. Secret-bearing operations
remain callable by direct application code because they are part of the public Device API, but they do
not receive automatic retry merely because another operation with the same method is safe.

One-time API keys, invite or authorization material, verification codes, passwords, session payloads,
message content, and webhook signing secrets must remain in controlled memory and must not enter
examples, tests, issue reports, or logs.

## Integer and data integrity

Generated integer types follow the contract. Review every `int64`/`uint64` change for range and signed
semantics, and do not convert identifiers or cursors through floating point. Unknown-format numeric
identifiers should remain strings in the public contract.

## Custom HTTP clients

The SDK owns authentication and redirect behavior even when the application supplies an HTTP client.
Injected clients are for connection pools, proxies, TLS settings, and offline tests; they must not be
used to inject authentication, rewrite destinations, follow redirects, or capture sensitive payloads.
Test doubles should store only the minimum synthetic fields needed for assertions.

## Dependency and release controls

- Keep generator versions pinned in `internal/generator/go.mod` and analysis tool versions pinned;
  review upgrades explicitly.
- Use a read-only GitHub Actions token and pinned action revisions.
- Run `govulncheck ./...` in both the root and `internal/generator` modules and inspect findings before
  release.
- Run `bash scripts/check-public.sh` on the complete worktree and inspect the release diff manually.
- Keep staging/provider end-to-end data outside this repository and publish only a redacted gate result.

## Review checklist

For every public behavior change, answer:

1. Can credentials leave the configured origin/path or cross a redirect?
2. Does the operation change state, and is retry safety explicitly proven?
3. Does input or output contain a secret, personal data, or a large payload?
4. Are timeout, cancellation, byte limits, and pagination bounds still finite?
5. Can any public error, result, logger, or test double expose raw data?
6. Are integer ranges and no-content behavior represented exactly?
7. Which offline tests and external gates prove the boundary?
