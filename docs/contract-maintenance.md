# Contract maintenance

[English](contract-maintenance.md) | [简体中文](zh-CN/contract-maintenance.md)

## Sources of truth

| Concern | Authoritative file | Rule |
| --- | --- | --- |
| Device API wire protocol | `contracts/device.openapi.yaml` | Only approved public schema input |
| Approved Go naming workaround | `codegen/go.overlay.yaml` | Exact fail-closed allowlist; no type/wire changes |
| Operation safety | `policy/operation-policies.json` | Explicit, language-neutral classifications |
| Node baseline | `parity/source.json` | Public release/commit and contract digest only |
| Derived coverage | generated parity manifest | Never maintained by hand |

Do not infer a field, status, side effect, authentication rule, or provider behavior from an
unpublished implementation, production observation, or another private repository. If approved public
material is incomplete, keep the existing contract and record the open question outside generated
files.

## Prerequisites

- the latest security patch in the supported Go 1.25 or 1.26 release line;
- the generator version pinned by `internal/generator/go.mod` and the root `go:generate` entrypoint;
- an approved public API schema and release note for the change;
- a clean understanding of all existing worktree changes;
- compatibility and security owners for the affected behavior.

Generation being successful proves only that tools can process the document. It does not prove that
the protocol or operation security classification is correct.

## Standard update workflow

### 1. Review the public change

Confirm:

- stable unique `operationId`, method, path, tag, and authentication;
- path/query/header/body parameters and their required/nullable rules;
- every public success and error status, content type, response header, and empty-body case;
- schema formats, enums, ranges, unions, and additional-property behavior;
- side effects, idempotency, secret lifecycle, destructive behavior, and compatibility;
- additions, deprecations, and breaking changes described by release notes.

### 2. Update the checked-in contract

Modify `contracts/device.openapi.yaml` only from that approved material. Keep provider-account routes
under the Device API. Model `204`, non-JSON responses, redirects, and integer ranges as the protocol
defines them rather than forcing them into a common JSON shape.

### 3. Keep the overlay fail-closed

`codegen/go.overlay.yaml` is not a general extension surface. The validator currently accepts exactly
one action:

| Property | Only allowed value |
| --- | --- |
| `target` | `$.components.responses.RuntimeActionResult` |
| update key | `x-go-name` |
| update value | `RuntimeActionResponse` |

This resolves the response/schema component name collision without changing the contract. The
validator rejects `x-go-type`, another update key/value, another target, more than one action, unknown
YAML fields, or multiple documents before code generation runs.

Any future workaround requires explicit contract/security review and a validator/test change in the
same pull request. It must not change method, path, operation ID, authentication, parameter location,
request/response fields, requiredness, nullability, status, media type, schema constraints, protocol
descriptions, or Go type semantics.

### 4. Classify operation behavior

Update `policy/operation-policies.json` only after reviewing each affected operation. Every new
operation starts with the strict defaults: no retry, no secret assumptions, and no extra exposure.

At minimum classify:

- `mutability`: read, write, or destructive;
- `retryable`: true only with proven idempotency and safe failure behavior;
- `secretInput` and `secretOutput` independently;
- any retained cross-language policy field needed for parity reporting.

MCP classifications may remain in language-neutral parity data, but they do not create an MCP runtime
or public tool surface in the Go repository.

### 5. Generate and inspect drift

```bash
go generate ./...
git status --short
git diff --exit-code
```

Review generated model types, operation names, method/path mappings, request requiredness, response
statuses, policy metadata, and manifest counts. `git diff --exit-code` is expected to report changes
during an intentional update; after committing or comparing the intended generated output, a second
generation must be clean.

Never correct a generated file manually. Fix the contract, policy, overlay, template, or generator.
Generation executes the isolated `internal/generator` module. The root runtime module must continue to
contain no generator-only dependency and no embedded OpenAPI document.

### 6. Add boundary tests and documentation

Depending on the change, cover:

- request path/query/body serialization and successful response parsing;
- public error, empty body, malformed/non-JSON body, and oversized body behavior;
- credential scope and redirect refusal;
- timeout, cancellation, retry classification, `Retry-After`, and backoff limits;
- synchronous build/parse deadline checks and rejection before credential/HTTP after budget expiry;
- generated `dependentRequired` guards before credential or network work;
- exact `int64`/`uint64` boundaries and no-content results;
- missing/repeated cursor and maximum-page pagination failures;
- stable Go method naming and example compilation.

Update the canonical English documents and their complete Simplified Chinese counterparts together.

### 7. Run release gates

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

Inspect `git status --short` after generation to catch untracked outputs. CI runs the latest available
patches in the supported Go 1.25 and 1.26 lines. The nested test suite includes a clean external
consumer that imports and compiles the root package and rejects generator-only modules in its graph. A
real staging/provider end-to-end result remains a separate release gate.

## Compatibility review

Treat at least these changes as potentially breaking:

- removing or renaming an operation ID, public Go method, type, field, or error;
- changing Go initialism normalization or request-struct layout;
- adding a required input, narrowing a type, or changing a success status/media type;
- changing integer signedness/range, no-content representation, or visible result metadata;
- changing authentication, redirect, retry, timeout, response-size, or pagination defaults;
- exposing generated/raw transport types that callers could start depending on;
- raising the minimum Go version.

An optional response field can still affect exhaustive business logic and must be documented.

## Node synchronization

The public Node SDK is the behavioral baseline, while the checked-in OpenAPI contract remains the only
wire input. For a baseline update:

1. verify the public Node release and repository revision;
2. compare the complete contract digest and operation policy, not selected files;
3. copy only approved public contract/policy changes;
4. update `parity/source.json` with public provenance and no local paths or private branches;
5. regenerate and inspect the complete Go diff;
6. update [Node parity](node-parity.md) with intentional ecosystem differences;
7. run all offline gates and record the separate external E2E status.

## Diagnosing generated drift

When a clean-generation check fails:

1. confirm Go and the generator version in `internal/generator/go.mod`;
2. run generation and inspect all tracked and untracked changes;
3. check the contract, policy, overlay, ordering, line endings, timestamps, and accidental absolute
   paths;
4. fix the input or generator and generate again;
5. rerun the full gates.

Do not commit an unexplained generated diff solely to make CI green.
