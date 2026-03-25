## Why

AgentForge 已经在 TypeScript Bridge 底层接入了 `claude_code`、`codex` 和 `opencode` 三类 coding-agent runtime，但产品层仍没有把它们收敛成一套可配置、可选择、可校验、可运营的完整 provider 能力。现在补齐这条链路，可以把 PRD 中“可插拔 Agent 运行时”的目标从底层实验接口推进到项目设置、启动入口、团队流水线和运行时诊断都一致可用的真实产品能力。

## What Changes

- 增加项目级 coding-agent provider 管理能力，定义 Claude Code、Codex、OpenCode 的 catalog、默认 runtime/provider/model、兼容关系、可用性状态和配置诊断入口。
- 在前端设置页与 Agent/Team 启动入口中暴露统一的 runtime/provider/model 选择，而不是继续把 Team 启动锁死在 `anthropic` + Claude 模型。
- 让 Team 流水线在 planner、coder、reviewer 全阶段继承同一组 provider/runtime 选择，并在摘要、事件和运行详情里保留真实 runtime/provider/model 信息。
- 收紧 Go→Bridge 的执行合同，消除按 `provider` 猜 `runtime` 的隐式回退，明确 Claude Code、Codex、OpenCode 的合法组合、默认值和错误语义。
- 补齐面向运维和开发者的文档与验证范围，包括所需环境变量、命令发现、缺失配置提示、以及各 runtime 的最小可运行约束。

## Capabilities

### New Capabilities
- `coding-agent-provider-management`: 定义项目级 coding-agent provider catalog、默认值、可用性诊断，以及前端/后端消费这一 catalog 的统一合同。

### Modified Capabilities
- `team-management`: 团队启动与团队摘要需要支持显式的 runtime/provider/model 选择，并在 planner/coder/reviewer 全链路保持一致而不丢失配置。
- `bridge-agent-runtime-registry`: 运行时注册表需要提供可用于上游 catalog/diagnostics 的兼容元数据与可用性判断，而不只是本地 execute 时临时解析。
- `agent-sdk-bridge-runtime`: 执行请求需要严格校验 runtime/provider/model 组合并拒绝含糊回退，确保 Claude Code、Codex、OpenCode 的运行语义一致且可诊断。

## Impact

- Affected frontend surfaces: `app/(dashboard)/settings/page.tsx`, `components/team/start-team-dialog.tsx`, 以及任何 Agent/Team 启动与摘要展示相关 store/component。
- Affected backend surfaces: `src-go/internal/model/project.go`, project/team handler/service/repository 链路，以及 `agent_runs` / `projects.settings` 的 DTO 和默认值语义。
- Affected bridge runtime surfaces: `src-bridge/src/runtime/registry.ts`, `src-bridge/src/handlers/execute.ts`, 可能新增 runtime catalog/diagnostics 模块与接口。
- Affected docs: `README.md`, `docs/PRD.md`, `docs/role-yaml.md`，以及与项目 settings / runtime 配置相关的运维说明。
