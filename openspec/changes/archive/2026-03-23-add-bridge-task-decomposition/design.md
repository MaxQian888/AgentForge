## Context

AgentForge 的 PRD 与技术拆解文档已经把“AI 分解任务”定义为主链路的一部分，并明确要求轻量 AI 调用统一经过 TypeScript Bridge，而不是由 Go 直接碰 LLM。当前代码中，`src-bridge` 只有 `/bridge/execute`、`/bridge/status/:id`、`/bridge/cancel` 等 Agent runtime 接口，`src-go` 的任务 handler/service 只有 CRUD、状态流转与指派，`src-im-bridge` 的 `/task` 仅支持 `create|list|status|assign`。同时，任务模型已经具备 `parent_id`、项目归属与创建任务仓储能力，说明“把分解结果持久化为子任务”所需的数据基础已存在，但编排链路还没有被打通。

这次变更是典型的跨模块能力补齐：需要在 Bridge 增加轻量 AI 接口，在 Go 层增加任务分解编排和持久化边界，在 IM Bridge 暴露用户入口，并把错误语义、请求结构和返回形态对齐成一条可验证的路径。

## Goals / Non-Goals

**Goals:**
- 为现有任务提供端到端的 AI 分解能力，入口覆盖 Go API 与 IM `/task decompose <id>`。
- 保持 TypeScript Bridge 作为唯一 AI 出口，Go 只负责业务编排、校验和持久化。
- 将分解结果持久化为父任务下的结构化子任务，并保证失败时不留下半成品。
- 让 API 与 IM 调用方拿到稳定、可读的结果摘要与错误信息。

**Non-Goals:**
- 不在本次变更中实现自然语言意图识别、自动派单、自动启动 Agent 执行。
- 不做任务分解历史、人工编辑工作流、few-shot 样本库等增强功能。
- 不在本次变更中重新设计任务优先级/预算体系，只做必要的输入归一化。
- 不复用 Agent runtime 的长会话、流式事件和 session 管理来承载轻量分解调用。

## Decisions

### 1. 在 `src-bridge` 增加独立的轻量分解接口，而不是复用 Agent runtime

Bridge 将新增独立的 decomposition request/response schema 与 `/bridge/decompose` 路由，由专门的轻量 AI handler/service 负责提示词组装、模型调用和结构化输出校验。它与 `RuntimePoolManager` 管理的 Agent 执行链路分离，只共享必要的错误包装与成本统计基础能力。

这样做的原因是任务分解本质上是一次性分析调用，不需要 worktree、session resume、streaming 事件和运行中 agent 状态。若复用 `/bridge/execute`，Go 需要伪造 agent 任务和上下文，既抬高实现复杂度，也会把“轻量 AI 调用”和“长生命周期 Agent 执行”混成一个抽象。

备选方案：用隐藏 role prompt 调 `/bridge/execute` 让 Agent 产出 subtasks。放弃原因是调用成本更高、时延更长，而且输出稳定性更难保证。

### 2. 由 `src-go` 的任务服务负责唯一编排与原子持久化

Go API 将新增 `POST /api/v1/tasks/:id/decompose`。TaskService 负责读取父任务、确认其可分解、调用 Bridge、校验/归一化分解结果，并在单次业务操作里创建所有子任务。子任务沿用现有 `tasks` 表，以 `parent_id` 关联父任务，继承父任务的 `project_id` 与 `sprint_id`，默认状态为 `inbox`。对 Bridge 返回的优先级进行白名单归一化，无法识别时回退到父任务优先级或 `medium`。

这样做可以把 LLM 输出与数据库写入之间的防线留在业务层：Go 既能阻止非法字段进入仓储，也能在失败时保证“不创建任何子任务”。仓储层需要补足“查询父任务是否已有子任务”和“批量创建子任务”这类接口，但不会承担 AI 调用职责。

备选方案：让 Bridge 直接返回可落库 payload，由 handler 逐条写入。放弃原因是业务约束会散落在 handler 中，且更难保证全有或全无。

### 3. API 语义保持同步返回，并显式防重复分解

`POST /api/v1/tasks/:id/decompose` 采用同步请求-响应：成功时返回父任务、创建出的子任务列表和摘要；失败时返回结构化错误。为了避免重复点击或重复命令制造两批子任务，服务端在分解前检查父任务是否已存在子任务；若存在则直接拒绝，并提示先人工清理或等待后续增强方案。

选择同步是因为当前链路没有任务队列或后台 job 语义，而分解本身是轻量 AI 调用，保持同步更容易与 Web/IM 两个入口共享同一业务语义。防重复策略先采用保守拒绝，比“静默追加一批 subtasks”更安全。

备选方案：异步入队，稍后推送结果。放弃原因是当前仓库没有可靠的任务分解 job 基础设施，会把这次 change 范围扩大到状态机和通知系统。

### 4. IM Bridge 只作为触发器和结果展示层

`src-im-bridge` 新增 `/task decompose <id>` 命令，调用 Go API 的分解接口，而不是直接访问 TypeScript Bridge。命令层先回一条“正在分解任务”的即时提示，再在调用完成后回显摘要和生成的子任务列表；当平台支持卡片时，优先使用卡片/富文本格式，否则退化为纯文本。

这样可以保持所有任务业务逻辑集中在 Go 后端，IM Bridge 只负责命令解析、调用 API 与用户提示，避免形成第二套业务编排逻辑。

备选方案：让 IM Bridge 直接调 `/bridge/decompose`。放弃原因是会绕开权限、项目归属和数据库持久化边界。

## Risks / Trade-offs

- [LLM 输出不稳定或字段不合法] → 使用 Bridge 侧 schema 校验 + Go 侧白名单归一化，无法修复时整体失败并返回可诊断错误。
- [重复分解导致父任务下出现多批重复 subtasks] → 在创建前检查现有子任务；已有子任务时返回冲突，避免静默追加。
- [Bridge/Go 现有接口命名不一致导致实现漂移] → 在本次变更中明确 `/bridge/*` 为 Bridge 路由约定，并为 Go bridge client 增加对应方法而不是继续沿用旧 `/api/*` 路径。
- [IM 调用时延较长，用户误以为命令没有生效] → 先发送即时反馈，再展示最终结果；若后续发现体验不足，再单独提异步通知增强变更。
- [任务优先级与现有 schema/数据库枚举存在不一致风险] → 分解结果进入持久化前统一做优先级映射与回退，不把模型原始值直接写入数据库。

## Migration Plan

1. 先部署 `src-bridge` 的 `/bridge/decompose` 与 schema 校验能力，并确认健康检查不受影响。
2. 部署 `src-go` 的任务分解服务、仓储补充接口与 `POST /api/v1/tasks/:id/decompose` 路由。
3. 部署 `src-im-bridge` 的 `/task decompose <id>` 命令，使 IM 入口开始复用新的 Go API。
4. 回滚时按相反顺序移除入口；若 Go API 已回滚，IM 命令也必须同时回滚或降级为“暂未开放”。由于本次不涉及存量数据迁移，已创建子任务保留即可。

## Open Questions

- Bridge 侧最终采用哪一个具体模型与提示词模板属于实现细节，只要满足结构化输出契约即可。
- IM 平台最终展示为卡片还是纯文本取决于现有平台能力，本次 spec 只要求可读摘要和子任务列表可见。