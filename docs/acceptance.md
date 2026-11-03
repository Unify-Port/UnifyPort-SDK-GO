# Acceptance report

[English](acceptance.md) | [简体中文](zh-CN/acceptance.md)

Report date: 2026-07-29

This file retains the initial 2026-07-29 acceptance record. See [Contract update](contract-update.md) for the current unreleased 78-operation/225-schema update and compatibility notes; the historical gates below do not establish this update's release or current CI status.

## Outcome

The current source candidate satisfies the local offline architecture, generation, API-coverage,
runtime, documentation, consumer-isolation, and public-information gates described below on the
reported local toolchain. It is not yet evidence of a published release or real-provider success:
GitHub-hosted CI, a clean tagged-commit check, released-module verification, and controlled
staging/provider end-to-end validation remain separate gates.

## Accepted scope

| Requirement | Evidence | Result |
| --- | --- | --- |
| Public Device API coverage | Generated manifest and generator tests | 64 operations, 69 component schemas, 11 resource tags |
| Safety-policy parity | Generated manifest and policy tests | 19 retryable, 7 destructive, 8 secret-input, 8 secret-output operations |
| Public facade | Generated `Client` methods and compile tests | All 64 operation IDs mapped to Go methods |
| Contract-only wire input | OpenAPI snapshot, fail-closed Go overlay tests | Passed; only the approved `RuntimeActionResult` response rename is accepted |
| Request dependency validation | Generated facade and runtime regression tests | OpenAPI `dependentRequired` pairs are rejected before credential or HTTP work |
| Internal generated client | Go `internal/` import boundary | Public consumers cannot import the raw wire client |
| Generator isolation | Nested module and generated-output checks | Toolchain pinned in `internal/generator`; no embedded spec or generator-only dependency in the root runtime graph |
| Clean consumer | `TestCleanConsumerCompilesWithoutCodegenDependencies` | A `GOWORK=off` external module really imports/compiles the SDK and rejects four generator-only modules |
| Result envelope | Integration and result tests | All current public envelope fields preserved; raw response/body excluded |
| Package documentation | Root `doc.go` and `go doc` discovery | Public package purpose and security boundary are discoverable without exposing internals |
| Documentation | Canonical English plus complete Simplified Chinese guides | Architecture, security, maintenance, parity, usage, release, and 64/64 coverage present |
| Examples | `go test ./...` compiles every example package | Getting started, real list-operation pager adapter, and offline pager |

## Local commands run

The following commands completed successfully against the source candidate:

```bash
go generate ./...
go mod tidy -diff
go -C internal/generator mod tidy -diff
test -z "$(find . -type f -name '*.go' -not -path './.git/*' -print0 | xargs -0 gofmt -l)"
go test ./...
go -C internal/generator test ./...
go -C internal/generator test -run TestCleanConsumerCompilesWithoutCodegenDependencies -v
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
go doc github.com/Unify-Port/UnifyPort-SDK-GO
git diff --check
```

Generation was run with SHA-256 snapshots of `internal/wire/device.gen.go`,
`parity/manifest.gen.json`, `models.gen.go`, and `operations.gen.go` before and after; all outputs were
byte-identical. The nested generator check-mode and clean-consumer tests also passed.

Go 1.25.12 and Go 1.26.5 each passed generation, root/nested tidy-diff, tests with cache disabled, race
tests with cache disabled, and vet. The generated artifact SHA-256 values were identical within and
across both toolchains. Staticcheck v0.7.0 and `govulncheck` v1.6.0 then passed against both modules on
Go 1.26.5; both vulnerability scans reported no reachable known vulnerability. This remains local
evidence and does not claim the GitHub-hosted Go 1.25.x/1.26.x matrix.

`git diff --exit-code` is intentionally reserved for CI on the committed candidate because local
project creation differs from the initial repository. The workflow runs it after generation and also
rejects untracked generated files.

## Boundary coverage

Offline tests cover:

- base URL structure, HTTPS/loopback rules, path scope, and rejection before credential lookup;
- redirect refusal, cookie-jar removal, and nil/custom `http.Client` normalization;
- one credential-provider call per logical operation and no raw provider/network cause;
- safe/unsafe retry policy, replayable bodies, temporary statuses, `Retry-After`, jitter, and cancelable
  backoff;
- a shared timeout budget from `executeOperation` across build, credential, HTTP/retry/read, and parse;
  context-aware work is canceled promptly, while synchronous build/parse may return after the deadline
  only as `TimeoutError`, never success;
- filtered API diagnostics, response headers, envelope metadata, and malformed-success redaction;
- operation catalog completeness, optional request bodies, success-status handling, generated
  manifest freshness, rejection of `x-go-type` or extra overlay targets, and generated
  `dependentRequired` preflight before credential/HTTP work;
- root/nested module separation, no embedded OpenAPI spec, real clean-consumer compilation, and absence
  of generator-only dependencies from that consumer graph;
- lazy pagination, empty pages, missing/repeated cursors, page limits, fetcher errors, and context
  cancellation;
- compile-time coverage for every public example.

Tests use synthetic values and local HTTP servers. They do not use a real API key, account identifier,
authorization payload, customer record, or provider message.

## Documentation and public boundary

The documentation and public-boundary checks verify:

- both API coverage tables contain exactly the same 64 operation IDs as the generated manifest;
- every canonical guide has a Simplified Chinese counterpart, local Markdown targets exist, and root
  package documentation is present in `doc.go`;
- high-risk credential/token/private-key patterns, developer absolute paths, private network URLs,
  private repository metadata, and sensitive filenames are absent;
- independent fixtures exercise every scanner rule class: private keys, access tokens, JWTs,
  production-like keys, hard-coded key assignments, developer paths, private URLs, private metadata,
  and sensitive filenames; a safe synthetic placeholder is also accepted.

Automated scanning complements manual review; it is not a proof that arbitrary sensitive data cannot
exist. Release review must inspect the complete committed diff and public archive.

## External gates not claimed

| Gate | Status and required evidence |
| --- | --- |
| GitHub Actions | Workflow selects the latest Go 1.25/1.26 patches; it must pass on the exact committed release candidate |
| Clean committed generation | `go generate ./...`, `git diff --exit-code`, and empty `git status --short` in CI/release checkout |
| Staging/provider E2E | Not performed by offline acceptance; requires a controlled test workspace and redacted outcome |
| Public tag/module resolution | No tag or GitHub Release is claimed; after authorization, compile real importing source against the released version and inspect its module graph |
| Vulnerability freshness | Local `govulncheck` is point-in-time evidence; rerun immediately before release |

`v0.1.0-alpha.1` preparation can proceed after the committed CI gate passes. Stable `v0.1.0` remains
blocked until the controlled staging/provider E2E gate passes and an authorized maintainer approves the
release.
