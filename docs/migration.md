# Migration to 0.2.0

[English](migration.md) | [简体中文](zh-CN/migration.md)

This guide covers `github.com/Unify-Port/UnifyPort-SDK-GO` `0.1.0` → `0.2.0`. Version `0.2.0` is prepared in source and remains unreleased. Review these changes before upgrading: pre-1.0 minor versions can contain breaking changes.

## Account and response changes

| Previous assumption | Required migration |
| --- | --- |
| The account-list response `data` is an array | The wire body now contains `data.items`, `data.has_more` and `data.next_cursor`. Read `result.Data.Items` through the SDK result; forward opaque cursors unchanged until `has_more` is false. |
| One `AccountRequest` schema serves creation and update | Replace it with `AccountCreateRequest` or `AccountUpdateRequest`. Creation requires `provider`, `region` and `auth_mode`; `name` is optional. |
| PATCH null and omission are interchangeable | Omitted fields preserve values; explicit null is invalid. Empty arrays/objects can clear collections. An actual `auth_mode` change must be submitted alone and can clear a Telegram/WhatsApp session. |
| Deletion always returns 204 and is complete | Branch on HTTP status: 202 contains an account operation; 204 has `nil` data. Follow `getAccountOperation` with `operation_id`. `PENDING`/`PROCESSING` does not prove deletion, refund or released capacity. |
| Legacy workspace quota fields are always present | Handle absent quota fields in wallet mode; do not replace missing information with a fabricated zero balance. |
| Every QR response contains an image | Inspect authorization status and optional payload. QR start/check expose structured metadata. |

Pass a caller-owned `Idempotency-Key` only for the same logical account creation and parameters. An unknown write outcome must be reconciled before another submission. Idempotency does not enable SDK write retries. Existing `session_url` imports remain supported; `whatsapp-protocol` credentials use their named nested object.

Telegram message reads use one ASCII decimal `up_to_message_id` without `up_to_message_sender_id`. WhatsApp message-level reads follow the provider-specific message/sender pairing rule. Do not enforce one unconditional rule across providers. Optional API error `NumericCode` is a validated five-digit string; it supplements the stable string error code and never exposes raw error bodies.

## Go consumer changes

`DeleteAccount` changes from `Result[NoContent]` to `Result[*AccountOperation]`; check `StatusCode` before dereferencing `Data`. Wallet-compatible quota fields are pointers; QR results use named models instead of a free-form map. Recompile consumers of public model aliases and typed unions. Other no-content operations retain `NoContent`. Follow deletion with `GetAccountOperation`; pass an idempotency key through `CreateAccountParams.IdempotencyKey`.

## Facebook and Webhook consumers

- Corrected message/Webhook unions include sticker/poll/link_preview, member/all mentions, shared alternative-payload fields, decoded/unavailable poll votes, disabled settings, unpinned snapshots and sparse call records. Re-check existing constructors and event branches rather than widening them to an untyped object.

- Add `facebook` to exhaustive provider branches. Use session authorization, refresh runtime state, and wait for `runtime_status=running` before sending. Imported/authorized credentials do not prove the account is running.
- The initial Facebook scope is synchronized sendable E2EE direct conversations and text or one media attachment per send. Groups, captions, mentions, quoted sends, contacts/actions, posts/comments and calls are outside that scope. Shared request types do not grant provider capabilities. See [Facebook](facebook.md).
- Handle `facebookMessageReceived`, `facebookAccountAuthSucceeded`, `facebookAccountStatusUpdated` and `facebookAccountAuthFailed` as receiver declarations. They reuse existing standard event names; there are no `facebook.*` event names or callable receiver methods.
- `message.received` can be inbound or outbound. Preserve string IDs, attachment order, missing URLs, explicit false/zero/empty values and all available timestamp digits. `metadata.duration` has no confirmed unit and is not `duration_ms`. An incoming reply reference does not authorize a quoted send.
- Merge supplied fields only. Distinguish connection errors from revoked authorization. Preserve terminal call state across out-of-order updates; `message.*` call records and `call.*` browser events are separate streams. Validate raw bodies, reuse the existing signature procedure and deduplicate by top-level event ID; types do not guarantee exactly-once delivery.

## Upgrade and rollback checklist

1. Pin the intended version only after it is publicly available; use a local package/archive or module replacement for preparation checks.
2. Re-run application type/static checks and exercise paginated accounts, both deletion statuses, optional wallet/QR fields, provider switches and Webhook fixtures.
3. Keep `X-Api-Key` injection, credential redaction and the 19-operation retry allowlist. No new operation becomes automatically retryable. `accepted`, HTTP 2xx and offline tests do not prove delivery or read state.
4. Record controlled staging/provider E2E separately. No live-account result or deployment is asserted by this preparation.
5. If rolling back, pin the previous SDK and restore compatible application code. A package downgrade does not roll back server response shapes, account state or side effects; retain pagination/deletion compatibility or stop the affected workflow first. Do not replay uncertain writes.

See [release notes](release-notes.md) for coverage, version sources and remaining release gates.
