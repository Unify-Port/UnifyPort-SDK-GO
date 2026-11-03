# Facebook 渠道

[English](../facebook.md) | [简体中文](facebook.md)

本次源码更新为现有 Device API client 增加 `provider=facebook`，不增加 HTTP 路由或标准事件名，也不表示包已发布或服务已部署。

创建账号时指定 `provider=facebook`、`auth_mode=session` 和可分配的 `region`。通过 `POST /v1/accounts/{account_id}/auth/session` 提交短期有效的 HTTPS `session_url`，随后查询 `POST /v1/accounts/{account_id}/runtime/refresh`。会话材料导入或 authorized 不证明运行就绪；发送前需确认 `runtime_status=running`。会话材料及其 URL 属于凭证，不进入日志或模型输入。

使用 `GET /v1/accounts/{account_id}/conversations`，指定 `type=user`。列表只覆盖当前同步且可发送的 E2EE 单聊，不代表完整历史。原样保留不透明游标；过滤后的空页在 `has_more=true` 时不表示结束。发送时原样使用返回的字符串会话 ID。

每次只发送一条文本或一个图片、视频、音频、document/file 附件。群、频道、扫码或验证码授权、caption、mentions、引用回复、会话详情和写动作、联系人、消息操作、帖子评论及通话不在首期能力范围。共用请求类型不表示每个渠道支持全部选项。`accepted` 只表示渠道确认受理，不证明送达或已读。结果未知时不自动重发，不扩大重试 allowlist。

## 标准 Webhook

源码契约提供 `facebookMessageReceived`、`facebookAccountAuthSucceeded`、`facebookAccountStatusUpdated` 和 `facebookAccountAuthFailed` 四个声明，复用 `message.received`、`account.auth.succeeded`、`account.status.updated` 和 `account.auth.failed`，不引入 `facebook.*` 事件名。实际开放情况和真实账号投递需单独验收。

- `message.received` 可以是 `direction=inbound` 或 `outbound`。ID 始终保留字符串，包括超出 JavaScript 安全整数范围的值。接收消息中的 `reply_to_message_id` 只是上下文，不代表允许引用发送，也不是 `reply_token`。
- 普通文本和媒体省略 `message.type`；单个无正文贴纸可为 `sticker`。保留附件顺序、缺失媒体 URL、零大小或时长、显式 `false`。不为加密或不可用媒体补造 URL。
- 附件 `metadata.duration` 保留单位尚未确认的原值，不作为 `duration_ms`。
- 公开时间为 UTC RFC3339，保留可用小数精度。没有原生消息时间时省略 `sent_at`；`occurred_at` 可为观察时间，不等同于消息创建时间。
- 普通连接错误使用 `account.status.updated`、`runtime_status=error`，省略 `auth_status`，不撤销授权。已确认的授权失效使用 `account.auth.failed`、`auth_status=failed`、`runtime_status=disconnected`。
- 沿用原始正文验签和顶层事件 ID 去重。重试保留原 ID；SDK 不生成 ID，不承诺恰好一次投递。未知回调和帖子评论不作为 FB 标准业务事件。

四个接收声明只提供模型，不生成可调用的 API 方法；自动重试策略保持不变。
