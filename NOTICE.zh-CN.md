# 许可说明

[English](NOTICE.md) | [简体中文](NOTICE.zh-CN.md)

Go SDK 源码和生成的 client facade 按 [`LICENSE`](LICENSE) 中的 MIT License 分发。

`contracts/device.openapi.yaml` 描述对外 API，并保留其自身的 `info.license` 元数据。该元数据记录契约所描述
API 的条款，不会替代本仓库 SDK 源码的 MIT License。不得仅为了与 SDK license 一致而重写契约元数据，
否则会改变已批准的公开协议快照。

生成的 wire type 可能由 `oapi-codegen` 产生。generator 以及其他开发或 runtime 依赖仍分别受其自身
license 约束。本项目不包含第三方生产实现、凭据、客户数据或私有仓库内容。
