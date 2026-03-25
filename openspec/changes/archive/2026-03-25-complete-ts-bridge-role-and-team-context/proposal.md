## Why

AgentForge 现在已经有可运行的 TypeScript Bridge、runtime/provider registry，以及 Team 的 planner/coder/reviewer 流程，但 Go→Bridge 的正式执行合同还没有把这些能力收敛成一条完整的上下文链。当前 `src-bridge` schema 已预留 `team_id`、`team_role`、`role_config.tools`、`knowledge_context`、`output_filters` 等字段，而 `src-go/internal/bridge/client.go`、`AgentService.resolveRoleConfig(...)`、Team spawn/resume 路径仍没有稳定传递这些上下文，导致多 Agent 阶段身份、角色知识/过滤约束、以及恢复链路里的执行语义无法端到端对齐。

现在补齐这条能力，可以把“TS Bridge 是所有后端 AI 调用统一出口”的架构目标推进到真正可扩展、可协作、可恢复的产品契约，避免后续继续在 Team orchestration、Role YAML、Bridge runtime 之间长出各自独立的兼容逻辑。

## What Changes

- 为 Go→TypeScript Bridge 增加完整的 agent execution context 合同，覆盖 Team 级 `team_id` / `team_role`、恢复时保持一致的上下文、以及桥接状态/快照中可回显的阶段身份。
- 扩展 Go 侧 role execution profile 投影，使 Bridge 真正拿到当前 schema 已支持的 runtime-facing role metadata，包括工具插件选择、知识上下文和输出过滤器，而不是只传最小 persona 字段。
- 收敛 Team planner/coder/reviewer 启动、重试、排队提升、暂停/恢复链路里的上下文传播规则，确保多 Agent 阶段在 Bridge 侧拥有一致且可诊断的身份与约束。
- 明确高级 Role YAML 元数据的执行边界：哪些字段进入 Bridge 执行合同，哪些仍保留在 Go 侧/存储侧，避免把 collaboration、memory、triggers 等字段再次隐式丢弃或误当作运行时配置。
- 补齐 focused verification 与文档，覆盖 role YAML -> Go execution profile -> Bridge execute/resume/status/snapshot -> Team lifecycle 的全链路验证范围。

## Capabilities

### New Capabilities
- `team-agent-context-handoff`: 定义 Team 多 Agent 执行时 Go 与 TypeScript Bridge 之间的上下文传递契约，包括 team identity、planner/coder/reviewer 阶段身份、恢复链路一致性，以及对后续多 Agent 扩展友好的上下文边界。

### Modified Capabilities
- `agent-sdk-bridge-runtime`: 执行、恢复、状态与快照合同需要接受并保留更完整的 role/team execution context，而不仅是最小 runtime tuple 与基础 role persona。
- `agent-spawn-orchestration`: spawn、retry、queued-promotion 等后端启动路径需要在 Bridge 启动前写入并传播稳定的 role/team context，避免 Team 运行与普通 agent run 出现语义漂移。
- `role-plugin-support`: Role manifest 到 execution profile 的投影规则需要扩展为包含 Bridge 真正消费的 tools、knowledge context、output filters 等 runtime-facing 字段，并明确 advanced metadata 的支持边界。

## Impact

- Affected Go backend: `src-go/internal/bridge/client.go`, `src-go/internal/service/agent_service.go`, `src-go/internal/service/team_service.go`, 相关 DTO/model/repository/test 文件。
- Affected Bridge runtime: `src-bridge/src/types.ts`, `src-bridge/src/schemas.ts`, `src-bridge/src/handlers/execute.ts`, `src-bridge/src/handlers/claude-runtime.ts`, session/status snapshot surfaces，以及相关测试。
- Affected role execution seams: `src-go/internal/role/*`, `docs/role-yaml.md`, Team/Agent 启动与摘要链路。
- Affected verification/doc surfaces: focused Go tests、Bridge tests、必要的 Team lifecycle contract docs 与 OpenSpec delta specs。
