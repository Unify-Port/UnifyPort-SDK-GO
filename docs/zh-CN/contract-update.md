# 0.2.0 契约更新（未发布）

[English](../contract-update.md) | [简体中文](contract-update.md)

Go SDK 对齐已批准的公开 Node 源码 [ce60499](https://github.com/Unify-Port/UnifyPort-SDK-Nodejs/commit/ce60499d7169c860ed3f64a686e04bff44316f35)。这是 approved-source 基线，不代表 npm 已发布同等能力。已准备版本 `0.2.0`，尚未打 tag 或发布。详见[发布说明](release-notes.md)和[迁移指南](migration.md)。

## 覆盖与策略

快照包含 78 个 HTTP operation、225 个 component schema、24 个标准 Webhook 事件名及订阅通配符 `*`、40 个专用 Webhook 声明。57 个公开示例覆盖 40 个声明中的 29 个。HTTP 调用与接收方声明继续分开维护。

新增能力包括账号操作查询、浏览器授权会话及 Passkey 提交/确认、添加联系人及联系人关系查询、会话历史请求、帖子/评论和受内测权限限制的通话接口，同时同步账号、workspace、地区、授权及 endpoint 字段。

策略包含 read 25、write 45、destructive 8 个 operation。仍只有原有 19 个显式批准接口自动重试，secret input/output 分别覆盖 9/16 个接口。read 分类本身不授予重试权限，未知接口不可用。幂等能力不等于允许自动重试建号。

## 迁移

- 账号列表从数组改为 `data.items/has_more/next_cursor`，各 SDK result 的读取方式见[迁移指南](migration.md)。

- 创建账号必须提供 `provider`、`region`、`auth_mode`，`name` 可选。原先使用 `AccountRequest` 的位置应区分创建和更新 schema。
- workspace 新增余额字段；余额模式可能缺少旧额度字段，应显式处理缺省。
- `createAccount` 可传调用方提供的 `Idempotency-Key`；只对同一次逻辑创建及相同参数复用。结果不明时先查询，不直接再次建号。
- `deleteAccount` 可返回带账号操作的 `202`，或无数据的旧 `204`。`PENDING`/`PROCESSING` 不代表删除、退款或容量释放完成，应使用返回的 operation ID 查询 `getAccountOperation`。
- 实际切换 `auth_mode` 必须单独提交，且可能清除 Telegram/WhatsApp 的旧会话。PATCH 字段省略保留，显式 null 非法，空数组/对象可清空集合。
- Telegram 已读使用单个 ASCII 十进制 `up_to_message_id`，不传 `up_to_message_sender_id`；WhatsApp 消息级已读按渠道规则成对提供，两者不能被通用 guard 一律绑定。
- QR 启动/检查返回结构化授权元数据，应检查状态和已提供的 payload，不假定每次都有二维码。已有渠道的 `session_url` 导入仍受支持，`whatsapp-protocol` 凭证使用同名嵌套对象。
- 可选 API 错误字段 `NumericCode` 为五位数字字符串，不替代原字符串 code；类型错误独立丢弃。公开异常仍不保留错误正文及上游 details。

Go 调用方需重新编译：`DeleteAccount` 返回 `Result[*AccountOperation]`（204 时为 nil），兼容余额模式的旧额度字段改为指针，QR 结果使用命名模型而非自由 map。其他无正文接口继续使用 `NoContent`。适配器不改公开契约字节或命名 overlay，等价展开 boolean schema，并在保留原约束的同时附加冗余交集结构；变异测试比较适配前后的校验结果。

## Webhook 消费

只合并已提供的字段，保留显式 false、0、空字符串和空选择列表。通话记录乱序到达时保留终态 `ended`，后续事件省略媒体类型时保留邀请阶段的信息。`message.*` 通话记录和 `call.*` 浏览器事件是两个独立事件流。投递成功不证明消费方落库或外部业务结果。

生成类型不校验不可信 HTTP 正文；数值范围、格式及未表达的跨字段条件仍需在接收边界验证，Go 也无法用编译类型完整表达所有禁止字段。本次不新增接收服务或签名算法，未验证生产投递及真实渠道 E2E。
