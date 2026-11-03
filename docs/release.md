# Release process

[English](release.md) | [简体中文](zh-CN/release.md)


Current source preparation targets `v0.2.0` (unreleased); see [release notes](release-notes.md) and [migration](migration.md). Module versions come from reviewed tags, not a `go.mod` version field. The original `v0.1.0` examples below explain the release procedure; substitute the approved version. The controlled staging/provider E2E gate remains pending for this candidate and must pass before a stable publication; it must not be described as passed during preparation.

## Release model

This repository publishes a Go module through reviewed Git tags. It does not upload a separate binary
or copy source to another registry. Tags follow Semantic Versioning:

- `v0.1.0-alpha.1`: first public pre-release candidate;
- `v0.1.0`: first release after staging/provider verification;
- later versions: compatibility impact determined from the public facade and contract.

Before `v1.0.0`, avoid unnecessary API churn and document every breaking public change explicitly.
Creating or pushing a tag and creating a GitHub Release are external actions that require explicit
maintainer approval.

## Release gates

A source release candidate requires evidence for every gate:

| Gate | Evidence |
| --- | --- |
| Contract | Approved public schema/release notes and reviewed digest |
| Coverage | Generated manifest accounts for every operation and schema |
| Generation | A second `go generate ./...` leaves no tracked or untracked drift |
| Runtime | Unit/integration tests cover authentication, URL, redirect, timeout, retry, limits, errors, integers, and pagination |
| Consumer isolation | A clean external module imports/compiles the SDK and its graph excludes generator-only modules |
| Compatibility | Public API and Node-parity differences reviewed |
| Documentation | English canonical and complete Simplified Chinese docs agree |
| Public boundary | Automated scan and manual diff contain no secrets or private provenance |
| Supply chain | Vet, static analysis, vulnerability scan, action/tool pin review |
| Real API | Controlled staging/provider E2E result recorded separately |

The real-API gate may remain pending for an alpha source candidate, but it blocks `v0.1.0`. A health
response or unit test is not proof that an account is authorized or a provider workflow succeeds.

## Pre-release validation

Run from a clean worktree using the latest supported Go 1.25 patch and again through CI using the
latest supported Go 1.26 patch:

```bash
go mod download
go -C internal/generator mod download
go mod tidy -diff
go -C internal/generator mod tidy -diff
unformatted="$(gofmt -l $(git ls-files '*.go'))"
test -z "${unformatted}"
go generate ./...
git diff --exit-code
git status --short
go test ./...
go -C internal/generator test ./...
go test -race ./...
go -C internal/generator test -race ./...
go vet ./...
go -C internal/generator vet ./...
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
staticcheck ./...
(cd internal/generator && staticcheck ./...)
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
govulncheck ./...
(cd internal/generator && govulncheck ./...)
bash scripts/check-public-test.sh
bash scripts/check-public-docs.sh
bash scripts/check-public.sh
git diff --check
```

`git status --short` must be empty before tagging. Review all tracked files, generated artifacts,
examples, test fixtures, and GitHub workflow changes manually even when the scanner passes.

## Version review

Before approving a version:

1. confirm the intended tag and compatibility level;
2. confirm root `go.mod` has the public module path and supported Go baseline, while generator
   toolchain dependencies are pinned only by `internal/generator/go.mod`;
3. verify the contract digest and parity source use public provenance only;
4. review generated operation/schema counts and security policy counts;
5. update release-facing documentation for contract or behavior changes;
6. confirm no temporary replace directive, local path, debug flag, credential, or test-only export exists;
7. confirm CI passed from the exact release commit.

Do not place a real credential in a release command, workflow secret name beyond the documented SDK
environment variables, release note, or validation output.

## Tag and GitHub Release

After explicit approval, an authorized maintainer creates the reviewed tag and pushes only that tag.
Release notes should state:

- public operation/schema or behavior changes;
- compatibility impact and migration instructions;
- security-boundary changes;
- supported Go versions;
- offline gate status and the separate staging/provider E2E status;
- known limitations without internal implementation or customer details.

Attach no source bundle other than GitHub's standard archive unless a separately reviewed artifact is
needed. Do not attach test logs, `.env` files, coverage profiles, generated debug output, or E2E data.

## Consumer verification

Before tagging, run the automated local isolation gate. It creates a temporary module with
`GOWORK=off`, writes real source that imports the root package, compiles it, and rejects generator-only
dependencies in the consumer graph:

```bash
go -C internal/generator test -run TestCleanConsumerCompilesWithoutCodegenDependencies -v
```

After the tag is public, independently verify the released module from a new temporary directory. The
source file is required: `go test ./...` in an empty module does not compile the SDK and is not release
evidence.

```bash
consumer_dir="$(mktemp -d)"
cd "${consumer_dir}"
GOWORK=off go mod init example.com/unifyport-release-check
GOWORK=off go get github.com/Unify-Port/UnifyPort-SDK-GO@v0.1.0
cat > consumer.go <<'EOF'
package consumer

import (
	"context"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func compilePublicSDK(ctx context.Context, client *unifyport.Client) {
	_, _ = client.GetWorkspace(ctx)
	_ = unifyport.ProviderNameLine
}
EOF
GOWORK=off go mod tidy
GOWORK=off go test ./...
modules="$(GOWORK=off go list -m -f '{{.Path}}' all)"
printf '%s\n' "${modules}"
for forbidden in \
  github.com/getkin/kin-openapi \
  github.com/oapi-codegen/oapi-codegen/v2 \
  github.com/speakeasy-api/openapi \
  go.yaml.in/yaml/v3; do
  if grep -Fxq "${forbidden}" <<<"${modules}"; then
    printf 'unexpected generator dependency: %s\n' "${forbidden}" >&2
    exit 1
  fi
done
```

Use the actual released version in place of `v0.1.0`. The consumer check uses no API key and does not
call a live service; it proves released-module resolution, public-package compilation, and generator
dependency isolation only.

## Post-release verification

- confirm the tag and GitHub Release point to the reviewed commit;
- confirm `go list -m` resolves the intended version through the public module ecosystem;
- compile the documented example in a clean consumer module;
- inspect public archives for unexpected files;
- record the controlled staging/provider E2E result without payloads or identifiers;
- open a follow-up issue for any deferred gate rather than describing it as complete.

If a published release exposes a secret or private data, follow the private security-reporting process
immediately. Deleting a tag alone does not remove content already mirrored by public module proxies.
