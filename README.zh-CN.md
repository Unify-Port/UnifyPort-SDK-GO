# UnifyPort Go SDK

[English](README.md) | [简体中文](README.zh-CN.md) | [网站](https://www.unifyport.ai) |
[安全策略](SECURITY.zh-CN.md) | [MIT License](LICENSE)

[![CI](https://github.com/Unify-Port/UnifyPort-SDK-GO/actions/workflows/ci.yml/badge.svg)](https://github.com/Unify-Port/UnifyPort-SDK-GO/actions/workflows/ci.yml)

面向公开 UnifyPort Device API 的 contract-first Go SDK。它提供符合 Go 习惯的 `Client`、生成的 operation/model
覆盖、安全优先 HTTP runtime、typed error、安全 response 元数据和有界 cursor pagination。

当前公开 baseline 覆盖 64 个 operation、69 个 component schema 和 11 个资源组。`/v1/accounts/...` 下的路由
管理 provider 账号资源，但仍属于同一个 Device API client。

## 能力与边界

| 资产 | 用途 | 边界 |
| --- | --- | --- |
| `package unifyport` | 公开 client、model、result、error、option 和 pager | 受支持 consumer API |
| `contracts/device.openapi.yaml` | 已批准公开 Device API wire contract | 唯一协议生成输入 |
| `internal/wire` | 生成 OpenAPI HTTP client 和 model | Consumer 不能导入 |
| 安全 runtime | 认证、URL scope、redirect、timeout、retry、limit 和解析 | 每个公开 operation 都必须经过 |
| Parity manifest | Operation/schema 与 Node 行为映射 | 由 generator 生成，不是第二份协议定义 |

本仓库有意不提供通用 raw request、任意 authentication header、MCP server、provider 专用授权编排，也不实现
公开契约尚未定义的 webhook 签名算法。

## 要求

- Go 1.25 或 1.26 支持线中的最新安全 patch；
- Device API base URL；
- 具有所需 workspace 最小权限的 Device API key。

## 安装

```bash
go get github.com/Unify-Port/UnifyPort-SDK-GO@latest
```

应用应使用已审查 tag 替代 `@latest`，并提交 `go.mod` 和 `go.sum`。默认 branch 是开发状态；只有已发布 tag
才是 release。

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	unifyport "github.com/Unify-Port/UnifyPort-SDK-GO"
)

func main() {
	client, err := unifyport.NewClient(unifyport.Config{
		BaseURL: requiredEnv("UNIFYPORT_DEVICE_API_BASE_URL"),
		APIKeyProvider: unifyport.StaticAPIKey(
			requiredEnv("UNIFYPORT_DEVICE_API_KEY"),
		),
	})
	if err != nil {
		log.Fatal(err)
	}

	result, err := client.GetWorkspace(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Print filtered diagnostics only; business data may be sensitive.
	fmt.Printf("status=%d request_id=%s\n", result.StatusCode, result.RequestID)
}

func requiredEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		log.Fatalf("missing required environment variable: %s", name)
	}
	return value
}
```

完整设置见[入门](docs/zh-CN/getting-started.md)和经过编译检查的
[`examples/getting-started`](examples/getting-started/main.go)。Credential 只能来自 `Config.APIKeyProvider`；
operation input 不能覆盖 `X-Api-Key`。

## 安全默认值

| 行为 | 默认值 |
| --- | --- |
| HTTPS | 必须；只有本地测试可显式开启 loopback HTTP |
| Redirect | 不自动跟随 |
| 逻辑 operation timeout | 30 秒 |
| Response limit | 8 MiB，配置硬上限 64 MiB |
| 额外 retry attempt | 2，只对 19 个显式安全 operation 生效 |
| Retry status | `408`、`429`、`502`、`503`、`504`，以及 transport failure |
| Pagination 最大值 | 默认 10,000 页，并检测 repeated/missing cursor |

API-key provider 每个逻辑 operation 只运行一次，所有 attempt 复用其结果。单次 call option 可以取消、关闭或
收紧 timeout/retry 预算，不能扩大全局限制，也不能让 unsafe operation 获得 retry。

Timeout 预算从 `executeOperation` 开始执行时启动，覆盖同步 request build/marshal、credential lookup、HTTP
attempt、retry backoff、有界 response 读取和 response parse。同步 build/parse 无法安全抢占；它们返回后若
deadline 已过，只返回 `TimeoutError`，并丢弃看似成功的结果。

## Result 与 error

每个 operation 返回 `Result[T]`。`Data`、`RequestID` 和 `ClientRequestID` 保留当前公开成功 envelope 的全部
字段；`StatusCode` 和 allowlist `HeaderSnapshot` 增加安全 transport 元数据。不暴露 raw response/body 或
无限制 header map。`204` operation 使用明确 `NoContent` type。

Typed configuration、network、timeout、cancellation、API、response-parse 和 pagination error 支持符合 Go
习惯的 `errors.As`/`errors.Is`。公开 API error 只包含过滤后的 status、code、request ID 和 operation ID；
省略 raw 上游 message、details、body、header、provider error 和 cause。

增加应用日志前请阅读 [Error 处理](docs/zh-CN/errors.md)。成功 `Data` 仍可能包含业务数据或一次性 secret，不能
整体写入日志。

## API 与 model 查找

- [API coverage](docs/zh-CN/api-coverage.md)把全部 64 个 operation ID 映射到 Go method、path、retry 和数据
  分类。
- `parity/manifest.gen.json` 记录生成 request/response type、status、schema reference，以及全部 69 个
  component-to-Go 映射。
- `contracts/device.openapi.yaml` 是协议字段和描述的权威来源。
- `go doc github.com/Unify-Port/UnifyPort-SDK-GO` 显示公开 Go declaration。

不要导入 `internal/wire` 或手工构造 path。

## 验证

```bash
go generate ./...
git diff --exit-code
go mod tidy -diff
go -C internal/generator mod tidy -diff
unformatted="$(gofmt -l $(git ls-files '*.go'))"
test -z "${unformatted}"
go test ./...
go -C internal/generator test ./...
go test -race ./...
go -C internal/generator test -race ./...
go vet ./...
go -C internal/generator vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go -C internal/generator run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
go -C internal/generator run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
bash scripts/check-public-test.sh
bash scripts/check-public-docs.sh
bash scripts/check-public.sh
git diff --check
```

生成后还必须保证 `git status --short` 为空，避免新的 untracked artifact 绕过 diff gate。GitHub Actions 覆盖
Go 1.25 和 1.26 支持线中的最新 patch。

## 仓库结构

```text
.
├── contracts/          # 已批准公开 OpenAPI contract
├── codegen/            # 固定 Go generation 配置和 Go-only overlay
├── internal/
│   ├── generator/      # 独立 nested Go module 中的确定性 generator
│   ├── transport/      # 安全优先 HTTP runtime
│   └── wire/           # 生成 OpenAPI client；不对外公开
├── policy/             # 语言中立 operation 分类
├── parity/             # 公开 Node baseline 和生成 manifest
├── docs/               # Canonical 英文和完整简体中文指南
├── examples/           # 通过编译检查的安全示例
└── *.go                # 公开 unifyport package 与生成 facade/model
```

`internal/generator/go.mod` 独立固定 generation toolchain，不把它放入公开 runtime module。Root module 只包含
生成的 runtime 代码及其 runtime dependency：不会嵌入 OpenAPI 文档，也不会把 generator library 加入
consumer module graph。

## 文档

- [入门](docs/zh-CN/getting-started.md)
- [配置](docs/zh-CN/configuration.md)
- [API coverage 与 model 查找](docs/zh-CN/api-coverage.md)
- [Retry 与 timeout](docs/zh-CN/retry.md)
- [Error 处理](docs/zh-CN/errors.md)
- [Cursor pagination](docs/zh-CN/pagination.md)
- [架构](docs/zh-CN/architecture.md)
- [安全边界](docs/zh-CN/security.md)
- [契约维护](docs/zh-CN/contract-maintenance.md)
- [Node/Go parity](docs/zh-CN/node-parity.md)
- [发布流程](docs/zh-CN/release.md)
- [验收报告](docs/zh-CN/acceptance.md)
- [贡献指南](CONTRIBUTING.zh-CN.md)

真实 staging/provider E2E 是独立 release gate。离线测试、生成 coverage 或 health response 不能证明账号已授权
或 provider workflow 成功。
