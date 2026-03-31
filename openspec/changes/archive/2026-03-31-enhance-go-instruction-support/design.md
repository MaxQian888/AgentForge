# Design: Enhance Go Instruction Support

## Context

仓库里已经存在一批 Go 侧 instruction support 相关实现，但这条 change 的 artifacts 仍在描述一条更大、更多未来承诺的路线图。结果是：

- specs 里混有“已经落地的合同”和“尚未实现的愿景”
- tasks 进度与代码真相脱节
- proposal / design 对外传达的是另一套范围

这次设计的目标不是扩写未来计划，而是把 change 收敛为“当前实现真相”。

当前仓库里已可证明的能力包括：

- `src-go/internal/instruction`: 注册式 instruction router，支持 target routing、validator、timeout、priority queue、dependency、cancel、history、metrics、panic recovery
- `src-go/internal/role` + `src-go/internal/handler/role_handler.go`: advanced role manifest、canonical `roles/<id>/role.yaml`、legacy fallback、inheritance merge、preview / sandbox、execution profile projection、skill diagnostics
- `src-go/internal/ws/bridge_handler.go` + `src-go/internal/service/agent_service.go` + `src-bridge/src/ws/event-stream.ts`: internal bridge websocket transport、ready / heartbeat / buffering，以及 Go 当前消费的 runtime output / cost / terminal status events
- `src-go/internal/memory` + `src-go/internal/service/memory_service.go`: short-term memory、project memory store/search/delete、prompt injection、team learnings persistence
- `src-go/internal/plugin/runtime.go`: wazero + WASI backed WASM runtime manager，支持 activate / health / invoke / restart / capability gating
- `src-go/internal/service/team_service.go`: strategy-based team creation、runtime config persistence、team-context-aware spawn
- `src-go/internal/service/workflow_execution_service.go` + `workflow_step_router.go`: workflow plugin execution、supported process modes、retry、approval pause、step actions
- `src-go/internal/cost/tracker.go` + runtime handlers: task-level budget warning / hard-stop
- role security / knowledge sections: parser + inheritance + runtime projection contracts

## Goals / Non-Goals

**Goals**

1. 让 proposal / design / specs / tasks 全部围绕当前已实现能力说同一套话。
2. 把“已实现合同”与“未来愿景”明确切开，避免 spec 继续过度承诺。
3. 保留 capability 名称连续性，但把 requirement 收敛到真实代码路径。

**Non-Goals**

1. 不在本次中新增 Go/TS runtime 功能。
2. 不把尚未落地的 knowledge indexing、semantic memory、global quota governance、security middleware 强行写成已支持。
3. 不拆成多个新 change；继续沿当前 active change 收口。

## Decisions

### 1. Change scope 改为“当前实现合同对齐”，不再保留 14 周路线图

当前 artifacts 的核心问题不是缺少更多规划，而是它们已经偏离代码。当前 change 改为记录并约束现有实现：

- 已落地行为写成 requirement / scenario
- 尚未落地行为从强承诺中移除
- 后续未来能力另开新 change

### 2. Instruction router 以 in-process contract 为准

`instruction-router` capability 只承诺当前 router 已具备的行为：

- definition registration
- local / bridge / plugin target reporting
- request normalization
- validator / timeout / cancellation
- queue priority and dependency semantics
- in-process history / metrics / pending introspection

不再承诺尚未存在的 HTTP introspection API。

### 3. Role / security / knowledge contracts 统一以 Go role stack 为真相源

角色相关 capability 全部以 Go role stack 的真实行为为边界：

- parser / store / registry / handler 为 canonical truth
- `preview` / `sandbox` 返回 normalized/effective manifest
- security 和 knowledge 当前属于 declarative schema + merge + runtime projection
- 不额外承诺全局 enforcement middleware 或实时 index engine

### 4. Bridge events 只承诺当前 transport 与 Go 已消费事件

bridge-events capability 现在只约束：

- TS bridge websocket ready / reconnect / buffering / heartbeat transport
- Go 侧 websocket intake
- Go 当前已处理的 `output` / `cost_update` / terminal `status_change`
- 其他先进 event types 只保证 transport shape preserved，不保证 Go 侧 orchestration 语义

### 5. Memory / team / workflow / plugin 以当前 service-level seams 为边界

这些 capability 统一按当前 service-level seams 写：

- memory: short-term memory + repository-backed project memory service
- teams: strategy-based startup and team-context propagation
- workflow: plugin-driven execution, supported process modes, step router actions
- plugin runtime: current Go WASM runtime manager, not future registry / hot reload platform

### 6. Resource governor 收敛到 task-level runtime budget governance

当前真实预算治理主要发生在：

- runtime handlers emit warning / hard-stop behavior
- Go updates persisted run/task spend
- task-level threshold and exhaustion handling

因此 resource-governor 不再承诺 Redis-backed monthly quotas、fair scheduling、quota APIs。

## Risks / Trade-offs

- 收窄 specs 会让这条 change 不再承担“未来平台规划”角色。代价是后续若要推进未实现能力，需要新 change。
- 但这样做换来的是 artifact truthfulness：apply / review / archive 都可以基于当前代码真相继续推进，而不是被愿景任务拖住。

## Migration Plan

1. 重写 capability specs，使其仅覆盖当前实现行为。
2. 重写 proposal / design，把 change 目标改为 contract alignment。
3. 重写 tasks，使任务列表只保留当前 change 已完成的真实工作项。
4. 用 `openspec validate` 验证 change 结构仍然有效。

## Open Questions

1. 尚未落地的未来能力是否需要拆成一条新的 roadmap change，而不是继续挂在这条 contract-alignment change 下。
