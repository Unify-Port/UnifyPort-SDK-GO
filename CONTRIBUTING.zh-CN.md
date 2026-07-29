# 贡献指南

[English](CONTRIBUTING.md) | [简体中文](CONTRIBUTING.zh-CN.md)

感谢你帮助改进 UnifyPort Go SDK。本项目是对外公开、contract-first 的项目，每次变更都必须保持下面的
协议、安全和信息披露边界。

## 开发环境

使用 Go 1.25 或 1.26 支持线中的最新安全 patch，并在仓库根目录执行：

```bash
go mod download
go generate ./...
go test ./...
```

如需运行完整 release gate，请安装与 CI 相同版本的可选工具：

```bash
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
```

## Contract-first 变更

不得手改生成文件。应从已批准的公开 Device API schema 或 release note 开始，更新
`contracts/device.openapi.yaml`，按需更新显式 operation policy，再重新生成。Go 专用 overlay 当前只包含一个
采用 fail-closed 校验的已批准 response-component 重命名；它不是通用命名或类型覆盖机制，也不能改变 wire
semantics。

每个 operation 都要审查认证、副作用、重试安全、destructive 属性、secret input/output、response limit
和整数精度。未知行为采用保守默认值：不自动重试，也不增加新的逃生口。

## 代码与文档

- 公开 API 保持符合 Go 习惯、精简且包含 GoDoc。
- 生成的 transport 代码保留在 internal，并统一经过安全 runtime。
- 只有在解释意图、边界、兼容性、外部限制或风险时，才增加简短中文注释。
- 测试和示例只使用合成数据；不得使用真实 API key、授权 payload、标识符、消息、hostname 或 response
  body。
- 英文文档是 canonical；同一个 pull request 必须同步更新完整的简体中文版本。

## Pull request 检查表

- [ ] 变更属于已批准的公开 Device API 范围。
- [ ] 生成文件来自契约、policy、overlay 或 generator，而不是手工修改。
- [ ] 已说明兼容性和安全影响。
- [ ] 离线测试覆盖新增行为与失败边界。
- [ ] 英文和简体中文文档保持一致。
- [ ] `go generate ./...` 后不存在无法解释的 tracked 或 untracked 漂移。
- [ ] 测试、race test、vet、静态分析、漏洞扫描、公开内容扫描和 `git diff --check` 已通过；若某个外部门禁
      无法执行，已明确说明。

完整命令见 [`AGENTS.md`](AGENTS.zh-CN.md)。Pull request 的验证过程不得发布、轮换或索取真实凭据。
