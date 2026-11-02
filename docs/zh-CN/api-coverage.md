# API 覆盖

[English](../api-coverage.md) | [简体中文](api-coverage.md)

本表把 `parity/manifest.gen.json` 中每个公开 Device API `operationId` 映射到 Go method 和已审查 operation
policy。仓库内契约是 parameter、request/response field、status 和 schema 描述的权威来源。Manifest 记录生成
Go request/response type，以及全部 218 个 component-schema 映射。

只有两种语言的表格都与生成 manifest 包含完全相同的 78 个 operation ID，
`bash scripts/check-public-docs.sh` 才会通过。

| operationId | Go method | 资源 | HTTP path | Retry | 分类 |
| --- | --- | --- | --- | --- | --- |
| `blockContact` | `BlockContact` | Contacts | `POST /v1/accounts/{account_id}/contacts/block` | 否 | write |
| `cancelAccountAuth` | `CancelAccountAuth` | Account Auth | `POST /v1/accounts/{account_id}/auth/cancel` | 否 | write |
| `checkAccountQrAuth` | `CheckAccountQRAuth` | Account Auth | `POST /v1/accounts/{account_id}/auth/qr/check` | 否 | write; secret output |
| `confirmAccountPasskey` | `ConfirmAccountPasskey` | Account Auth | `POST /v1/accounts/{account_id}/auth/passkey-confirm` | 否 | write; secret output |
| `createAccount` | `CreateAccount` | Accounts | `POST /v1/accounts` | 否 | write; secret input; secret output |
| `createAccountAuthSession` | `CreateAccountAuthSession` | Account Auth | `POST /v1/accounts/{account_id}/auth-sessions` | 否 | write; secret output |
| `createApiKey` | `CreateAPIKey` | API Keys | `POST /v1/api-keys` | 否 | write; secret output |
| `createCall` | `CreateCall` | Calls | `POST /v1/calls` | 否 | write; secret output |
| `createContact` | `CreateContact` | Contacts | `POST /v1/accounts/{account_id}/contacts/add` | 否 | write |
| `createGroup` | `CreateGroup` | Groups | `POST /v1/accounts/{account_id}/groups/create` | 否 | write |
| `createWebhookEndpoint` | `CreateWebhookEndpoint` | Webhook Endpoints | `POST /v1/webhook-endpoints` | 否 | write; secret input |
| `deactivateWebhookEndpoint` | `DeactivateWebhookEndpoint` | Webhook Endpoints | `POST /v1/webhook-endpoints/{endpoint_id}/deactivate` | 否 | write |
| `deleteAccount` | `DeleteAccount` | Accounts | `DELETE /v1/accounts/{account_id}` | 否 | destructive |
| `deleteConversationLabel` | `DeleteConversationLabel` | Conversations | `POST /v1/accounts/{account_id}/conversations/labels/delete` | 否 | destructive |
| `deleteWebhookEndpoint` | `DeleteWebhookEndpoint` | Webhook Endpoints | `DELETE /v1/webhook-endpoints/{endpoint_id}` | 否 | destructive |
| `editMessage` | `EditMessage` | Messages | `POST /v1/messages/edit` | 否 | write |
| `getAccount` | `GetAccount` | Accounts | `GET /v1/accounts/{account_id}` | 是 | read; secret output |
| `getAccountAuthState` | `GetAccountAuthState` | Account Auth | `GET /v1/accounts/{account_id}/auth` | 是 | read; secret output |
| `getAccountOperation` | `GetAccountOperation` | Accounts | `GET /v1/account-operations/{operation_id}` | 否 | read |
| `getActiveCall` | `GetActiveCall` | Calls | `GET /v1/calls/active` | 否 | read |
| `getContact` | `GetContact` | Contacts | `GET /v1/accounts/{account_id}/contacts/info` | 是 | read |
| `getContactMembership` | `GetContactMembership` | Contacts | `GET /v1/accounts/{account_id}/contacts/membership` | 否 | read |
| `getConversation` | `GetConversation` | Conversations | `GET /v1/accounts/{account_id}/conversations/info` | 是 | read |
| `getGroup` | `GetGroup` | Groups | `GET /v1/accounts/{account_id}/groups/info` | 是 | read |
| `getGroupInviteCode` | `GetGroupInviteCode` | Groups | `GET /v1/accounts/{account_id}/groups/invite-code` | 是 | read; secret output |
| `getPost` | `GetPost` | Posts | `POST /v1/accounts/{account_id}/posts/info` | 否 | read |
| `getWebhookEndpoint` | `GetWebhookEndpoint` | Webhook Endpoints | `GET /v1/webhook-endpoints/{endpoint_id}` | 是 | read |
| `getWorkspace` | `GetWorkspace` | Workspace | `GET /v1/workspace` | 是 | read |
| `hangupCall` | `HangupCall` | Calls | `POST /v1/calls/{call_id}/hangup` | 否 | destructive |
| `importAccountAuthSession` | `ImportAccountAuthSession` | Account Auth | `POST /v1/accounts/{account_id}/auth/session` | 否 | write; secret input |
| `leaveGroup` | `LeaveGroup` | Groups | `POST /v1/accounts/{account_id}/groups/leave` | 否 | destructive |
| `listAccounts` | `ListAccounts` | Accounts | `GET /v1/accounts` | 是 | read; secret output |
| `listApiKeys` | `ListAPIKeys` | API Keys | `GET /v1/api-keys` | 是 | read |
| `listContactBlocklist` | `ListContactBlocklist` | Contacts | `GET /v1/accounts/{account_id}/contacts/blocklist` | 是 | read |
| `listContacts` | `ListContacts` | Contacts | `GET /v1/accounts/{account_id}/contacts` | 是 | read |
| `listConversationLabels` | `ListConversationLabels` | Conversations | `GET /v1/accounts/{account_id}/conversations/labels` | 是 | read |
| `listConversationMembers` | `ListConversationMembers` | Conversations | `GET /v1/accounts/{account_id}/conversations/members` | 是 | read |
| `listConversations` | `ListConversations` | Conversations | `GET /v1/accounts/{account_id}/conversations` | 是 | read |
| `listGroupJoinRequests` | `ListGroupJoinRequests` | Groups | `GET /v1/accounts/{account_id}/groups/join-requests` | 是 | read |
| `listGroups` | `ListGroups` | Groups | `GET /v1/accounts/{account_id}/groups` | 是 | read |
| `listPostComments` | `ListPostComments` | Posts | `POST /v1/accounts/{account_id}/posts/comments/list` | 否 | read |
| `listPosts` | `ListPosts` | Posts | `GET /v1/accounts/{account_id}/posts` | 否 | read |
| `listProviderRegions` | `ListProviderRegions` | Providers | `GET /v1/providers/{provider}/regions` | 是 | read |
| `listWebhookEndpoints` | `ListWebhookEndpoints` | Webhook Endpoints | `GET /v1/webhook-endpoints` | 是 | read |
| `markConversationRead` | `MarkConversationRead` | Conversations | `POST /v1/accounts/{account_id}/conversations/read` | 否 | write |
| `markConversationUnread` | `MarkConversationUnread` | Conversations | `POST /v1/accounts/{account_id}/conversations/unread` | 否 | write |
| `muteConversation` | `MuteConversation` | Conversations | `POST /v1/accounts/{account_id}/conversations/mute` | 否 | write |
| `pinConversation` | `PinConversation` | Conversations | `POST /v1/accounts/{account_id}/conversations/pin` | 否 | write |
| `pinMessage` | `PinMessage` | Messages | `POST /v1/messages/pin` | 否 | write |
| `reactMessage` | `ReactMessage` | Messages | `POST /v1/messages/reaction` | 否 | write |
| `reconnectAccountRuntime` | `ReconnectAccountRuntime` | Account Runtime | `POST /v1/accounts/{account_id}/runtime/reconnect` | 否 | write |
| `refreshAccountRuntime` | `RefreshAccountRuntime` | Account Runtime | `POST /v1/accounts/{account_id}/runtime/refresh` | 否 | write |
| `replyPostComment` | `ReplyPostComment` | Posts | `POST /v1/accounts/{account_id}/posts/comments/reply` | 否 | write |
| `requestConversationHistory` | `RequestConversationHistory` | Conversations | `POST /v1/accounts/{account_id}/conversations/history/request` | 否 | write |
| `revokeMessage` | `RevokeMessage` | Messages | `POST /v1/messages/revoke` | 否 | destructive |
| `rotateApiKey` | `RotateAPIKey` | API Keys | `POST /v1/api-keys/{key_id}/rotate` | 否 | write; secret output |
| `sendMessage` | `SendMessage` | Messages | `POST /v1/messages` | 否 | write; secret input; secret output |
| `setContactNote` | `SetContactNote` | Contacts | `POST /v1/accounts/{account_id}/contacts/note` | 否 | write |
| `setConversationLabelMembers` | `SetConversationLabelMembers` | Conversations | `POST /v1/accounts/{account_id}/conversations/labels` | 否 | write |
| `setGroupJoinApprovalMode` | `SetGroupJoinApprovalMode` | Groups | `POST /v1/accounts/{account_id}/groups/join-approval-mode` | 否 | write |
| `startAccountAuth` | `StartAccountAuth` | Account Auth | `POST /v1/accounts/{account_id}/auth/start` | 否 | write; secret output |
| `startAccountQrAuth` | `StartAccountQRAuth` | Account Auth | `POST /v1/accounts/{account_id}/auth/qr/start` | 否 | write; secret output |
| `startAccountRuntime` | `StartAccountRuntime` | Account Runtime | `POST /v1/accounts/{account_id}/runtime/start` | 否 | write |
| `stopAccountRuntime` | `StopAccountRuntime` | Account Runtime | `POST /v1/accounts/{account_id}/runtime/stop` | 否 | write |
| `submitAccountAuthCode` | `SubmitAccountAuthCode` | Account Auth | `POST /v1/accounts/{account_id}/auth/code` | 否 | write; secret input |
| `submitAccountAuthPassword` | `SubmitAccountAuthPassword` | Account Auth | `POST /v1/accounts/{account_id}/auth/password` | 否 | write; secret input |
| `submitAccountPasskeyResponse` | `SubmitAccountPasskeyResponse` | Account Auth | `POST /v1/accounts/{account_id}/auth/passkey-response` | 否 | write; secret input; secret output |
| `unblockContact` | `UnblockContact` | Contacts | `POST /v1/accounts/{account_id}/contacts/unblock` | 否 | write |
| `unmuteConversation` | `UnmuteConversation` | Conversations | `POST /v1/accounts/{account_id}/conversations/unmute` | 否 | write |
| `unpinConversation` | `UnpinConversation` | Conversations | `POST /v1/accounts/{account_id}/conversations/unpin` | 否 | write |
| `updateAccount` | `UpdateAccount` | Accounts | `PATCH /v1/accounts/{account_id}` | 否 | write; secret input; secret output |
| `updateApiKeyStatus` | `UpdateAPIKeyStatus` | API Keys | `PATCH /v1/api-keys/{key_id}` | 否 | write |
| `updateGroupInfo` | `UpdateGroupInfo` | Groups | `POST /v1/accounts/{account_id}/groups/update-info` | 否 | write |
| `updateGroupJoinRequests` | `UpdateGroupJoinRequests` | Groups | `POST /v1/accounts/{account_id}/groups/join-requests/update` | 否 | destructive |
| `updateGroupMembers` | `UpdateGroupMembers` | Groups | `POST /v1/accounts/{account_id}/groups/members` | 否 | destructive |
| `updateWebhookEndpoint` | `UpdateWebhookEndpoint` | Webhook Endpoints | `PATCH /v1/webhook-endpoints/{endpoint_id}` | 否 | write; secret input |
| `updateWorkspace` | `UpdateWorkspace` | Workspace | `PATCH /v1/workspace` | 否 | write |
| `upsertConversationLabel` | `UpsertConversationLabel` | Conversations | `POST /v1/accounts/{account_id}/conversations/labels/upsert` | 否 | write |

`MarkConversationReadRequestBody.UpToMessageID` 与 `UpToMessageSenderID` 实现契约中的
`dependentRequired` 配对：调用方必须同时设置或同时省略。单边设置会由生成 facade 在 credential lookup 或
HTTP 前返回 `ConfigurationError`。

## 查找 model 与 request type

通过 `parity/manifest.gen.json` 查找 operation 的生成 request function、response type、parameter type、request-body
type、成功 status 和引用 schema；通过其中 `schemas` map 把全部 component name 映射到 Go type。然后使用
`go doc` 查看公开 Go declaration，并以契约确认协议描述和字段约束。

不要导入 `internal/wire` 或手工构造契约 path。公开 `Client` method 是受支持入口。
