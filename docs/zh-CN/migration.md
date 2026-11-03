# 迁移到 0.2.0

[English](../migration.md) | [简体中文](migration.md)

本指南覆盖 `github.com/Unify-Port/UnifyPort-SDK-GO` `0.1.0` → `0.2.0`。源码已准备 `0.2.0`，该版本尚未发布。升级前应审查以下变化：1.0 之前的 minor 版本可能包含不兼容变更。

## 账号与响应变化

| 原先的假设 | 必须完成的迁移 |
| --- | --- |
| 账号列表响应的 `data` 是数组 | HTTP 正文现在使用 `data.items`、`data.has_more`、`data.next_cursor`。通过 SDK result 读取 `result.Data.Items`；原样回传不透明游标，直到 `has_more` 为 false。 |
| 创建和更新共用 `AccountRequest` | 按用途替换为 `AccountCreateRequest` 或 `AccountUpdateRequest`。创建必须提供 `provider`、`region`、`auth_mode`，`name` 可选。 |
| PATCH 的 null 等价于省略 | 省略保留现值，显式 null 非法；空数组/对象可清空集合。实际切换 `auth_mode` 必须单独提交，可能清除 Telegram/WhatsApp 会话。 |
| 删除总是返回 204 且已经完成 | 按 HTTP 状态分支：202 携带账号操作，204 的 data 为 `nil`。使用 `operation_id` 查询 `getAccountOperation`；`PENDING`/`PROCESSING` 不证明删除、退款或容量释放完成。 |
| workspace 的旧额度字段总是存在 | 余额模式应处理额度字段缺省，不把缺失信息伪装为零余额。 |
| QR 响应总是包含图片 | 检查授权状态和可选载荷；QR 启动/检查返回结构化元数据。 |

调用方提供的 `Idempotency-Key` 只对相同逻辑创建及相同参数复用。写入结果不明时先核对，再决定是否提交；幂等能力不会启用 SDK 写操作重试。已有 `session_url` 导入继续支持，`whatsapp-protocol` 凭证使用同名嵌套对象。

Telegram 消息已读使用单个 ASCII 十进制 `up_to_message_id`，不传 `up_to_message_sender_id`；WhatsApp 消息级已读遵循渠道特有的 message/sender 配对规则，不应跨渠道统一强制配对。可选 API 错误字段 `NumericCode` 为经过验证的五位数字字符串，用于补充原字符串错误码，不暴露原始错误正文。

## Go 调用方调整

`DeleteAccount` 从 `Result[NoContent]` 改为 `Result[*AccountOperation]`，访问 `Data` 前先检查 `StatusCode`。兼容余额模式的额度字段改为指针，QR 响应使用具名 model 而不是自由 map。使用公开 model alias 和联合类型的代码需重新编译；其他无内容操作保留 `NoContent`。删除后通过 `GetAccountOperation` 查询，幂等键放在 `CreateAccountParams.IdempotencyKey` 中。

## Facebook 与 Webhook 消费方

- 修正后的消息/Webhook 联合类型包含 sticker/poll/link_preview、member/all 提及、候选载荷共同字段、decoded/unavailable 投票、关闭的设置、取消置顶及稀疏通话记录。应复查现有构造代码和事件分支，不通过退化为无类型对象绕过检查。

- 穷举 provider 的代码增加 `facebook`。使用 session 授权，刷新运行状态并等待 `runtime_status=running` 后发送；导入或授权成功不证明账号已经运行。
- Facebook 首期只覆盖已同步、可发送的 E2EE 私聊，以及文本或单个媒体附件发送。群组、caption、mentions、引用发送、联系人/动作、帖子/评论和通话不在范围内；共享请求类型不代表渠道能力。详见 [Facebook](facebook.md)。
- `facebookMessageReceived`、`facebookAccountAuthSucceeded`、`facebookAccountStatusUpdated`、`facebookAccountAuthFailed` 是接收方声明，复用已有标准事件名，不新增 `facebook.*` 事件名或可调用的接收方法。
- `message.received` 可以是 inbound 或 outbound。保留字符串 ID、附件顺序、缺失 URL、显式 false/0/空值及时间的全部精度。`metadata.duration` 的单位未确认，不等同于 `duration_ms`；收到引用关系不代表可以发送引用消息。
- 只合并已提供字段，区分连接错误与授权失效。乱序通话更新保留终态，`message.*` 通话记录与 `call.*` 浏览器事件分开处理。校验原始正文，复用既有签名流程并按顶层事件 ID 去重；类型不保证 exactly-once 投递。

## 升级与回退清单

1. 仅在版本公开可用后固定安装目标版本；准备阶段使用本地产物或 module replacement 验证。
2. 重新运行应用的类型/静态检查，验证账号分页、两种删除状态、可选额度/QR 字段、provider 分支及 Webhook fixture。
3. 保持 `X-Api-Key` 注入、凭证脱敏和 19 个操作的重试白名单，不自动重试新增操作。`accepted`、HTTP 2xx 和离线测试不证明投递或已读。
4. 单独记录受控 staging/provider E2E。本次准备不声称真实账号验证或服务部署完成。
5. 回退时固定旧 SDK 并恢复兼容的应用代码。降级包不会回退服务器响应结构、账号状态或已发生的副作用；保留分页/删除兼容处理，或先停止受影响流程，不重放结果未知的写入。

覆盖范围、版本来源及剩余发布门禁见[发布说明](release-notes.md)。
