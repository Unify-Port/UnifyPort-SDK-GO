# Release notes

[English](release-notes.md) | [简体中文](zh-CN/release-notes.md)

## 0.2.0 — prepared, unreleased

Prepared on 2026-11-03; this is not a publication date. This pre-1.0 minor version includes breaking response/type changes from `0.1.0`. Read the [migration guide](migration.md) before upgrading.

### Changes

- Synchronizes 78 HTTP operations, 225 component schemas, 24 standard event names plus `*`, and 40 specialized Webhook receiver declarations. The 57 synthetic examples cover 29 receiver declarations, not all 40.
- Adds account-operation queries, hosted authorization/Passkey, contacts/membership, on-demand history, posts/comments and gated calls. Account pagination, creation/update inputs, asynchronous deletion, wallet/QR metadata and message/Webhook unions are updated.
- Adds the `facebook` provider and four receiver declarations with eight examples; Facebook itself adds no HTTP operations or standard event names. Session-only authorization and E2EE direct-message limits are documented in [Facebook](facebook.md).
- Keeps authentication, credential filtering and the 19-operation retry allowlist. Adds optional validated five-digit `NumericCode` error metadata without exposing server bodies. No new automatic write retries or MCP exposure are enabled.


### Version and provenance

The planned Git tag is `v0.2.0`. Go resolves the module version from that tag; `go.mod` keeps the existing module path without a `/v2` suffix. No version constant or tag is created by this preparation.

The approved public contract comes from [Node source ce60499](https://github.com/Unify-Port/UnifyPort-SDK-Nodejs/commit/ce60499d7169c860ed3f64a686e04bff44316f35). Its package metadata was `0.4.0` with baseline kind `approved-source`; that immutable provenance remains unchanged and does not claim npm `0.5.0` has been published. Contract SHA-256: `8190a34dc8ed60476f79e00b37d9dea418082886724ee6d64efa389f440adffc`.

Supported runtime: Latest security patch in the Go 1.25 or 1.26 line.

### Verification and remaining gates

Run from the source checkout:

```bash
go generate ./...
go test ./...
go -C internal/generator test ./...
go test -race ./...
go -C internal/generator test -race ./...
go vet ./...
go -C internal/generator vet ./...
staticcheck ./...
govulncheck ./...
bash scripts/check-public-test.sh
bash scripts/check-public-docs.sh
bash scripts/check-public.sh
git diff --check
```

Review the exact candidate diff and package/archive contents; install the local artifact into a clean consumer and confirm its version and public imports. These are preparation gates, not a statement that the listed commands or release CI have already passed.

No tag, registry publication, GitHub Release, deployment or real-account E2E is performed by this preparation. CI must pass on the final reviewed release commit. Controlled staging/provider E2E and post-publication registry/module resolution remain separate gates. Re-run time-sensitive vulnerability checks before release. Rollback constraints are documented in the [migration guide](migration.md).

Also run generator-module static/vulnerability checks and the isolated-consumer gate in the [release process](release.md). Its real-API gate remains pending for this candidate.
