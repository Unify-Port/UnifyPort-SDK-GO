# AGENTS.md

[English](AGENTS.md) | [简体中文](AGENTS.zh-CN.md)

This file defines the repository-wide rules for maintainers and automated agents. A nested
`AGENTS.md` may add stricter rules, but it must not relax the contract, security, or public-release
boundaries below.

## Language and comments

- Use Simplified Chinese for collaboration by default. Keep identifiers, commands, configuration
  keys, protocol fields, and error codes in English.
- Public user documentation uses English as the canonical version. Update the complete Simplified
  Chinese counterpart in the same change.
- Add or update a short Simplified Chinese comment when a code change needs explanation. Comments
  explain only why: intent, boundaries, compatibility, external constraints, or risk.
- Delete stale comments when behavior changes. Test names, code blocks, and commit messages use
  English unless an existing file defines a narrower convention.

## Public repository boundary

- This repository implements only capabilities in the approved public Device API contract.
  Provider-account routes under `/v1/accounts/...` remain part of that Device API.
- Never add credentials, customer or account data, production payloads, private hosts, local absolute
  paths, unpublished repository metadata, internal implementation details, or unapproved protocol
  observations to source, tests, docs, generated output, logs, commits, or releases.
- The only wire-protocol input to generation is `contracts/device.openapi.yaml`. The Go overlay is
  fail-closed and currently permits exactly one approved response-component rename; it must not change
  methods, paths, fields, requiredness, responses, authentication, or Go type semantics.
- Do not edit a file carrying a `Code generated` marker. Update the public contract, language-neutral
  operation policy, overlay, or generator and regenerate.
- This repository does not contain an MCP server, a generic raw HTTP client, or a webhook signature
  algorithm that is absent from the public contract.
- Every public delivery must run `bash scripts/check-public.sh`. Other tests do not replace this gate.

## Before changing files

1. Read this file, `README.md`, and the relevant document under `docs/`.
2. Use `rg` to confirm real exports, operation IDs, generated markers, and existing tests. Do not guess
   API names from memory.
3. Inspect `git status` and preserve work belonging to the user or other agents.
4. If interpretations would change the public API, compatibility, or a security boundary, explain
   the alternatives and impact before choosing one.
5. For contract work, confirm the approved public schema and release notes before modifying the
   checked-in snapshot.

## Architectural invariants

- The root `unifyport` package is the supported facade. Generated HTTP code stays under `internal/`
  and cannot be imported by consumers.
- `X-Api-Key` comes only from client configuration. Operation requests and call options cannot
  replace it, change the target base URL, or send it outside the configured origin and path prefix.
- HTTPS is required by default. An insecure HTTP exception must be explicit and limited to loopback
  development. Redirects are not followed automatically.
- Resolve the API-key provider once per logical operation and reuse that value across its attempts.
- Retry only operations explicitly classified as safe. Unknown operations, writes, destructive
  actions, authorization submissions, and one-time-secret operations default to no retry.
- Per-call options may cancel, disable, or tighten global timeout and retry limits; they cannot expand
  the client's configured budget.
- Enforce response limits while reading. Do not rely only on `Content-Length`.
- Public errors expose only stable, filtered metadata. Never retain or print raw bodies, headers,
  upstream messages/details, credentials, provider errors, or raw causes.
- Preserve integer precision, explicit no-content results, context cancellation, and bounded cursor
  pagination. Do not hide these behaviors behind a raw transport escape hatch.

## Contract and generation workflow

1. Review the approved public schema and release notes.
2. Update `contracts/device.openapi.yaml`. Change the Go overlay only for an explicitly approved
   workaround and extend its fail-closed validator/tests in the same change.
3. Review authentication, mutability, retry safety, secret input/output, and destructive behavior for
   every changed operation. Unknown classifications use the strictest defaults.
4. Update the language-neutral policy and run `go generate ./...`.
5. Inspect the complete generated diff; never patch generated output directly.
6. Add offline tests for transport, errors, response limits, integer boundaries, pagination, and any
   changed operation behavior.
7. Update English and Simplified Chinese documentation together.
8. Run the applicable quality and public-release gates.

## Verification

The full local acceptance sequence is:

```bash
go generate ./...
git diff --exit-code
go test ./...
go test -race ./...
go vet ./...
staticcheck ./...
govulncheck ./...
bash scripts/check-public.sh
git diff --check
```

Also inspect `git status --short` after generation because `git diff --exit-code` does not report new
untracked generated files. Documentation-only changes require at least the public check, link review,
and `git diff --check`. If a command cannot run, state the exact reason and remaining risk; never report
an unexecuted check as passing.

## Delivery notes

Lead with the outcome, then list the important files, compatibility and security impact, commands
actually run, and any gate that remains external. Do not push, tag, publish, or create a release unless
the user explicitly authorizes that external action.
