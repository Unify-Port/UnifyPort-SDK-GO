# 示例

[English](README.md) | [简体中文](README.zh-CN.md)

每个示例都是独立 Go package，并由 `go test ./...` 编译。

| 示例 | 用途 | 外部访问 |
| --- | --- | --- |
| [`getting-started`](getting-started/main.go) | 构造安全 client 并调用 `GetWorkspace` | 调用配置的 Device API |
| [`list-contacts-pager`](list-contacts-pager/main.go) | 把 `ListContacts` 适配到 `Pager[Contact]`，不记录 contact/cursor | 调用配置的 Device API |
| [`pagination`](pagination/main.go) | 使用合成页面展示有界 cursor iteration | 无 |

运行离线 pagination 示例：

```bash
go run ./examples/pagination
```

Live 示例从 process environment 读取 `UNIFYPORT_DEVICE_API_BASE_URL` 和 `UNIFYPORT_DEVICE_API_KEY`；contact
示例还读取 `UNIFYPORT_ACCOUNT_ID`。真实值不得写入源码、shell history、`.env`、test output 或 issue report。
示例只记录过滤后的 SDK 元数据或汇总数量，不输出业务记录或 cursor。

设置方法见[入门](../docs/zh-CN/getting-started.md)，生产 error 分支见[Error 处理](../docs/zh-CN/errors.md)。
