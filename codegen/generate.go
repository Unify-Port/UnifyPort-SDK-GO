// Package codegen 固化生成入口，避免开发者手工编辑 wire 与 parity 产物。
package codegen

// overlay 先做结构校验，确保它只携带 Go codegen extension，不改变公开 wire semantics。
//go:generate go -C ../internal/generator run . -root ../.. -validate-overlay

// CLI 由 nested module 固定，避免生成依赖进入 SDK 使用方的 module graph。
//go:generate go -C ../internal/generator tool oapi-codegen --config ../../codegen/oapi-codegen.yaml ../../contracts/device.openapi.yaml

// wire 先生成，parity 才能从真实 Go 标识符反查映射，避免重复实现命名规则。
//go:generate go -C ../internal/generator run . -root ../..
