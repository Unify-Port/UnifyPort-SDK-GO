# Security policy

[English](SECURITY.md) | [简体中文](SECURITY.zh-CN.md)

## Reporting a vulnerability

Report vulnerabilities privately through the repository's
[GitHub Security Advisory form](https://github.com/Unify-Port/UnifyPort-SDK-GO/security/advisories/new).
Do not open a public issue containing exploit details, credentials, account or customer data,
request/response bodies, logs, or private infrastructure information.

Include the affected SDK version, a minimal synthetic reproduction, expected behavior, and security
impact. Replace all credentials, identifiers, URLs, messages, and personal data with unmistakable
placeholders. Never attach `.env` files, packet captures, production logs, or unredacted traces.

## Supported versions

Security fixes target the latest stable release. Pre-release versions are supported on a best-effort
basis until `v0.1.0` is published. Older versions may receive a fix only when maintainers explicitly
announce an extended support window.

## Security boundary

The SDK is designed to enforce:

- API-key isolation to the configured origin and path prefix;
- no automatic redirect following;
- explicit safe-operation retry policy with finite budgets;
- finite timeout, response-size, and pagination limits;
- precision-safe integer handling;
- filtered public error metadata without raw bodies, headers, upstream details, or causes;
- no generic raw request API that bypasses operation policy.

Applications remain responsible for secure secret storage, using a suitably configured `http.Client`,
avoiding sensitive logging, applying least privilege to API keys, and upgrading supported SDK versions.
Successful local tests do not prove authorization against a real provider; staging/provider end-to-end
verification is a separate release gate.

See [`docs/security.md`](docs/security.md) for the detailed threat model and implementation rules.
