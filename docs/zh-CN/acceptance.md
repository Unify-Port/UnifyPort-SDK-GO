# 验收报告

[English](../acceptance.md) | [简体中文](acceptance.md)

报告日期：2026-07-29

本文件保留 2026-07-29 的初始验收记录。当前未发布的 78 个接口/225 个 schema 更新与兼容变化见[本次契约更新](contract-update.md)；以下历史门禁不代表本次发布或当前 CI 状态。

## 结论

当前源码 candidate 在下文报告的本地 toolchain 上满足离线架构、生成、API 覆盖、runtime、文档、consumer
隔离和公开信息门禁。但这不代表已发布 release 或真实 provider 成功：GitHub 托管 CI、干净 tagged-commit
检查、released-module 验证和受控 staging/provider E2E 仍是独立 gate。

## 已验收范围

| 要求 | 证据 | 结果 |
| --- | --- | --- |
| 公开 Device API 覆盖 | 生成 manifest 和 generator test | 64 个 operation、69 个 component schema、11 个资源 tag |
| 安全 policy parity | 生成 manifest 和 policy test | 19 个 retryable、7 个 destructive、8 个 secret-input、8 个 secret-output operation |
| Public facade | 生成 `Client` method 和编译测试 | 全部 64 个 operation ID 映射到 Go method |
| Contract-only wire 输入 | OpenAPI snapshot、fail-closed Go overlay test | 通过；只接受已批准 `RuntimeActionResult` response 重命名 |
| Request dependency 校验 | 生成 facade 与 runtime regression test | OpenAPI `dependentRequired` 配对在 credential/HTTP work 前拒绝非法输入 |
| Internal generated client | Go `internal/` import boundary | 公开 consumer 不能导入 raw wire client |
| Generator 隔离 | Nested module 与生成物检查 | Toolchain 固定在 `internal/generator`；root runtime graph 不含 embedded spec 或 generator-only dependency |
| Clean consumer | `TestCleanConsumerCompilesWithoutCodegenDependencies` | `GOWORK=off` external module 真实 import/compile SDK，并拒绝四类 generator-only module |
| Result envelope | Integration/result test | 保留当前全部公开 envelope 字段；排除 raw response/body |
| Package 文档 | Root `doc.go` 与 `go doc` 查找 | 不暴露内部实现即可发现公开 package 用途和安全边界 |
| 文档 | Canonical 英文和完整简体中文指南 | 架构、安全、维护、parity、使用、release 和 64/64 coverage 均存在 |
| 示例 | `go test ./...` 编译每个 example package | 入门、真实 list-operation pager 适配和离线 pager |

## 已运行本地命令

以下命令针对源码 candidate 执行成功：

```bash
go generate ./...
go mod tidy -diff
go -C internal/generator mod tidy -diff
test -z "$(find . -type f -name '*.go' -not -path './.git/*' -print0 | xargs -0 gofmt -l)"
go test ./...
go -C internal/generator test ./...
go -C internal/generator test -run TestCleanConsumerCompilesWithoutCodegenDependencies -v
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
go doc github.com/Unify-Port/UnifyPort-SDK-GO
git diff --check
```

运行 generation 前后分别计算 `internal/wire/device.gen.go`、`parity/manifest.gen.json`、`models.gen.go` 和
`operations.gen.go` 的 SHA-256，所有输出字节完全一致；nested generator check-mode 和 clean-consumer test
也通过。

Go 1.25.12 和 Go 1.26.5 分别通过 generation、root/nested tidy-diff、禁用 cache 的 test、禁用 cache 的 race
test 和 vet；生成制品 SHA-256 在单个版本内及两个 toolchain 之间完全一致。随后使用 Go 1.26.5 对两个 module
运行 staticcheck v0.7.0 和 `govulncheck` v1.6.0，全部通过，两次漏洞扫描均未发现可达已知漏洞。这仍只代表
本地证据，不宣称 GitHub 托管 Go 1.25.x/1.26.x matrix 已通过。

由于本地正在创建的项目与初始仓库必然存在差异，`git diff --exit-code` 有意保留给已提交 candidate 的 CI。
Workflow 会在 generation 后执行该命令，并额外拒绝 untracked generated file。

## 边界覆盖

离线 test 覆盖：

- base URL 结构、HTTPS/loopback 规则、path scope，以及 credential lookup 前拒绝非法目标；
- redirect 拒绝、cookie-jar 移除，以及 nil/custom `http.Client` 规范化；
- 每个逻辑 operation 只调用一次 credential provider，且不暴露 raw provider/network cause；
- safe/unsafe retry policy、可重放 body、临时 status、`Retry-After`、jitter 和可取消 backoff；
- 从 `executeOperation` 开始、覆盖 build、credential、HTTP/retry/read 和 parse 的共享 timeout 预算；可感知
  context 的工作会及时取消，同步 build/parse 可能在 deadline 后才返回，但只能得到 `TimeoutError`，不能成功；
- 过滤 API diagnostics、response header、envelope metadata 和 malformed-success 脱敏；
- operation catalog 完整性、optional request body、success-status 行为、生成 manifest freshness，以及拒绝
  `x-go-type` 或额外 overlay target，并在 credential/HTTP work 前执行生成的 `dependentRequired` preflight；
- root/nested module 隔离、不嵌入 OpenAPI spec、真实 clean-consumer 编译，以及 consumer graph 不含
  generator-only dependency；
- lazy pagination、empty page、missing/repeated cursor、page limit、fetcher error 和 context cancellation；
- 每个公开示例的 compile-time coverage。

Test 只使用合成值和本地 HTTP server，不使用真实 API key、账号 identifier、授权 payload、客户记录或 provider
message。

## 文档与公开边界

文档与公开边界检查验证：

- 两种语言 API coverage 表与生成 manifest 包含完全相同的 64 个 operation ID；
- 每个 canonical guide 都有简体中文 counterpart、本地 Markdown target 存在，root package 文档位于 `doc.go`；
- 不存在高风险 credential/token/private-key pattern、开发者绝对路径、私有 network URL、私有 repository
  metadata 或敏感 filename；
- 独立 fixture 覆盖每类 scanner 规则：private key、access token、JWT、production-like key、hard-coded key
  assignment、developer path、private URL、private metadata 和 sensitive filename；安全合成 placeholder 也可通过。

自动扫描补充人工 review，但不能证明任意敏感数据一定不存在。Release review 仍要检查完整 committed diff 和
公开 archive。

## 不宣称完成的外部门禁

| Gate | 状态与所需证据 |
| --- | --- |
| GitHub Actions | Workflow 选择 Go 1.25/1.26 最新 patch；必须在精确 committed release candidate 上通过 |
| 干净 committed generation | 在 CI/release checkout 执行 `go generate ./...`、`git diff --exit-code`，且 `git status --short` 为空 |
| Staging/provider E2E | 离线验收未执行；需要受控 test workspace 和脱敏结果 |
| 公开 tag/module resolution | 不宣称已有 tag 或 GitHub Release；获得授权后用真实 import 源码编译 released version，并检查 module graph |
| 漏洞时效性 | 本地 `govulncheck` 只代表当前时间点；release 前立即重跑 |

Committed CI gate 通过后可继续准备 `v0.1.0-alpha.1`。在受控 staging/provider E2E 通过且授权维护者批准 release
之前，stable `v0.1.0` 仍处于 blocked 状态。
