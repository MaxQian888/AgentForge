## Why

AgentForge 已经把 IM Bridge 作为真实运行栈的一部分暴露在仓库结构、README 和桌面能力里，但当前仓库真相仍然是不完整的：`dev:all` 不会启动 IM Bridge，`tauri:dev` / `build:desktop` 也不会构建或打包 IM Bridge sidecar，VS Code 的桌面调试前置同样只覆盖前端。现在需要把这条启动与调试链路补齐，否则本地联调、桌面调试和最终桌面产物都会继续缺失 IM Bridge 这一段关键功能。

## What Changes

- 扩展根级全栈启动命令族，使 `pnpm dev:all` 及其配套的 `status` / `stop` / `logs` 将 IM Bridge 作为受支持的本地开发服务一起启动、复用、探测健康并落盘日志，而不是继续要求开发者额外手工开一个 `src-im-bridge` 终端。
- 新增桌面开发工作流契约，定义 `pnpm tauri:dev`、`pnpm build:desktop`、Tauri `beforeDevCommand` / `beforeBuildCommand`、以及 VS Code 桌面调试前置任务必须以同一套完整拓扑准备 frontend、Go Orchestrator、TS Bridge 与 IM Bridge。
- 修改桌面 sidecar 拓扑与监督行为，使 Rust/Tauri 壳层把 IM Bridge 视为必需 sidecar，与 backend、TS bridge 一样被构建、打包、启动、健康检查、失败恢复和降级上报，而不是停留在当前只管理两条 sidecar 的状态。
- 对齐 README、README_zh、脚本输出、运行时日志与调试入口描述，明确 IM Bridge 的端口、健康端点、依赖关系和失败诊断语义，确保文档承诺与仓库行为一致。

## Capabilities

### New Capabilities
- `desktop-development-workflow`: 定义仓库级桌面开发与调试命令契约，确保 `tauri:dev`、`build:desktop`、Tauri pre-command 与 VS Code 桌面调试入口都基于完整 sidecar 拓扑工作。

### Modified Capabilities
- `local-development-workflow`: 全栈本地启动工作流需要把 IM Bridge 纳入受支持的统一启动、状态、停止与诊断范围，而不是只覆盖 frontend、Go、TS Bridge 和 infra。
- `desktop-sidecar-supervision`: 桌面壳层的必需 sidecar 拓扑需要从 “Go Orchestrator + TS Bridge” 扩展为包含 IM Bridge 的完整运行时，并相应更新 ready/degraded 语义。
- `desktop-runtime-event-bridge`: 桌面运行时快照与事件桥需要把 IM Bridge 暴露为独立受管 runtime，而不是继续只投影 backend 与 bridge 两个运行单元。

## Impact

- Affected code: root `package.json`, `scripts/dev-all.js`, `scripts/dev-all.test.ts`, new or updated IM Bridge build helpers under `scripts/`, `.vscode/launch.json`, `.vscode/tasks.json`, `src-tauri/src/lib.rs`, `src-tauri/tauri.conf.json`, and related Rust/runtime logic tests.
- Affected workspaces: `src-go`, `src-bridge`, `src-im-bridge`, root frontend dev server, and repo-local runtime state/log handling under `.codex/`.
- Affected docs: `README.md`, `README_zh.md`, and any developer-facing startup/debug guidance that currently omits IM Bridge from the supported command path.
- Affected systems: local web debug stack, desktop debug/build pipeline, Tauri sidecar packaging, and desktop runtime readiness/degraded reporting.
