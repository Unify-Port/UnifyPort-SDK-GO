# Node/Go parity

[English](node-parity.md) | [简体中文](zh-CN/node-parity.md)

## Baseline

`parity/source.json` records the exact public Node SDK baseline and contract digest used by this Go
SDK. At the current baseline:

| Item | Value |
| --- | --- |
| Node package | `@unifyport/sdk-node` `0.4.0` |
| Public Node repository | `https://github.com/Unify-Port/UnifyPort-SDK-Nodejs.git` |
| Public Node commit | `ce60499d7169c860ed3f64a686e04bff44316f35` |
| Contract SHA-256 | `8190a34dc8ed60476f79e00b37d9dea418082886724ee6d64efa389f440adffc` |
| Operations | 78 |
| Component schemas | 225 |
| Resource tags | 13 |

The checked-in contract and generated manifest are authoritative. Counts in this document are
acceptance expectations and must be regenerated when the baseline changes.

The baseline kind is `approved-source`. The package version above is source metadata at that commit, not a claim that the npm release includes these capabilities.

## Coverage expectations

The Go SDK must preserve all 78 public operations across workspace, API keys, providers, accounts,
account authorization, account runtime, conversations, contacts, groups, messages, and webhook
endpoints. No operation may disappear because its route belongs to a provider account or because its
security policy is conservative.

The policy baseline contains:

| Classification | Expected count |
| --- | ---: |
| Explicitly retryable operations | 19 |
| Destructive operations | 8 |
| Secret-input operations | 9 |
| Secret-output operations | 16 |

The generated parity manifest, rather than these prose counts, must enumerate every operation and its
method, path, tag, Go method, request/response type, retry flag, mutability, and secret flags.

## Module mapping

| Node SDK concept | Go SDK concept | Parity rule |
| --- | --- | --- |
| `contracts/device.openapi.yaml` | `contracts/device.openapi.yaml` | Exact approved public snapshot |
| `UnifyPortDeviceClient` | `Client` | Same 78 operation IDs through Go-style names |
| Generated schemas | Generated public models and `internal/wire` models | All 225 schemas traceable |
| Generated operation catalog | Generated Go operations and parity manifest | Method/path/policy exact |
| `DeviceClientConfig` | `Config` | Same security defaults and finite limits |
| `RetryConfig` | `RetryConfig` | Same default and hard-limit intent |
| Request execution options | Go call options plus `context.Context` | May only tighten/disable budgets |
| `ApiResult<T>` | `Result[T]` | All envelope fields plus safe response metadata |
| `runtime.ts` | Secure Go transport/runtime | Same authentication, retry, limits, and redaction |
| Node error classes | Typed Go errors | One-to-one semantic categories |
| `paginateCursor` | `Pager[T]` | Same cursor safety and default page bound |
| Operation policy | `policy/operation-policies.json` | Same retry/mutability/secret decisions |

## Aligned behavior

- `X-Api-Key` comes only from client configuration and is resolved once per logical operation.
- Credentials remain inside the configured origin/path prefix and never cross a redirect.
- HTTPS is the default; only explicit loopback development may use HTTP.
- Only the 19 allowlisted operations receive automatic retry.
- Retryable failures are transport errors and `408`, `429`, `502`, `503`, or `504`, with bounded
  `Retry-After` or full jitter.
- A single call may cancel, disable, or tighten timeout/retry behavior but cannot expand global limits.
- The shared timeout starts at the `executeOperation` execution entry and covers build, credential,
  HTTP/retry/read, and parse without reset; synchronous build/parse may finish after the deadline only
  to return `TimeoutError`, never success.
- Body reads enforce the configured byte limit rather than trusting `Content-Length`.
- Public errors expose filtered status/code/request ID/operation ID and no raw body, headers, upstream
  detail, or cause.
- Pagination rejects missing/repeated cursors, observes cancellation, and enforces the same default
  maximum page count.
- Neither SDK exposes a generic request method that can bypass operation policy.

## Intentional Go differences

| Difference | Reason and impact |
| --- | --- |
| `Client` instead of `UnifyPortDeviceClient` | The module itself is Device-API-specific; the shorter name is idiomatic Go. |
| `ListAPIKeys`, `CheckAccountQRAuth`, and similar names | Go initialisms differ from lower-camel TypeScript while preserving operation-ID mapping in the manifest. |
| `context.Context` instead of `AbortSignal` | Standard Go cancellation/deadline propagation. |
| Operation-specific request structs | Avoid positional argument churn when the contract gains optional inputs. |
| Envelope fields are flattened into `Result[T]` | `Data`, request identifiers, status, and filtered headers preserve every public envelope field in an idiomatic Go result. |
| Exact Go `int64`/`uint64` where the schema permits | Go does not have JavaScript's safe-integer limitation; callers still must not narrow values. |
| Explicit `NoContent` result | Go has no `undefined`; `204` must remain distinguishable from an empty JSON value. |
| Safe response metadata rather than a raw `Response` | Prevent normal result/error logging from expanding authentication or server headers. |
| Typed Go errors with `errors.As`/`errors.Is` | Idiomatic inspection without exposing raw causes. |
| `Pager[T]` without prefetch | Context-aware iteration without hidden goroutines or leaked work. |
| Isolated nested generator module and no embedded spec | Keeps code-generation tools out of the public runtime dependency graph. |
| No MCP runtime | MCP is a Node workspace concern; retained policy fields are parity metadata only. |

These are ecosystem adaptations, not permission to change the wire contract or security semantics. If
the public response metadata schema gains a field, generation and parity checks must extend
`Result[T]`; silently discarding it is a parity failure.

## Updating parity

Follow [contract maintenance](contract-maintenance.md). A parity update is complete only when the
contract digest, operation policy, generated manifest, public facade, tests, and both documentation
languages agree. A green generator alone is insufficient, and real-provider E2E remains a separate
release gate.
