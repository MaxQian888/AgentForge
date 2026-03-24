## Why

AgentForge 现有插件体系已经具备注册表、清单校验和 TS Bridge 的 ToolPlugin 运行时，但 Go 宿主对可执行插件仍停留在占位状态，无法按 PRD 中约定真实加载和运行 WASM 插件。现在需要补齐 Go 侧真实运行时和对应 SDK，避免插件系统长期停留在“可注册、不可执行”的半成品状态。

## What Changes

- 将 Go 宿主侧的可执行插件从“激活即标记 active”的占位逻辑扩展为真实的 WASM 模块发现、加载、实例化、调用和健康检查链路。
- 为 Go 宿主定义并提供首版 Go WASM 插件 SDK，统一插件入口、宿主能力绑定、请求/响应模型、错误语义和打包约定。
- 扩展插件 manifest 与运行时校验，允许 Go 宿主声明并验证 WASM 模块入口、能力需求、配置注入和兼容版本信息。
- 建立 Go 运行时与插件注册表之间的真实状态同步，让激活、降级、重启和健康信息来自实际运行时，而不是静态推断。
- 提供至少一个仓库内可验证的 WASM 插件样例与端到端验收路径，确保插件能被真实安装、激活、调用和观测。

## Capabilities

### New Capabilities
- `go-wasm-plugin-sdk`: 定义 Go 宿主 WASM 插件的 SDK、宿主接口、打包约定和示例插件契约。

### Modified Capabilities
- `plugin-runtime`: 将 Go 宿主的可执行插件运行时从占位激活扩展为真实的 WASM 模块加载、实例生命周期管理和宿主调用契约。
- `plugin-registry`: 扩展注册表对 Go 宿主运行时状态的要求，确保 WASM 插件的实际激活结果、健康信息和失败原因可被权威记录和查询。

## Impact

- Affected specs: `openspec/specs/plugin-runtime/spec.md`, `openspec/specs/plugin-registry/spec.md`, new `openspec/specs/go-wasm-plugin-sdk/spec.md`
- Affected Go code: `src-go/internal/service/plugin_service.go`, `src-go/internal/model/plugin.go`, `src-go/internal/plugin/*`, `src-go/internal/handler/plugin_handler.go`, runtime/host wiring under `src-go`
- Affected plugin assets and docs: `plugins/**`, `docs/PRD.md` aligned implementation seams, new SDK/sample plugin materials
- New dependency surface: Go-side WASM runtime and SDK packaging workflow for real plugin execution
