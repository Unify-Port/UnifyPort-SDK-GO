# API coverage

[English](api-coverage.md) | [简体中文](zh-CN/api-coverage.md)

This table maps every public Device API `operationId` in `parity/manifest.gen.json` to its Go method
and reviewed operation policy. The checked-in contract is authoritative for parameters, request and
response fields, statuses, and schema descriptions. The manifest records generated Go request/response
types and all 225 component-schema mappings.

`bash scripts/check-public-docs.sh` fails unless both language tables contain exactly the same 78
operation IDs as the generated manifest.

| operationId | Go method | Resource | HTTP path | Retry | Classification |
| --- | --- | --- | --- | --- | --- |
| `blockContact` | `BlockContact` | Contacts | `POST /v1/accounts/{account_id}/contacts/block` | No | write |
| `cancelAccountAuth` | `CancelAccountAuth` | Account Auth | `POST /v1/accounts/{account_id}/auth/cancel` | No | write |
| `checkAccountQrAuth` | `CheckAccountQRAuth` | Account Auth | `POST /v1/accounts/{account_id}/auth/qr/check` | No | write; secret output |
| `confirmAccountPasskey` | `ConfirmAccountPasskey` | Account Auth | `POST /v1/accounts/{account_id}/auth/passkey-confirm` | No | write; secret output |
| `createAccount` | `CreateAccount` | Accounts | `POST /v1/accounts` | No | write; secret input; secret output |
| `createAccountAuthSession` | `CreateAccountAuthSession` | Account Auth | `POST /v1/accounts/{account_id}/auth-sessions` | No | write; secret output |
| `createApiKey` | `CreateAPIKey` | API Keys | `POST /v1/api-keys` | No | write; secret output |
| `createCall` | `CreateCall` | Calls | `POST /v1/calls` | No | write; secret output |
| `createContact` | `CreateContact` | Contacts | `POST /v1/accounts/{account_id}/contacts/add` | No | write |
| `createGroup` | `CreateGroup` | Groups | `POST /v1/accounts/{account_id}/groups/create` | No | write |
| `createWebhookEndpoint` | `CreateWebhookEndpoint` | Webhook Endpoints | `POST /v1/webhook-endpoints` | No | write; secret input |
| `deactivateWebhookEndpoint` | `DeactivateWebhookEndpoint` | Webhook Endpoints | `POST /v1/webhook-endpoints/{endpoint_id}/deactivate` | No | write |
| `deleteAccount` | `DeleteAccount` | Accounts | `DELETE /v1/accounts/{account_id}` | No | destructive |
| `deleteConversationLabel` | `DeleteConversationLabel` | Conversations | `POST /v1/accounts/{account_id}/conversations/labels/delete` | No | destructive |
| `deleteWebhookEndpoint` | `DeleteWebhookEndpoint` | Webhook Endpoints | `DELETE /v1/webhook-endpoints/{endpoint_id}` | No | destructive |
| `editMessage` | `EditMessage` | Messages | `POST /v1/messages/edit` | No | write |
| `getAccount` | `GetAccount` | Accounts | `GET /v1/accounts/{account_id}` | Yes | read; secret output |
| `getAccountAuthState` | `GetAccountAuthState` | Account Auth | `GET /v1/accounts/{account_id}/auth` | Yes | read; secret output |
| `getAccountOperation` | `GetAccountOperation` | Accounts | `GET /v1/account-operations/{operation_id}` | No | read |
| `getActiveCall` | `GetActiveCall` | Calls | `GET /v1/calls/active` | No | read |
| `getContact` | `GetContact` | Contacts | `GET /v1/accounts/{account_id}/contacts/info` | Yes | read |
| `getContactMembership` | `GetContactMembership` | Contacts | `GET /v1/accounts/{account_id}/contacts/membership` | No | read |
| `getConversation` | `GetConversation` | Conversations | `GET /v1/accounts/{account_id}/conversations/info` | Yes | read |
| `getGroup` | `GetGroup` | Groups | `GET /v1/accounts/{account_id}/groups/info` | Yes | read |
| `getGroupInviteCode` | `GetGroupInviteCode` | Groups | `GET /v1/accounts/{account_id}/groups/invite-code` | Yes | read; secret output |
| `getPost` | `GetPost` | Posts | `POST /v1/accounts/{account_id}/posts/info` | No | read |
| `getWebhookEndpoint` | `GetWebhookEndpoint` | Webhook Endpoints | `GET /v1/webhook-endpoints/{endpoint_id}` | Yes | read |
| `getWorkspace` | `GetWorkspace` | Workspace | `GET /v1/workspace` | Yes | read |
| `hangupCall` | `HangupCall` | Calls | `POST /v1/calls/{call_id}/hangup` | No | destructive |
| `importAccountAuthSession` | `ImportAccountAuthSession` | Account Auth | `POST /v1/accounts/{account_id}/auth/session` | No | write; secret input |
| `leaveGroup` | `LeaveGroup` | Groups | `POST /v1/accounts/{account_id}/groups/leave` | No | destructive |
| `listAccounts` | `ListAccounts` | Accounts | `GET /v1/accounts` | Yes | read; secret output |
| `listApiKeys` | `ListAPIKeys` | API Keys | `GET /v1/api-keys` | Yes | read |
| `listContactBlocklist` | `ListContactBlocklist` | Contacts | `GET /v1/accounts/{account_id}/contacts/blocklist` | Yes | read |
| `listContacts` | `ListContacts` | Contacts | `GET /v1/accounts/{account_id}/contacts` | Yes | read |
| `listConversationLabels` | `ListConversationLabels` | Conversations | `GET /v1/accounts/{account_id}/conversations/labels` | Yes | read |
| `listConversationMembers` | `ListConversationMembers` | Conversations | `GET /v1/accounts/{account_id}/conversations/members` | Yes | read |
| `listConversations` | `ListConversations` | Conversations | `GET /v1/accounts/{account_id}/conversations` | Yes | read |
| `listGroupJoinRequests` | `ListGroupJoinRequests` | Groups | `GET /v1/accounts/{account_id}/groups/join-requests` | Yes | read |
| `listGroups` | `ListGroups` | Groups | `GET /v1/accounts/{account_id}/groups` | Yes | read |
| `listPostComments` | `ListPostComments` | Posts | `POST /v1/accounts/{account_id}/posts/comments/list` | No | read |
| `listPosts` | `ListPosts` | Posts | `GET /v1/accounts/{account_id}/posts` | No | read |
| `listProviderRegions` | `ListProviderRegions` | Providers | `GET /v1/providers/{provider}/regions` | Yes | read |
| `listWebhookEndpoints` | `ListWebhookEndpoints` | Webhook Endpoints | `GET /v1/webhook-endpoints` | Yes | read |
| `markConversationRead` | `MarkConversationRead` | Conversations | `POST /v1/accounts/{account_id}/conversations/read` | No | write |
| `markConversationUnread` | `MarkConversationUnread` | Conversations | `POST /v1/accounts/{account_id}/conversations/unread` | No | write |
| `muteConversation` | `MuteConversation` | Conversations | `POST /v1/accounts/{account_id}/conversations/mute` | No | write |
| `pinConversation` | `PinConversation` | Conversations | `POST /v1/accounts/{account_id}/conversations/pin` | No | write |
| `pinMessage` | `PinMessage` | Messages | `POST /v1/messages/pin` | No | write |
| `reactMessage` | `ReactMessage` | Messages | `POST /v1/messages/reaction` | No | write |
| `reconnectAccountRuntime` | `ReconnectAccountRuntime` | Account Runtime | `POST /v1/accounts/{account_id}/runtime/reconnect` | No | write |
| `refreshAccountRuntime` | `RefreshAccountRuntime` | Account Runtime | `POST /v1/accounts/{account_id}/runtime/refresh` | No | write |
| `replyPostComment` | `ReplyPostComment` | Posts | `POST /v1/accounts/{account_id}/posts/comments/reply` | No | write |
| `requestConversationHistory` | `RequestConversationHistory` | Conversations | `POST /v1/accounts/{account_id}/conversations/history/request` | No | write |
| `revokeMessage` | `RevokeMessage` | Messages | `POST /v1/messages/revoke` | No | destructive |
| `rotateApiKey` | `RotateAPIKey` | API Keys | `POST /v1/api-keys/{key_id}/rotate` | No | write; secret output |
| `sendMessage` | `SendMessage` | Messages | `POST /v1/messages` | No | write; secret input; secret output |
| `setContactNote` | `SetContactNote` | Contacts | `POST /v1/accounts/{account_id}/contacts/note` | No | write |
| `setConversationLabelMembers` | `SetConversationLabelMembers` | Conversations | `POST /v1/accounts/{account_id}/conversations/labels` | No | write |
| `setGroupJoinApprovalMode` | `SetGroupJoinApprovalMode` | Groups | `POST /v1/accounts/{account_id}/groups/join-approval-mode` | No | write |
| `startAccountAuth` | `StartAccountAuth` | Account Auth | `POST /v1/accounts/{account_id}/auth/start` | No | write; secret output |
| `startAccountQrAuth` | `StartAccountQRAuth` | Account Auth | `POST /v1/accounts/{account_id}/auth/qr/start` | No | write; secret output |
| `startAccountRuntime` | `StartAccountRuntime` | Account Runtime | `POST /v1/accounts/{account_id}/runtime/start` | No | write |
| `stopAccountRuntime` | `StopAccountRuntime` | Account Runtime | `POST /v1/accounts/{account_id}/runtime/stop` | No | write |
| `submitAccountAuthCode` | `SubmitAccountAuthCode` | Account Auth | `POST /v1/accounts/{account_id}/auth/code` | No | write; secret input |
| `submitAccountAuthPassword` | `SubmitAccountAuthPassword` | Account Auth | `POST /v1/accounts/{account_id}/auth/password` | No | write; secret input |
| `submitAccountPasskeyResponse` | `SubmitAccountPasskeyResponse` | Account Auth | `POST /v1/accounts/{account_id}/auth/passkey-response` | No | write; secret input; secret output |
| `unblockContact` | `UnblockContact` | Contacts | `POST /v1/accounts/{account_id}/contacts/unblock` | No | write |
| `unmuteConversation` | `UnmuteConversation` | Conversations | `POST /v1/accounts/{account_id}/conversations/unmute` | No | write |
| `unpinConversation` | `UnpinConversation` | Conversations | `POST /v1/accounts/{account_id}/conversations/unpin` | No | write |
| `updateAccount` | `UpdateAccount` | Accounts | `PATCH /v1/accounts/{account_id}` | No | write; secret input; secret output |
| `updateApiKeyStatus` | `UpdateAPIKeyStatus` | API Keys | `PATCH /v1/api-keys/{key_id}` | No | write |
| `updateGroupInfo` | `UpdateGroupInfo` | Groups | `POST /v1/accounts/{account_id}/groups/update-info` | No | write |
| `updateGroupJoinRequests` | `UpdateGroupJoinRequests` | Groups | `POST /v1/accounts/{account_id}/groups/join-requests/update` | No | destructive |
| `updateGroupMembers` | `UpdateGroupMembers` | Groups | `POST /v1/accounts/{account_id}/groups/members` | No | destructive |
| `updateWebhookEndpoint` | `UpdateWebhookEndpoint` | Webhook Endpoints | `PATCH /v1/webhook-endpoints/{endpoint_id}` | No | write; secret input |
| `updateWorkspace` | `UpdateWorkspace` | Workspace | `PATCH /v1/workspace` | No | write |
| `upsertConversationLabel` | `UpsertConversationLabel` | Conversations | `POST /v1/accounts/{account_id}/conversations/labels/upsert` | No | write |

`MarkConversationReadRequestBody.UpToMessageID` and `UpToMessageSenderID` implement the contract's
`dependentRequired` pair: callers must set both or omit both. The generated facade rejects a one-sided
value with `ConfigurationError` before credential lookup or HTTP.

## Finding models and request types

Use `parity/manifest.gen.json` to find an operation's generated request function, response type,
parameter type, request-body type, success statuses, and referenced schemas. Use the `schemas` map in
that file to map all component names to Go types. Then use `go doc` for the public Go declaration and
the contract for protocol descriptions and field constraints.

Do not import `internal/wire` or construct a contract path manually. The public `Client` method is the
supported entry point.
