## Why

AgentForge 的产品文档已经把本地开发和桌面侧联调描述成一条完整的工程体验，但当前仓库真相仍然是分散的手工步骤：前端、Go Orchestrator、TS Bridge、PostgreSQL/Redis 需要分别启动，缺少类似 `pnpm dev:all` 的统一入口，也没有成体系的状态、停止、日志与重复启动保护。现在需要把这条开发闭环补成一个独立 change，避免 README、PRD、插件设计文档与真实可执行流程继续漂移。

## What Changes

- 为仓库增加统一的本地全栈开发工作流契约，覆盖前端、Go Orchestrator、TS Bridge 与本地基础设施的启动、复用、健康检查和就绪输出。
- 为根级开发命令补齐与 `dev:all` 配套的 status、stop、logs/诊断语义，确保开发者可以查看当前运行态而不是重复启动孤儿进程。
- 将现有 `plugin:dev`、`.vscode/tasks.json` 端口守卫、`.codex/runtime-logs` 约定和手工 `docker compose`/`go run`/`bun run dev` 流程收敛为统一的 repo-truthful 开发脚本模式。
- 为本地开发工作流定义前置依赖检查、端口占用判断、健康探测和失败提示标准，明确区分“可复用已有服务”“可自动拉起的缺口”和“需要人工处理的阻塞”。
- 更新 README 与相关开发文档，使仓库对外承诺的启动路径、环境要求、日志位置和健康检查端点与脚本行为保持一致。

## Capabilities

### New Capabilities
- `local-development-workflow`: 定义 AgentForge 根级全栈本地开发命令的启动、复用、停止、状态与诊断行为。

### Modified Capabilities

## Impact

- Affected code: root `package.json`, `scripts/*.js`, `scripts/*.test.ts`, `.vscode/tasks.json`, potential shared dev-workflow helpers, and repo-local runtime metadata/log handling under `.codex/`
- Affected docs: `README.md`, `README_zh.md`, and any developer-facing setup/run guidance that currently prescribes manual multi-terminal startup
- Affected systems: frontend dev server, Go backend, TS bridge, local Docker-backed PostgreSQL/Redis, and the repo-local runtime logs / pid tracking convention
- No product API break is intended; the change is focused on developer workflow completeness and operational consistency
