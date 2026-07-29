# AGENTS.md

[English](AGENTS.md) | [简体中文](AGENTS.zh-CN.md)

本文件约束本仓库中的维护者和自动化代理。子目录可以通过新的 `AGENTS.md` 增加更严格的规则，
但不能放宽这里定义的契约、安全和公开发布边界。

## 语言与注释

- 协作默认使用简体中文；标识符、命令、配置键、协议字段和错误码保持英文。
- 面向公开用户的文档以英文为 canonical；同一次变更必须同步更新完整的简体中文版本。
- 代码改动确实需要说明时，应新增或更新简短中文注释。注释只解释 why：意图、边界、兼容性、
  外部限制或风险。
- 行为变化时删除过期注释。测试名、代码块和提交信息默认使用英文，除非文件已有更具体的规范。

## 公开仓库边界

- 本仓库只实现已批准公开 Device API 契约中的能力；`/v1/accounts/...` 下的 provider 账号路由仍属于
  同一个 Device API。
- 源码、测试、文档、生成物、日志、提交和 release 中不得包含凭据、客户或账号数据、生产 payload、
  私有 hostname、本地绝对路径、未公开仓库元数据、内部实现细节或未经批准的协议观察结果。
- 代码生成唯一的 wire protocol 输入是 `contracts/device.openapi.yaml`。Go overlay 采用 fail-closed 校验，当前
  只允许一个已批准 response-component 重命名；不能改变 method、path、字段、required、response、认证或
  Go type semantics。
- 不得手工编辑带有 `Code generated` 标记的文件；应修改公开契约、语言中立的 operation policy、overlay
  或 generator 后重新生成。
- 本仓库不包含 MCP server、通用 raw HTTP client，也不实现公开契约尚未定义的 webhook 签名算法。
- 每次公开交付都必须运行 `bash scripts/check-public.sh`；其他测试不能替代这一门禁。

## 修改前

1. 阅读本文件、`README.md` 和 `docs/` 下与任务相关的文档。
2. 使用 `rg` 确认真正的 export、operation ID、generated 标记和现有测试，不根据记忆猜测 API 名称。
3. 检查 `git status`，保留用户和其他代理的工作。
4. 若不同解释会改变公开 API、兼容性或安全边界，先说明选项及影响再选择。
5. 修改契约前，先确认已批准的公开 schema 和 release notes。

## 架构不变量

- 根目录的 `unifyport` package 是受支持的 facade；生成的 HTTP 代码保留在 `internal/`，消费者不能导入。
- `X-Api-Key` 只能来自 client 配置。operation request 和 call option 不能覆盖它、修改目标 base URL，或把它
  发送到配置的 origin/path prefix 之外。
- 默认只允许 HTTPS；不安全 HTTP 例外必须显式开启且仅限 loopback 开发。不得自动跟随 redirect。
- 每个逻辑 operation 只解析一次 API-key provider，并在该 operation 的所有 attempt 中复用。
- 只有明确分类为安全的 operation 才能重试。未知 operation、写操作、破坏性动作、授权提交和一次性 secret
  operation 默认不重试。
- 单次调用可以取消、关闭或收紧全局 timeout/retry 限制，不能扩大全局预算。
- 读取过程中必须实际执行 response limit，不能只信任 `Content-Length`。
- 公开错误只暴露稳定且经过过滤的元数据；不得保留或输出 raw body/header、上游 message/details、凭据、
  provider error 或 raw cause。
- 保证整数精度、明确的 no-content 结果、context cancellation 和有界 cursor pagination，不得用 raw
  transport 逃生口绕过这些行为。

## 契约与生成流程

1. 审查已批准的公开 schema 和 release notes。
2. 更新 `contracts/device.openapi.yaml`。只有 workaround 获得显式批准时才能修改 Go overlay，并在同一次变更
   中扩展其 fail-closed validator/test。
3. 审查每个变更 operation 的认证、mutability、retry 安全性、secret input/output 和 destructive 属性；未知
   分类采用最严格默认值。
4. 更新语言中立的 policy，运行 `go generate ./...`。
5. 审查完整生成 diff；禁止直接修补生成物。
6. 为 transport、error、response limit、整数边界、pagination 和 operation 行为变化增加离线测试。
7. 同步更新完整的英文和简体中文文档。
8. 运行适用的质量门禁和公开发布门禁。

## 验证

完整本地验收顺序为：

```bash
go generate ./...
git diff --exit-code
go test ./...
go test -race ./...
go vet ./...
staticcheck ./...
govulncheck ./...
bash scripts/check-public.sh
git diff --check
```

生成后还要检查 `git status --short`，因为 `git diff --exit-code` 不会报告新的 untracked 生成物。只修改文档时，
至少执行公开信息检查、链接复核和 `git diff --check`。若命令无法执行，必须说明具体原因和剩余风险，不能把
未执行的检查写成通过。

## 交付说明

先说明结果，再列出关键文件、兼容性和安全影响、实际执行的命令以及仍属于外部门禁的事项。除非用户明确
授权，不得 push、打 tag、公开发布或创建 release。
