# Configuration

[English](configuration.md) | [简体中文](zh-CN/configuration.md)

`NewClient(Config)` validates all security-sensitive configuration before returning a client. Zero
values select documented defaults except for required fields.

## Config fields

| Field | Required | Default | Purpose and boundary |
| --- | --- | --- | --- |
| `BaseURL` | Yes | none | Absolute Device API base URL; no user info, query, or fragment |
| `APIKeyProvider` | Yes | none | Resolves `X-Api-Key` once per logical operation |
| `HTTPClient` | No | secure default | Reuses caller connection/TLS settings while SDK owns redirect, cookie, and timeout behavior |
| `Timeout` | No | 30 seconds | Shared `executeOperation` budget for request build, credential lookup, attempts, backoff, response reading, and parsing |
| `MaxResponseBytes` | No | 8 MiB | Actual response-read limit; hard maximum is 64 MiB |
| `Retry` | No | 2 retries, 200 ms base, 5 s maximum delay | Applies only to operations explicitly marked retryable |
| `AllowInsecureHTTP` | No | `false` | Allows HTTP only for an explicit loopback development URL |

`BaseURL`, `APIKeyProvider`, negative durations/counts, and response limits above the hard maximum are
rejected with `ConfigurationError`. Configuration errors use stable messages and do not echo a key or
complete untrusted URL.

The budget starts at the `executeOperation` execution entry and the same deadline flows through every
later stage. Request construction/marshaling and response parsing run synchronously and are not
forcefully interrupted; immediately after either returns, an expired budget produces `TimeoutError`
and no successful result is returned.

## Secure construction

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

The example reads credentials at runtime. It does not place a real API key in source or build output.
`StaticAPIKey` keeps the value inside the provider closure, but the application must still avoid
printing the configuration or closure environment.

## Rotating or managed credentials

Use an `APIKeyProvider` when a secret manager supplies the current value:

```go
provider := func(ctx context.Context) (string, error) {
	return secretStore.CurrentDeviceAPIKey(ctx)
}
```

The callback receives the operation context and must observe cancellation. It runs once for a logical
operation, not once per retry attempt. Do not log or wrap provider errors with secret-manager paths or
values; the SDK intentionally does not expose the callback's cause.

## Base URL rules

Accepted production shape:

```text
https://api.example.com/device
```

The client preserves the configured path prefix and refuses generated requests outside it. A URL with
credentials, query, fragment, unsupported scheme, or a non-loopback HTTP host is invalid.

For a local test server only:

```go
unifyport.Config{
	BaseURL:           "http://127.0.0.1:8080",
	APIKeyProvider:    unifyport.StaticAPIKey("synthetic-test-key"),
	AllowInsecureHTTP: true,
}
```

Never enable `AllowInsecureHTTP` for a remote host or production deployment.

## Custom HTTP client

An optional `*http.Client` can provide a custom `Transport`, proxy, TLS configuration, or connection
pool. The SDK clones it and takes ownership of the per-call security behavior:

- redirects remain disabled;
- a cookie jar is not used;
- the SDK's operation timeout remains authoritative;
- authentication remains SDK-managed.

Do not use a custom `RoundTripper` that rewrites destinations, captures headers/bodies, injects
credentials, or performs redirects internally. Test doubles must use synthetic data.

## Per-call tightening

Every generated method accepts zero or more `CallOption` values:

```go
result, err := client.GetWorkspace(
	ctx,
	unifyport.WithTimeout(5*time.Second),
	unifyport.WithMaxRetries(1),
)
```

Available options are:

- `WithTimeout(duration)`: uses the smaller of the call and client timeouts;
- `WithMaxRetries(count)`: uses the smaller of the call and client retry counts;
- `WithoutRetry()`: disables retry for that call.

Options cannot make a non-retryable operation retry, extend the global timeout, increase the global
retry count, change the response limit, replace authentication, or change the target URL.
