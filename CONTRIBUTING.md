# Contributing

[English](CONTRIBUTING.md) | [简体中文](CONTRIBUTING.zh-CN.md)

Thank you for helping improve the UnifyPort Go SDK. This is a public, contract-first project; every
change must preserve the protocol, security, and information-disclosure boundaries below.

## Development setup

Use the latest security patch in the Go 1.25 or 1.26 release line. From the repository root:

```bash
go mod download
go generate ./...
go test ./...
```

Install the optional release-gate tools at the versions used by CI:

```bash
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
```

## Contract-first changes

Do not edit generated files. Start with an approved public Device API schema or release note, update
`contracts/device.openapi.yaml`, update explicit operation policy when required, and regenerate. The
Go-only overlay currently contains one fail-closed, approved response-component rename; it is not a
general naming or type override mechanism and cannot alter wire semantics.

For every operation, review authentication, side effects, retry safety, destructive behavior, secret
input/output, response limits, and integer precision. Unknown behavior uses conservative defaults: no
automatic retry and no new escape hatch.

## Code and documentation

- Keep public APIs idiomatic, small, and documented with GoDoc.
- Keep generated transport code internal and route calls through the secure runtime.
- Add brief Simplified Chinese comments only where they explain intent, boundaries, compatibility,
  external constraints, or risk.
- Use synthetic values in tests and examples. Never use real API keys, authorization payloads,
  identifiers, messages, hosts, or response bodies.
- English documentation is canonical. Update the complete Simplified Chinese counterpart in the same
  pull request.

## Pull-request checklist

- [ ] The change is within the approved public Device API scope.
- [ ] Generated files came from the contract, policy, overlay, or generator rather than manual edits.
- [ ] Compatibility and security impact are described.
- [ ] Offline tests cover new behavior and failure boundaries.
- [ ] English and Simplified Chinese documentation remain aligned.
- [ ] `go generate ./...` leaves no unexplained tracked or untracked drift.
- [ ] Tests, race tests, vet, static analysis, vulnerability scanning, public-content scanning, and
      `git diff --check` pass, or any unavailable external gate is stated explicitly.

The complete commands are documented in [`AGENTS.md`](AGENTS.md). A pull request must not publish,
rotate, or request real credentials as part of validation.
