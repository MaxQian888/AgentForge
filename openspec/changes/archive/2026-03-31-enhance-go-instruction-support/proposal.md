# Enhance Go Instruction Support

## Why

`enhance-go-instruction-support` 这条 change 的原始 artifacts 仍在描述一条很长的未来路线图，但仓库里实际上已经落地了一批明确可用的 Go 侧能力，包括：

- in-process instruction router
- canonical role manifest parsing, inheritance, preview, sandbox, and runtime projection
- bridge websocket transport plus Go-side runtime event projection
- short-term memory and repository-backed project memory service
- Go-managed WASM plugin runtime
- team startup/runtime propagation
- workflow plugin execution and step routing
- task-level runtime budget tracking

当前问题不是“缺少一份宏大规划”，而是这些已实现能力没有被 OpenSpec artifacts 准确描述，导致 proposal / design / specs / tasks 之间互相冲突，也让后续 apply / archive 很难基于 change 真相推进。

## What Changes

本次 change 收敛为“对齐现有实现的 Go instruction support contract”，覆盖以下已实现能力面：

- `instruction-router`: 注册式 routing、validator、timeout、queue、history、metrics、cancel
- `role-management`: advanced manifest schema、canonical layout、effective merge、preview / sandbox、runtime projection
- `bridge-events`: TS bridge websocket transport、ready / heartbeat / buffering，以及 Go 当前已消费的 output / cost / terminal status events
- `memory-system`: short-term memory、repository-backed project memory、prompt injection、team learnings persistence
- `plugin-runtime`: Go-managed WASM runtime activation / health / invoke / restart / capability gating
- `agent-teams`: strategy-based team creation、runtime selection persistence、team-context-aware spawning
- `workflow-engine`: workflow plugin execution、supported process modes、step router actions、retry / pause semantics
- `resource-governor`: task-level runtime budget tracking、warning threshold、budget exhaustion stop
- `security-policies`: declarative role security schema、stricter inheritance merge、runtime-facing projection
- `knowledge-index`: declarative knowledge references plus runtime knowledge-context projection

## Capabilities

### Modified Capabilities

- `instruction-router`
- `role-management`
- `bridge-events`
- `memory-system`
- `plugin-runtime`
- `agent-teams`
- `workflow-engine`
- `resource-governor`
- `security-policies`
- `knowledge-index`

## Impact

### Affected Artifacts

- `openspec/changes/enhance-go-instruction-support/proposal.md`
- `openspec/changes/enhance-go-instruction-support/design.md`
- `openspec/changes/enhance-go-instruction-support/specs/**/*.md`
- `openspec/changes/enhance-go-instruction-support/tasks.md`

### Runtime Truth Sources

- `src-go/internal/instruction/*`
- `src-go/internal/role/*`
- `src-go/internal/handler/role_handler.go`
- `src-go/internal/service/{agent_service,memory_service,team_service,workflow_execution_service}.go`
- `src-go/internal/plugin/runtime.go`
- `src-go/internal/ws/bridge_handler.go`
- `src-bridge/src/ws/event-stream.ts`

### Non-Goals

- 不在本次 change 中承诺尚未落地的 semantic search / knowledge graph / plugin hot reload / Redis 全局资源治理 / 安全 enforcement middleware / 完整 team message bus。
- 不在本次 change 中新增运行时代码；这次以 contract 对齐和 artifact 收敛为主。
