# Examples

[English](README.md) | [简体中文](README.zh-CN.md)

Every example is a standalone Go package compiled by `go test ./...`.

| Example | Purpose | External access |
| --- | --- | --- |
| [`getting-started`](getting-started/main.go) | Construct a secure client and call `GetWorkspace` | Calls the configured Device API |
| [`list-contacts-pager`](list-contacts-pager/main.go) | Adapt `ListContacts` to `Pager[Contact]` without logging records/cursors | Calls the configured Device API |
| [`pagination`](pagination/main.go) | Demonstrate bounded cursor iteration with synthetic pages | None |

Run the offline pagination example:

```bash
go run ./examples/pagination
```

The live examples read `UNIFYPORT_DEVICE_API_BASE_URL` and `UNIFYPORT_DEVICE_API_KEY` from the process
environment; the contacts example also reads `UNIFYPORT_ACCOUNT_ID`. Never store a real value in
source, shell history, `.env`, test output, or issue reports. The examples log only filtered SDK
metadata or an aggregate count, not business records or cursors.

See [Getting started](../docs/getting-started.md) for setup and [Error handling](../docs/errors.md) for
production branching.
