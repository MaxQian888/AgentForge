## Why

`PRD.md` 和 `PLUGIN_SYSTEM_DESIGN.md` 都把 Tauri 定义为 AgentForge 桌面模式的增强层，要求它承担双 sidecar 启动、原生通知、文件选择、托盘、快捷键、自动更新与插件事件转发等职责；但当前仓库里的 `src-tauri` 仍停留在模板级实现，只启动单个 Go sidecar 并暴露 `get_backend_url`。现在需要先把这部分缺口收敛成可实现、可验证的 OpenSpec 契约，避免桌面模式长期停留在“文档完整、产品壳不完整”的状态。

## What Changes

- 为 AgentForge 桌面模式定义完整的 Tauri sidecar 监督能力，覆盖 Go Orchestrator 与 TS Bridge 的启动顺序、就绪信号、健康检查、崩溃重启和前端可消费的运行态。
- 为桌面端定义统一的原生能力层，覆盖原生文件选择、系统通知、托盘状态、全局快捷键和自动更新，并明确每项能力在 Web 模式下的 fallback 语义。
- 为 Tauri 与前端之间定义平台能力访问约定，要求前端通过统一平台能力入口消费桌面能力，而不是零散直接调用 `invoke(...)`。
- 补齐 Tauri 与插件/运行时面板之间的桌面桥接契约，包括插件状态查询、sidecar 或插件事件转发、以及桌面专属事件不侵入业务主链路的边界。
- 约束桌面构建与分发元数据，使 Tauri 包名、sidecar 打包、capability 权限声明和 AgentForge 品牌信息与当前产品一致，而不再保留模板仓库残留。

## Capabilities

### New Capabilities
- `desktop-sidecar-supervision`: 定义 Tauri 桌面壳如何管理 Go Orchestrator 与 TS Bridge sidecar 的启动、就绪、健康、重启与状态暴露。
- `desktop-native-capabilities`: 定义桌面模式下的原生文件选择、系统通知、托盘、全局快捷键、自动更新与对应的 Web fallback 合同。
- `desktop-runtime-event-bridge`: 定义 Tauri 如何向前端暴露插件状态、运行时事件与桌面专属系统事件，同时保持业务通信主链路仍为 Frontend ↔ Go ↔ TS。

### Modified Capabilities

## Impact

- 受影响文档与设计基线包括 [docs/PRD.md](d:/Project/AgentForge/docs/PRD.md)、[docs/part/PLUGIN_SYSTEM_DESIGN.md](d:/Project/AgentForge/docs/part/PLUGIN_SYSTEM_DESIGN.md) 和现有桌面设计残留配置。
- 后续实现预计会触达 `src-tauri/src/lib.rs`、`src-tauri/Cargo.toml`、`src-tauri/tauri.conf.json`、`src-tauri/capabilities/default.json`，以及前端平台能力入口与插件管理/运行态消费层。
- 会影响桌面构建产物与 sidecar 打包方式，包括是否同时分发 `server` 与 `bridge` 二进制、如何上报运行态，以及桌面权限声明的最小化策略。
- 该变更只补齐 Tauri 桌面增强层，不改变插件业务通信主链路，也不在本次内实现完整插件市场、远程部署控制台或非桌面模式专属能力。
