## Why

AgentForge 现在已经有 `dev:backend` 启动/状态/日志能力，也有 `src-go`、`src-bridge`、`src-im-bridge` 各自的 focused tests，但仓库真相仍然缺一条“把整个后端真正跑通”的官方 backend-only 验证路径。`TESTING.md` 目前明确说明没有 single-command test surface，IM stub smoke 也主要停留在 `src-im-bridge` 下的 PowerShell 手工步骤；当训练员说“完整地跑通整个后端，包括 TS 和 IM Bridge”时，仓库还没有一个零外部凭据、可复用、能指出断点所在 hop 的权威 smoke workflow。

## What Changes

- 新增一条 root-level backend smoke workflow，提供受支持的 `dev:backend` 联调验收命令，负责启动或复用 Go Orchestrator、TS Bridge、IM Bridge 与本地 infra，然后按固定 stage 证明后端整栈可用。
- 把现有的 backend health、Bridge proxy、IM stub smoke seam 收束成同一条 repo-supported runbook：至少覆盖 Go `/health`、TS `/bridge/health`、IM `/im/health`，以及一条通过 IM stub 触发的 Bridge-backed 命令链路。
- 新增 cross-platform 的 smoke runner / helper，复用 `src-im-bridge` 现有 stub test endpoints 与 fixtures，避免“后端跑通”只能依赖 PowerShell 手工脚本。
- 统一失败输出与诊断语义：新的 backend smoke workflow 必须说明失败发生在 startup、Go health、TS Bridge proxy、IM Bridge stub intake，还是 IM command reply stage，并直接给出对应日志或状态位置。
- 更新 README / README_zh / TESTING 等 source of truth，让 backend-only 启动、验证、日志排查和零凭据 stub 约束对齐，不再让 `dev:backend`、IM smoke、Bridge health 各说各话。

## Capabilities

### New Capabilities
- `backend-runtime-smoke-workflow`: 定义 root-level backend-only smoke verification contract，覆盖 Go、TS Bridge、IM Bridge 的启动复用、health checks、stub-backed command flow 和 stage-based diagnostics。

### Modified Capabilities
- `local-development-workflow`: 增加受支持的 backend verify / smoke 命令语义，要求其复用现有 managed stack、state、logs，而不是另起一套平行启动流程。
- `backend-bridge-connectivity`: 增加“后端整栈跑通”所需的最小 smoke proof，要求验证至少命中一条 IM -> Go -> TS Bridge 的 canonical flow，并在失败时指出具体断点 hop。

## Impact

- **Root scripts / commands**: `package.json`、`scripts/dev-all.js`，以及新的 backend smoke runner / helper。
- **IM stub smoke assets**: `src-im-bridge/scripts/smoke/*` 与相关 fixtures / docs，需要从 PowerShell-only 手工步骤收束成 repo-supported workflow。
- **Documentation / verification truth**: `README.md`、`README_zh.md`、`TESTING.md` 及相关开发指引。
- **Cross-stack diagnostics**: backend smoke 输出需要指向 Go / TS Bridge / IM Bridge 的 health endpoints、runtime state 和 repo-local logs。
