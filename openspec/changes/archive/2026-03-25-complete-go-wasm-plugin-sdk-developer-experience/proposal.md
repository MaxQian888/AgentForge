## Why

`docs/PRD.md`、`docs/part/PLUGIN_SYSTEM_DESIGN.md` 和现有 `openspec/specs/go-wasm-plugin-sdk/spec.md` 都已经把 Go-hosted/WASM 插件 SDK 定义成 AgentForge 插件生态的重要基础，但仓库里的 `src-go/plugin-sdk-go/sdk.go` 仍然停留在单文件最小运行时包装：作者侧只能围绕 `map[string]any` 与环境变量约定自行拼装行为，缺少 typed contract、结构化错误/结果 helper、显式执行上下文、统一导出封装，以及稳定的样例/模板验证链路。现在需要把 Go SDK 这一条 seam 单独补齐，否则后续 Workflow 或其他 Go-hosted 插件即使 runtime 存在，也会继续建立在脆弱且易漂移的作者体验之上。

## What Changes

- 扩展 Go-hosted WASM 插件 SDK，从最小 `Describe/Init/Health/Invoke` 包装升级为 manifest-aligned 的 typed authoring contract，补齐插件描述、能力声明、执行上下文、结构化结果和错误辅助类型。
- 为 Go SDK 增加稳定的导出/autorun helper，减少样例和未来模板里重复手写 `agentforge_abi_version`、`agentforge_run` 与 envelope 编排逻辑的样板代码。
- 定义并补齐 Go-hosted 插件作者工作流，包括仓库内样例/模板、构建辅助、验证入口，以及与当前 `runtime: wasm` + `wazero` 宿主真相一致的使用方式。
- 对齐 Go SDK 文档、样例和验证策略，确保 SDK 暴露的能力与当前主规格和运行时行为一致，不再让 PRD/设计文档、主 spec、样例代码三者各说各话。
- 明确当前 Go SDK 的支持边界：先围绕 Go-hosted/WASM 插件作者体验闭环补齐，不在这条 focused change 里并入 TypeScript SDK、公开 Marketplace 或完整 Workflow/Review 平台化范围。

## Capabilities

### New Capabilities
- `go-plugin-sdk-authoring-workflow`: 定义 Go-hosted WASM 插件在仓库内的标准作者工作流，包括模板或样例、构建辅助、验证入口和文档对齐要求。

### Modified Capabilities
- `go-wasm-plugin-sdk`: 将现有规格从“最小可运行 SDK”扩展为“typed authoring contract + bounded execution context + structured result/error helpers + stable export helpers + verifiable sample/template”。

## Impact

- Affected specs: new `go-plugin-sdk-authoring-workflow`; modified `go-wasm-plugin-sdk`
- Affected Go areas: `src-go/plugin-sdk-go`, `src-go/cmd/sample-wasm-plugin`, `src-go/internal/plugin/runtime.go`, Go-hosted plugin verification tests, and related build helpers under `scripts/`
- Affected docs: `docs/GO_WASM_PLUGIN_RUNTIME.md`, `docs/PRD.md`, `docs/part/PLUGIN_SYSTEM_DESIGN.md` references that currently imply a richer Go SDK surface than the repository actually exposes
- Affected developer flow: repository-supported Go plugin authoring, local sample builds, runtime verification, and future Go-hosted plugin template maintenance
- New dependency surface: typed SDK structs/helpers, sample/template verification coverage, and build helper conventions for Go-hosted WASM plugin artifacts
