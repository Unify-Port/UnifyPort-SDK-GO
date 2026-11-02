# Contract update (unreleased)

[English](contract-update.md) | [简体中文](zh-CN/contract-update.md)

The Go SDK tracks approved public Node source [5901244](https://github.com/Unify-Port/UnifyPort-SDK-Nodejs/commit/5901244c409f63fd6b77a899c239ebc9ee96dd50). This is an approved-source baseline, not an npm release claim. The package version has not changed and nothing is published.

## Coverage and policy

The snapshot contains 78 HTTP operations, 218 component schemas, 24 standard Webhook event names plus `*`, and 36 specialized Webhook declarations. The same 49 public examples cover 25 declarations. HTTP operations and receiver declarations remain separate.

Added operations cover account-operation lookup, browser authorization sessions and Passkey response/confirmation, contact creation and membership lookup, conversation history requests, posts/comments, and gated calls. Existing account, workspace, provider, authorization and endpoint fields are synchronized too.

There are 25 read, 45 write and 8 destructive operations. Only the existing 19 explicitly approved operations retry. Secret-input/output classifications cover 9/16 operations; a read classification alone does not enable retry. Unknown operations remain unavailable. Idempotency does not authorize automatic retries of account creation.

## Migration

- Account creation requires `provider`, `region`, and `auth_mode`; `name` is optional. Use the separate creation/update schemas where an older client used `AccountRequest`.
- Workspace wallet fields are additive; legacy account-quota fields can be absent in wallet mode. Handle their absence explicitly.
- `createAccount` accepts caller-supplied `Idempotency-Key`. Reuse a key only for the same logical creation and parameters. Query an uncertain outcome before submitting another creation.
- `deleteAccount` can return `202` with an account operation or legacy `204` without data. `PENDING`/`PROCESSING` is not completed deletion or refunded capacity. Query `getAccountOperation` with the returned operation ID.
- An actual `auth_mode` change must be submitted alone and can clear the existing Telegram/WhatsApp session. Omitted PATCH fields preserve their values; explicit null is invalid; empty arrays/objects can clear collections.
- Telegram read requests use one ASCII decimal `up_to_message_id` and omit `up_to_message_sender_id`. WhatsApp message reads require the provider-specific message/sender pair; there is no unconditional cross-provider pairing guard.
- QR start/check return typed authorization metadata; check status and supplied payload rather than assuming every response has a QR code. Existing provider `session_url` imports remain supported; `whatsapp-protocol` credentials use its named nested object.
- Optional API error `NumericCode` is a five-digit string and never replaces the stable string code. Malformed numeric metadata is ignored independently. Error bodies and upstream details remain excluded from public exceptions.

Go consumers must recompile: `DeleteAccount` now returns `Result[*AccountOperation]` (`nil` for 204), wallet-compatible quota fields are pointers, and QR results use named models instead of a free-form map. Other no-content methods retain `NoContent`. The compatibility adapter preserves the original contract bytes and naming overlay, expands boolean schemas equivalently, and appends redundant intersection structure while retaining the original constraints. Mutation tests compare validation before/after adaptation.

## Webhook consumers

Merge only supplied fields and preserve explicit false, zero, empty strings and empty selections. For call records, keep terminal `ended` across out-of-order delivery and retain offered media when later updates omit it. `message.*` call records and `call.*` browser events are distinct. A successful delivery does not prove consumer persistence or an external business outcome.

Generated types do not validate untrusted HTTP bodies. Numeric ranges, formats and unsupported cross-field conditions still need validation at the receiver boundary; Go cannot encode every forbidden field as a compile-time never type. No receiver, signature algorithm, production delivery or real-provider E2E is added by this change.
