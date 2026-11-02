# UnifyPort Go SDK

[English](README.md) | [简体中文](README.zh-CN.md) | [Website](https://www.unifyport.ai) |
[Security](SECURITY.md) | [MIT License](LICENSE)

[![CI](https://github.com/Unify-Port/UnifyPort-SDK-GO/actions/workflows/ci.yml/badge.svg)](https://github.com/Unify-Port/UnifyPort-SDK-GO/actions/workflows/ci.yml)

The contract-first Go SDK for the public UnifyPort Device API. It provides an idiomatic `Client`,
generated operation/model coverage, a security-focused HTTP runtime, typed errors, safe response
metadata, and bounded cursor pagination.

The current public baseline covers 78 operations, 218 component schemas, and 13 resource groups. Routes
under `/v1/accounts/...` manage provider-account resources but remain part of the same Device API
client.

## Capabilities and boundaries

| Asset | Purpose | Boundary |
| --- | --- | --- |
| `package unifyport` | Public client, models, results, errors, options, and pager | The supported consumer API |
| `contracts/device.openapi.yaml` | Approved public Device API wire contract | The only protocol-generation input |
| `internal/wire` | Generated OpenAPI HTTP client and models | Cannot be imported by consumers |
| secure runtime | Authentication, URL scope, redirect, timeout, retry, limits, and parsing | Every public operation passes through it |
| parity manifest | Operation/schema and Node behavior mapping | Generated; never a second protocol definition |

This repository intentionally does not provide a generic raw request method, arbitrary authentication
headers, MCP server, provider-specific authorization orchestration, or a webhook signature algorithm
that is absent from the public contract.

## Requirements

- the latest security patch in the supported Go 1.25 or 1.26 release line;
- a Device API base URL;
- a Device API key with the minimum required workspace permissions.

## Installation

```bash
go get github.com/Unify-Port/UnifyPort-SDK-GO@latest
```

Applications should replace `@latest` with a reviewed tag and commit both `go.mod` and `go.sum`. The
default branch is development state; only a published tag is a release.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	result, err := client.GetWorkspace(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Print filtered diagnostics only; business data may be sensitive.
	fmt.Printf("status=%d request_id=%s\n", result.StatusCode, result.RequestID)
}

func requiredEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		log.Fatalf("missing required environment variable: %s", name)
	}
	return value
}
```

See [Getting started](docs/getting-started.md) and the compile-checked
[`examples/getting-started`](examples/getting-started/main.go) program for the full setup. Credentials
come only from `Config.APIKeyProvider`; operation input cannot override `X-Api-Key`.

## Safe defaults

| Behavior | Default |
| --- | --- |
| HTTPS | Required; explicit loopback HTTP only for local tests |
| Redirects | Never followed automatically |
| Logical-operation timeout | 30 seconds |
| Response limit | 8 MiB, with a 64 MiB hard configuration limit |
| Additional retry attempts | 2, only for 19 explicitly safe operations |
| Retry statuses | `408`, `429`, `502`, `503`, `504`, plus transport failures |
| Pagination maximum | 10,000 pages by default, with repeated/missing cursor detection |

The API-key provider runs once per logical operation and its result is reused across attempts. Per-call
options can cancel, disable, or tighten timeout/retry budgets; they cannot expand client limits or make
an unsafe operation retry.

The timeout budget starts when `executeOperation` begins execution and covers synchronous request
construction/marshaling, credential lookup, HTTP attempts, retry backoff, bounded response reading,
and response parsing. Synchronous build and parse work cannot be safely preempted; after either returns,
an expired deadline produces `TimeoutError` and any apparent success is discarded.

## Results and errors

Every operation returns `Result[T]`. `Data`, `RequestID`, and `ClientRequestID` preserve all fields in
the current public success envelope; `StatusCode` and the allowlisted `HeaderSnapshot` add safe
transport metadata. No raw response/body or unrestricted header map is exposed. `204` operations use
the explicit `NoContent` type.

Typed configuration, network, timeout, cancellation, API, response-parse, and pagination errors support
idiomatic `errors.As`/`errors.Is`. Public API errors contain only filtered status, code, request ID, and
operation ID; raw upstream messages, details, bodies, headers, provider errors, and causes are omitted.

See [Error handling](docs/errors.md) before adding application logging. A successful `Data` value can
still contain business data or a one-time secret and must not be logged wholesale.

## API and model discovery

- [API coverage](docs/api-coverage.md) maps all 78 operation IDs to Go methods, paths, retry, and data
  classifications.
- `parity/manifest.gen.json` records generated request/response types, statuses, schema references, and
  all 218 component-to-Go mappings.
- `contracts/device.openapi.yaml` is authoritative for protocol fields and descriptions.
- `go doc github.com/Unify-Port/UnifyPort-SDK-GO` shows the public Go declarations.

Do not import `internal/wire` or construct paths manually.

## Validation

```bash
go generate ./...
git diff --exit-code
go mod tidy -diff
go -C internal/generator mod tidy -diff
unformatted="$(gofmt -l $(git ls-files '*.go'))"
test -z "${unformatted}"
go test ./...
go -C internal/generator test ./...
go test -race ./...
go -C internal/generator test -race ./...
go vet ./...
go -C internal/generator vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go -C internal/generator run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
go -C internal/generator run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
bash scripts/check-public-test.sh
bash scripts/check-public-docs.sh
bash scripts/check-public.sh
git diff --check
```

Generation must also leave `git status --short` empty so new untracked artifacts cannot escape the
diff gate. GitHub Actions runs the latest available patches in the Go 1.25 and 1.26 lines.

## Repository structure

```text
.
├── contracts/          # Approved public OpenAPI contract
├── codegen/            # Pinned Go generation config and Go-only overlay
├── internal/
│   ├── generator/      # Deterministic generator in an isolated nested Go module
│   ├── transport/      # Security-focused HTTP runtime
│   └── wire/           # Generated OpenAPI client; never public
├── policy/             # Language-neutral operation classifications
├── parity/             # Public Node baseline and generated manifest
├── docs/               # Canonical English and complete Simplified Chinese guides
├── examples/           # Compile-checked safe examples
└── *.go                # Public unifyport package and generated facade/models
```

`internal/generator/go.mod` pins the generation toolchain separately from the public runtime module.
The root module contains only generated runtime code and its runtime dependencies: it does not embed
the OpenAPI document or add generator libraries to a consumer's module graph.

## Documentation

- [Getting started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [API coverage and model discovery](docs/api-coverage.md)
- [Retry and timeout](docs/retry.md)
- [Error handling](docs/errors.md)
- [Cursor pagination](docs/pagination.md)
- [Architecture](docs/architecture.md)
- [Security boundaries](docs/security.md)
- [Contract maintenance](docs/contract-maintenance.md)
- [Node/Go parity](docs/node-parity.md)
- [Release process](docs/release.md)
- [Acceptance report](docs/acceptance.md)
- [Contributing](CONTRIBUTING.md)

Real staging/provider end-to-end verification is a separate release gate. Offline tests, generated
coverage, or a health response do not prove account authorization or provider success.

The synchronization baseline is approved Node source `5901244`, not a claim that npm has released these capabilities. No package is published by this update. See [Contract update](docs/contract-update.md).
