## Why

当前仓库里的 `pnpm tauri:dev`、Tauri `beforeDevCommand` 和 VS Code 桌面调试入口都把 Rust/Tauri 壳的启动与前端 dev server boot 绑在一起。结果是每次只想调试 `src-tauri` 时，仍然要重复穿过前端 ownership、sidecar prepare 和 GUI 启动这整条链，而且仓库里也没有一个受支持的 CLI 可以在“前端已就绪”的前提下单独拉起 Rust 桌面运行时。现在需要补齐这条独立调试路径，否则 Rust 侧启动、sidecar 监督和桌面故障排查仍然会持续被前端启动噪音掩盖。

## What Changes

- 新增受支持的 Rust 桌面独立调试模式：在前端 dev server 或已准备好的前端产物就绪时，允许开发者单独启动 `src-tauri` 这条 Rust 运行时链路，而不是每次都通过 `pnpm tauri:dev` 重新接管前端启动。
- 新增对应的 Rust 调试 CLI 工具，负责预检前端入口、当前主机 sidecar 产物、端口冲突与运行时前置条件，并在前台启动 Rust 运行时输出 ready/degraded 诊断信息。
- 修改桌面开发工作流契约，明确区分“完整桌面开发模式”和“Rust 独立调试模式”：前者继续通过 `tauri:dev`/Tauri pre-command 管理前端启动，后者复用同一套 sidecar prepare contract，但不再隐式启动新的前端 dev server。
- 对齐根级脚本、VS Code 桌面调试入口和开发文档，确保 CLI、IDE 与仓库文档对这两条桌面调试路径给出一致的命令、前置条件与失败诊断语义。

## Capabilities

### New Capabilities
- `desktop-runtime-debug-cli`: 定义用于独立调试 Rust/Tauri 运行时的 CLI 契约，覆盖前置检查、前台启动和开发者可读的诊断输出。

### Modified Capabilities
- `desktop-development-workflow`: 桌面开发工作流需要新增受支持的 Rust 独立调试入口，并把它与现有 `tauri:dev`/Tauri pre-command/IDE 调试路径的职责边界说明清楚。

## Impact

- Affected code: `src-tauri/Cargo.toml`, `src-tauri/src/main.rs`, `src-tauri/src/lib.rs`, new shared runtime-host or CLI files under `src-tauri/src/`, root `package.json`, `.vscode/tasks.json`, `.vscode/launch.json`, and any focused tests covering the desktop debug contract.
- Affected workflows: full desktop dev via `pnpm tauri:dev`, standalone Rust desktop debugging, current-host sidecar preparation, and maintained IDE launch tasks.
- Affected docs: `README.md`, `README_zh.md`, and developer-facing desktop debug guidance such as `docs/deployment/desktop-build.md` if it remains the maintained source of truth.
- Affected systems: local desktop runtime startup, sidecar readiness diagnostics, and developer debugging ergonomics around `src-tauri`.
