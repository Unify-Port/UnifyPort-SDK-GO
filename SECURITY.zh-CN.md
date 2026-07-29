# 安全策略

[English](SECURITY.md) | [简体中文](SECURITY.zh-CN.md)

## 报告漏洞

请通过仓库的 [GitHub Security Advisory 表单](https://github.com/Unify-Port/UnifyPort-SDK-GO/security/advisories/new)
私下报告漏洞。不得在公开 issue 中包含利用细节、凭据、账号或客户数据、request/response body、日志或私有
基础设施信息。

报告应包含受影响的 SDK 版本、最小合成复现、预期行为和安全影响。所有凭据、标识符、URL、消息和个人数据
都必须替换为明确占位符。不要附加 `.env`、抓包文件、生产日志或未脱敏 trace。

## 支持版本

安全修复以最新 stable release 为目标。在 `v0.1.0` 发布前，pre-release 版本按 best-effort 支持。只有维护者
明确公告延长支持窗口时，旧版本才可能继续获得修复。

## 安全边界

SDK 设计上负责保证：

- API key 仅用于配置的 origin 和 path prefix；
- 不自动跟随 redirect；
- 只有显式安全 operation 才能在有限预算内重试；
- timeout、response size 和 pagination 都有有限上限；
- 整数处理不会损失精度；
- 公开错误只包含经过过滤的元数据，不保留 raw body/header、上游 detail 或 cause；
- 不提供绕过 operation policy 的通用 raw request API。

应用仍需负责安全保存 secret、使用配置恰当的 `http.Client`、避免敏感日志、为 API key 应用最小权限，并及时
升级到受支持的 SDK 版本。本地测试通过不代表真实 provider 已授权；staging/provider 端到端验证是独立的
release gate。

详细 threat model 和实现规则见 [`docs/security.md`](docs/zh-CN/security.md)。
