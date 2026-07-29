# 安全边界

[English](../security.md) | [简体中文](security.md)

## Threat model

SDK 设计用于降低以下风险：

- API key、provider 授权 code/password/session payload、webhook secret 或一次性输出进入源码、日志、错误、
  fixture 或公开制品；
- credential 被发送到非预期 origin/path，或跨 redirect 转发；
- 自动重试重复执行改变状态或携带 secret 的 operation；
- 无上限 response、retry delay 或 pagination 消耗进程资源；
- raw 上游错误内容或 transport cause 进入 telemetry；
- 整数缩窄导致调用方操作错误资源；
- generated/raw client 绕过已审查 operation policy。

SDK 不能替代应用的 secret management、API-key rotation、authorization policy 或真实 provider 验证。

## 配置与 URL 限制

Client 构造时拒绝 relative URL、包含 user information/query/fragment 的 URL，以及不支持的 scheme。默认必须
使用 HTTPS；只有显式开启时，loopback 开发环境才能使用明文 HTTP。

Runtime 会规范化 base path，并保证每个生成目标都处于相同 scheme、host、有效 port 和 path prefix 内。
Operation input 不能提供 base URL 或任意 path。禁止跟随 redirect，避免 authentication header 被重放到其他
目标。

生产环境不得开启 loopback 例外。包括测试 URL 在内，URL 中都不得放置 credential。

## Credential 生命周期

`X-Api-Key` 只能来自配置的 `APIKeyProvider`。每个逻辑 operation 只解析一次 provider；retry 复用该结果，
避免轮换 callback 在调用中途改变身份。Provider error 转换为稳定 configuration/credential error，不保留其
message 或 cause。

Credential 不得进入：

- operation request struct 或 call option；
- `String`、Go syntax formatting、JSON、error 或日志；
- test snapshot、example、`.env` 或生成物；
- URL query、telemetry attribute 或 retry 诊断信息。

应用应从 secret manager 或 runtime environment 读取 key，并为每个 key 设置 workspace 最小权限。

## Retry、timeout 与 cancellation

只有显式 `retryable` policy 才允许重试。Transport failure 和 `408`、`429`、`502`、`503`、`504` 只对
allowlist operation 视为临时错误。Runtime 优先遵守合法且有上限的 `Retry-After`，否则使用有上限的 full
jitter。Attempt 次数和等待时间始终有限。

写操作、destructive action、账号授权提交和返回一次性 secret 的 operation 不自动重试。HTTP method 本身
从来不能证明 idempotency。

每个调用都有有限 SDK deadline 预算，并接受 `context.Context`。Cancellation 会及时停止可感知 context 的
credential、HTTP、backoff 和 response read，并阻止下一次 attempt。单次 call option 只能收紧，不能扩展配置预算。

Deadline 从 `executeOperation` 执行入口启动，由同步 request build/marshal、credential lookup、HTTP/retry、
有界 body read 和 parse 共用。Build/parse 保持同步执行，不放入可能泄漏的 goroutine；任一阶段完成后若 deadline
已过，只返回脱敏 `TimeoutError`，不能返回成功。

## Response 与 error 处理

Runtime 在读取过程中限制字节数，不信任 `Content-Length`。对超限、截断、空、非 JSON 或格式错误 response
的处理不会把 raw body 复制到 error。

公开 API error 只能暴露：

- HTTP status；
- 经过字符白名单和长度限制的 API `code`；
- 经过字符白名单和长度限制的 request ID；
- 稳定 operation ID；
- 由 SDK 生成的通用 message。

公开 error 不保留 raw response/header/body、server message/details、provider payload 或底层 cause。
Network、timeout 和 cancellation error 遵守相同 no-cause 规则，同时通过 typed error 和适用的 `errors.Is`
保持可区分。

成功 response 可能包含业务数据或一次性 secret。返回 envelope 应按敏感数据处理，避免 `%+v` 日志，只把
明确批准的字段复制到 telemetry。

## Secret operation

语言中立 policy 分别标记 secret input 和 output。Secret-bearing operation 属于公开 Device API，因此直接
应用代码仍可调用；但不能仅因同 method 的另一个 operation 安全，就获得自动 retry。

一次性 API key、invite/authorization material、verification code、password、session payload、message
content 和 webhook signing secret 必须留在受控内存，不能进入示例、测试、issue report 或日志。

## 整数与数据完整性

生成的 integer type 跟随契约。每个 `int64`/`uint64` 变化都要审查范围和 signed semantics，identifier 或
cursor 不能通过 floating point 转换。格式未知的数字标识符应在公开契约中保持 string。

## 自定义 HTTP client

即使应用提供 HTTP client，认证和 redirect 行为仍由 SDK 控制。注入 client 用于 connection pool、proxy、
TLS 配置和离线测试；不得用于注入认证、改写目标、跟随 redirect 或捕获敏感 payload。Test double 只保存断言
真正需要的最少合成字段。

## 依赖与 release 控制

- Generator 版本固定在 `internal/generator/go.mod`，分析工具也固定版本；升级必须显式审查。
- GitHub Actions token 只读，并固定 action revision。
- Release 前分别在 root 与 `internal/generator` module 运行 `govulncheck ./...` 并审查发现。
- 对完整工作树运行 `bash scripts/check-public.sh`，并人工审查 release diff。
- Staging/provider E2E 数据留在本仓库之外，只公开脱敏 gate 结果。

## 审查清单

每次公开行为变更都要回答：

1. Credential 是否可能离开配置的 origin/path 或跨 redirect？
2. Operation 是否改变状态，retry 安全性是否得到明确证明？
3. 输入或输出是否包含 secret、个人数据或大 payload？
4. Timeout、cancellation、byte limit 和 pagination bound 是否仍然有限？
5. 公开 error、result、logger 或 test double 是否可能暴露 raw data？
6. 整数范围和 no-content 行为是否精确表达？
7. 哪些离线测试和外部门禁证明了这些边界？
