# Facebook channel

[English](facebook.md) | [简体中文](zh-CN/facebook.md)

This source update adds `provider=facebook` to the existing Device API client. It adds no HTTP routes or standard event names and does not announce a package release or deployment.

Create an account with `provider=facebook`, `auth_mode=session` and an available `region`. Submit a short-lived HTTPS `session_url` through `POST /v1/accounts/{account_id}/auth/session`, then inspect `POST /v1/accounts/{account_id}/runtime/refresh`. Imported/authorized session material does not prove the account is running; wait for `runtime_status=running` before sending. Session material and its URL are credentials: keep them out of logs and model input.

Use `GET /v1/accounts/{account_id}/conversations` with `type=user`. It covers the currently synchronized sendable E2EE direct conversations, not full history. Preserve opaque cursors; an empty filtered page with `has_more=true` is not the end. Use its string conversation IDs unchanged for `POST /v1/messages`.

Send one text message or one image/video/audio/document/file attachment per request. Groups, channels, QR/code authorization, caption, mentions, quoted replies, conversation detail/write actions, contacts, message actions, posts/comments and calls are outside this channel's initial scope. Existing shared request types do not prove every provider implements every option. `accepted` proves channel acknowledgement, not delivery/read status. Do not automatically resend an uncertain result or extend the retry allowlist.

## Standard Webhooks

The source contract declares `facebookMessageReceived`, `facebookAccountAuthSucceeded`, `facebookAccountStatusUpdated` and `facebookAccountAuthFailed`. They reuse `message.received`, `account.auth.succeeded`, `account.status.updated` and `account.auth.failed`; no `facebook.*` event names are introduced. Actual availability and real-account delivery require separate environment acceptance.

- `message.received` may have `direction=inbound` or `outbound`. Preserve string IDs, including values beyond JavaScript's safe integer range. A received `reply_to_message_id` is context, not a sending permission or a `reply_token`.
- Ordinary text/media omit `message.type`; a single sticker without text may use `sticker`. Preserve ordered attachments, absent media URLs, zero sizes/durations and explicit `false`. Do not invent a URL for encrypted or unavailable media.
- Attachment `metadata.duration` retains its native value with an unconfirmed unit; it is not `duration_ms`.
- Public timestamps use UTC RFC3339 with available fractional precision. Missing native message time leaves `sent_at` absent; `occurred_at` can be observation time and is not a message creation time.
- Ordinary connection errors use `account.status.updated` with `runtime_status=error` and omit `auth_status`. They do not revoke authorization. Confirmed authorization failures use `account.auth.failed`, `auth_status=failed` and `runtime_status=disconnected`.
- Reuse the existing raw-body signature verification and top-level event-ID deduplication. Retries retain event identity; the SDK neither generates IDs nor guarantees exactly-once delivery. Unknown callbacks and post comments do not become Facebook business events.

The four receiver declarations provide models, not callable API methods. Automatic retry policies are unchanged.
