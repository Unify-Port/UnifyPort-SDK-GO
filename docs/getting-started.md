# Getting started

[English](getting-started.md) | [简体中文](zh-CN/getting-started.md)

This guide creates a Device API client, calls one generated operation, and handles the result without
placing credentials or business payloads in logs.

## Requirements

- the latest security patch in the supported Go 1.25 or 1.26 release line;
- a Device API base URL;
- a Device API key scoped to the required workspace permissions.

Use a tagged release in application code:

```bash
go get github.com/Unify-Port/UnifyPort-SDK-GO@latest
```

For reproducible builds, commit `go.mod` and `go.sum` and replace `@latest` with a reviewed release
version. The default branch is development state, not a release guarantee.

## Configure credentials

Provide values through the runtime environment or a secret manager:

```bash
export UNIFYPORT_DEVICE_API_BASE_URL="<device-api-base-url>"
export UNIFYPORT_DEVICE_API_KEY="<api-key>"
```

The placeholders above are not usable credentials. Avoid entering real secrets into shared shell
history; prefer the secret-injection mechanism of the deployment platform.

## Create a client and call an operation

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

The complete example is in [`examples/getting-started`](../examples/getting-started/main.go). It prints
only filtered diagnostics, not workspace data or headers.

## Understand `Result[T]`

Every operation returns `Result[T]` and an error:

| Field | Meaning |
| --- | --- |
| `Data` | The operation's typed business data, or `NoContent` for `204` |
| `StatusCode` | Successful HTTP status |
| `RequestID` | Filtered public envelope/header request ID |
| `ClientRequestID` | Filtered public response-envelope client request ID |
| `Headers` | Allowlisted, normalized diagnostic/content/rate-limit snapshot |

Together these fields preserve every field in the current public success envelope and add safe
transport metadata. A raw `http.Response`, body, and unrestricted header map are not exposed.

Do not log a whole result. Some operations return one-time credentials or business data even when
their transport metadata is safe.

## Call operations with input

Generated methods follow one of two shapes:

```go
result, err := client.GetWorkspace(ctx, options...)
result, err := client.OperationName(ctx, input, options...)
```

Each operation with parameters or a request body has one operation-specific `...Input` struct. This
avoids positional-argument changes as optional contract fields are added. Use
[API coverage](api-coverage.md) to find the Go method, then inspect the generated declaration with:

```bash
go doc github.com/Unify-Port/UnifyPort-SDK-GO.Client
go doc github.com/Unify-Port/UnifyPort-SDK-GO
```

`parity/manifest.gen.json` also records the wire request/response mapping and referenced schemas. The
OpenAPI contract remains authoritative for protocol field descriptions and constraints.

## Tighten a call

```go
result, err := client.GetWorkspace(
	ctx,
	unifyport.WithTimeout(5*time.Second),
	unifyport.WithMaxRetries(1),
)
```

Call options can only tighten the client budget. Use `WithoutRetry()` when the application requires a
single attempt. A call option cannot make an unsafe operation retry.

See [Configuration](configuration.md) and [Retry and timeout](retry.md) for defaults and hard limits.

## Handle errors

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

See [Error handling](errors.md) for all typed categories and safe logging rules.

## Paginate

List-response shapes differ by resource, so adapt the generated list operation to `PageFetcher[T]`.
`Pager[T]` then enforces context cancellation, missing/repeated cursor detection, and a finite page
limit without prefetch. See [Cursor pagination](pagination.md) and the offline
[`examples/pagination`](../examples/pagination/main.go) program.

## Production checklist

- Load the API key from a secret manager or protected runtime environment.
- Use HTTPS; keep `AllowInsecureHTTP` disabled outside loopback tests.
- Set a caller context deadline appropriate for the business action.
- Do not add generic retry around writes or authorization operations.
- Log only operation ID, filtered status/code/request ID, and approved application context.
- Never log `Result.Data`, request structs, headers, provider errors, or credentials by default.
- Pin a reviewed SDK release and run `govulncheck` in the consuming application.

Offline compilation does not prove that an account is authorized or a real provider workflow works.
Perform controlled staging/provider end-to-end validation separately and keep its credentials and data
outside this repository.
