## Context

AgentForge 的 TypeScript Bridge 与 Go orchestration 现在已经具备运行时选择、provider 校验、Role YAML 管理、以及 Team planner/coder/reviewer 生命周期等多个基础能力，但这些能力之间的执行上下文合同还没有真正闭环。

当前代码里已经能看到这种“合同半成品”状态：
- `src-bridge` 的 request schema 和 runtime type 已经预留了 `team_id`、`team_role`、`role_config.tools`、`knowledge_context`、`output_filters` 等字段。
- `src-bridge` 的执行路径已经会消费这些字段的一部分，例如 role injector 会注入 `knowledge_context`，Claude runtime 会应用 `output_filters`，execute handler 会根据 `role_config.tools` 选择活跃插件。
- 但 `src-go/internal/bridge/client.go` 的 `ExecuteRequest` / `RoleConfig` 仍只发送最小 persona/budget/permission 字段；`AgentService.resolveRoleConfig(...)` 也只投影了最小 execution profile；Team 的 spawn / retry / resume 流程只在 run 持久化后再补 `team_id` / `team_role`，Bridge 本身拿不到这些上下文。

这使得当前系统虽然“能跑”，却还没有形成一条完整的、多 Agent 友好的、可恢复的执行上下文链：Bridge status/snapshot 无法稳定反映 Team 阶段身份，resume 路径无法保证与初次执行使用同一组上下文，高级 role 元数据到底哪些属于 runtime-facing contract 也没有明确边界。

## Goals / Non-Goals

**Goals:**
- 定义一条统一的 Go→Bridge execution context 合同，覆盖普通 agent run 与 Team planner/coder/reviewer run。
- 扩展 Go 侧 execution profile 投影，使 Bridge 真正收到它已经支持消费的 runtime-facing role metadata：工具插件选择、知识上下文、输出过滤器，以及现有 persona/guardrail 字段。
- 让 Team 相关上下文在 execute、status、snapshot、pause/resume、retry 等链路中保持一致，避免靠运行后数据库回填或日志猜测阶段身份。
- 保持 TS Bridge 为无状态执行器：Bridge 接收的是规范化后的执行上下文，而不是原始 Role YAML 或 Go 域对象。
- 为后续扩展更多 Team phase、delegation/handoff 语义、或更复杂的 role runtime contract 留出明确扩展点。

**Non-Goals:**
- 不在本次 change 中实现基于 role `collaboration` 的自动委派逻辑。
- 不在本次 change 中让 Bridge 直接执行 role `memory`、`triggers` 或其他 PRD 级高级策略。
- 不重做现有 runtime/provider catalog、设置页选择器或 Team UI 编排模型。
- 不修改 IM Bridge、workflow step router、或 review pipeline 的产品语义，除非它们依赖新的 execution context contract。

## Decisions

### 1. Introduce one Go-side execution-context builder for Bridge-bound runs
本次将把当前分散在 `AgentService.Spawn`、`resumeRun`、Team spawn/retry 路径中的 Bridge 请求构造逻辑收敛成一个统一的 execution-context builder，由它负责拼装 runtime/provider/model、expanded role profile、以及可选的 team context。

这样做的原因是：当前 execute/resume/team 多处各自拼接请求，最容易导致“某条路径忘传新字段”的漂移。与其在每个调用点零散补字段，不如让所有 Bridge-bound run 都走同一条上下文构造入口。

备选方案：继续在现有每个 spawn/resume 调用点逐个补字段。
不采用原因：会放大当前已经存在的 drift，后续任何 role/team contract 变更都需要多处同步，验证成本高且容易漏。

### 2. Keep the Bridge contract normalized and bounded instead of shipping raw Role YAML
Bridge 继续只接收 Go 侧投影后的 execution profile，而不是原始 Role YAML。此次扩展的 `role_config` 会包含当前 Bridge 真正消费的 runtime-facing 字段：`allowed_tools`、`tools`、`knowledge_context`、`output_filters`、budget/turn/permission/persona 等；但不会把完整 `metadata`、`knowledge.memory`、`collaboration`、`triggers` 原样下发给 Bridge。

这样做的原因是：Bridge 的职责是执行器，不应重新承担 YAML 解析、继承合并、策略裁剪、或 PRD 语义解释。Go 侧 role store / resolver 仍是 source of truth，Bridge 只消费稳定、可测试的执行合同。

备选方案：直接把完整 resolved role manifest 发给 Bridge。
不采用原因：会把 Go 的 role schema 与 Bridge runtime 过度耦合，未来任意 PRD 字段调整都可能破坏执行合同，也会让 Bridge 重新背上“理解 Role 域模型”的职责。

### 3. Make Team context explicit, validated, and continuity-safe
Team 上下文将成为显式的 Bridge request / runtime identity 一部分，而不是运行后再通过 `SetTeamFields(...)` 补到数据库。对 Team-managed run，Go 会在执行前传入 `team_id` 与规范化后的 `team_role`；Bridge schema 会把 `team_role` 收紧到已支持的 phase 集，并在状态/快照里保留这些字段。

这样做的原因是：Team planner/coder/reviewer 的阶段身份不仅用于数据库查询，也用于恢复、诊断、以及后续多 Agent 扩展。若 Bridge 从未拿到这些字段，status/snapshot/resume 永远只能依赖外部重建语义，无法形成真正一致的执行上下文。

备选方案：继续只在 run record 中落 `team_id` / `team_role`，Bridge 不感知 Team。
不采用原因：pause/resume、status、snapshot、以及未来桥接内的 phase-aware hooks 都会缺乏直接上下文，造成“存储里知道，运行时不知道”的双轨语义。

### 4. Treat unsupported advanced role metadata as stored-only, not silently dropped
本次会明确划分 advanced role metadata 的执行边界：
- `tools`、`knowledge_context`、`output_filters` 这类已被 Bridge 执行路径消费或天然属于 runtime contract 的字段，进入 execution profile。
- `collaboration`、`memory`、`triggers` 等当前没有 Bridge runtime consumer 的字段，继续保存在 normalized role model 中，但不进入 Bridge contract。

这样做的原因是：现状最危险的不是“字段暂时不执行”，而是“字段存在却没有清晰边界”，导致一部分被悄悄丢弃，一部分被错误期待会生效。把边界写清楚，才能既保证当前合同稳定，又为后续能力扩展留下清晰入口。

备选方案：把所有 advanced 字段都塞进 execution profile，为未来留接口。
不采用原因：这会制造假的“已支持”表象，也会把未定义行为直接带进 Bridge 请求与测试合同。

### 5. Roll out additively so Bridge-first deployment stays safe
本次 contract 扩展将保持“向前兼容、向后可回退”的部署方式：新增字段在 Bridge schema 中保持可选，但一旦出现就必须通过规范化校验；Go 侧随后开始稳定发送这些字段。状态与快照输出也采用 additive 扩展，不破坏旧消费者的基础行为。

这样做的原因是：Bridge 与 Go 是跨进程部署边界，任何一次性强制 required 变更都会给本地开发、Tauri sidecar、以及 dirty-tree 调试带来不必要风险。加法式 rollout 更适合这条基础设施合同。

备选方案：直接把新字段变成 hard required 并一次性切换所有路径。
不采用原因：会放大部署顺序依赖，也不利于 scoped verification 与分阶段回滚。

## Risks / Trade-offs

- [Risk] Bridge contract 扩大后，Go role schema 与 Bridge request shape 再次漂移。 -> Mitigation: 以统一 execution-context builder 和 focused tests 约束 execute/resume/team 路径都走同一份 contract。
- [Risk] Team 上下文同时存在于 run record、snapshot、status 中，出现多处 source of truth。 -> Mitigation: 规定 Go 组装请求时的 execution context 为启动真相，Bridge snapshot/status 仅反映该真相，不反向推导新的 team identity。
- [Risk] 未来 collaboration/delegation 能力会希望更多 role metadata 进入 Bridge。 -> Mitigation: 在 spec 中保留显式扩展位，只允许新增被定义为 runtime-facing 的字段，而不是重新开放 raw manifest 传输。
- [Risk] 老的非 Team 调用方仍依赖最小 execute payload。 -> Mitigation: 新字段保持 optional，对非 Team / 无 role 的普通 run 继续允许最小合同；但 Go 正式链路的发送逻辑升级为完整合同。

## Migration Plan

1. 先扩展 `src-bridge` 的 request schema、runtime identity、snapshot/status shape 与相应测试，使 Bridge 能接受并保留新增上下文，但仍兼容旧调用方。
2. 再扩展 `src-go/internal/bridge/client.go` 与 role execution profile builder，把 `tools`、`knowledge_context`、`output_filters`、`team_id`、`team_role` 等字段纳入正式 Bridge request。
3. 统一 AgentService / TeamService / resume 路径的 execution-context builder，让普通 spawn、Team planner/coder/reviewer、retry、pause/resume 都走相同合同。
4. 更新 run summary / diagnostics / 文档与 focused verification，确保 role YAML -> Go projection -> Bridge execute/resume/status/snapshot 链路可验证。
5. 回滚时按相反顺序处理：Go 可以先停止发送新增字段；Bridge 继续容忍缺失字段并按旧最小合同运行。因为本次设计以加法式 contract 扩展为主，不涉及必须的数据迁移。

## Open Questions

- 当前 proposal 阶段无额外 blocker。`team_role` 本次先限定为 `planner` / `coder` / `reviewer` 三类已存在 phase；若后续引入新的 Team phase 或 delegation role，再通过 `team-agent-context-handoff` capability 增量扩展。
