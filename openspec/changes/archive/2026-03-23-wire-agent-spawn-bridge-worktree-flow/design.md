## Context

The current Go server already contains most of the pieces for agent execution, but they are not composed into a working runtime path. `src-go/cmd/server/main.go` wires repositories directly into `server.RegisterRoutes`, `src-go/internal/handler/agent_handler.go` inserts an `agent_runs` record without invoking any service, and the active WebSocket route uses `internal/handler/ws.go`, which is a token-validated echo endpoint instead of the hub-backed broadcaster in `src-go/internal/ws/handler.go`.

At the same time, config already defines `BRIDGE_URL`, `WORKTREE_BASE_PATH`, `REPO_BASE_PATH`, and `MAX_ACTIVE_AGENTS`; `internal/bridge/client.go`, `internal/worktree/manager.go`, `internal/service/agent_service.go`, and `internal/ws/hub.go` exist but are unused by the server composition root. The change therefore needs a cross-cutting design that turns those dormant pieces into a coherent spawn pipeline without widening scope into the full orchestration roadmap described in the docs.

## Goals / Non-Goals

**Goals:**
- Make `POST /api/v1/agents/spawn` execute a real runtime startup flow instead of only creating a database row.
- Wire the Go server composition root so bridge, worktree, agent service, and WebSocket broadcasting are instantiated once and passed through explicit dependencies.
- Persist runtime identifiers on the related task (`agent_branch`, `agent_worktree`, `agent_session_id`) and keep run/task status consistent across success and failure paths.
- Deliver `agent.started`, `agent.failed`, and follow-up lifecycle events through the authenticated WebSocket hub that the backend already defines.
- Add focused tests that cover DI wiring, spawn success, and startup failure cleanup.

**Non-Goals:**
- Building the full multi-agent scheduler, review agent flow, or background monitor loop described in long-form architecture docs.
- Redesigning every repository-backed handler to use services in this change; the refactor is limited to the agent and WebSocket path needed for spawn.
- Changing the public REST payload shape for agent spawn beyond what is necessary to support the existing inputs.
- Implementing token-by-token bridge event ingestion from the TypeScript service; this change only makes the initial spawn path executable and routable.

## Decisions

### 1. `main.go` becomes the runtime composition root for agent execution
The server startup path will instantiate a single `ws.Hub`, start its `Run()` loop, construct the bridge HTTP client from `cfg.BridgeURL`, construct the worktree manager from `cfg.WorktreeBasePath` and `cfg.RepoBasePath`, and build an `AgentService` that receives all collaborators explicitly. `server.RegisterRoutes` will be updated so the agent route receives the service dependency it actually needs, and `/ws` will switch to the hub-backed handler from `internal/ws`.

This is preferable to letting handlers construct their own clients because composition stays in one place, startup configuration stays testable, and runtime collaborators such as the hub are shared consistently across the process.

Alternatives considered:
- Keep constructing dependencies inside handlers: rejected because it hides config usage, duplicates clients, and makes cleanup/testing brittle.
- Introduce a global service locator: rejected because it weakens type safety and makes startup order implicit.

### 2. `AgentService` owns spawn orchestration instead of the HTTP handler
The spawn handler will remain responsible for request validation and HTTP error mapping, but the orchestration steps move into `AgentService`. The service will:
- guard against duplicate active runs for a task;
- create the run record in `starting` state;
- derive a deterministic branch name for the task and create the worktree;
- build the bridge execute request from task/member/runtime inputs;
- call the TypeScript bridge and capture the returned session ID;
- update task runtime metadata and promote the run to `running` only after bridge startup succeeds;
- emit lifecycle events through the hub;
- compensate on partial failure by marking the run failed and removing any newly created worktree.

Keeping this logic in the service minimizes route churn and aligns with the existing service-layer intent already present in `internal/service/agent_service.go`.

Alternatives considered:
- Create a brand new `AgentRuntimeOrchestrator`: viable, but unnecessary for this scope because `AgentService` already owns agent-run status and event semantics.
- Leave orchestration in the handler: rejected because HTTP concerns and runtime side effects would stay tangled and hard to test.

### 3. Task runtime metadata gets explicit repository operations
The task table already has `agent_branch`, `agent_worktree`, and `agent_session_id`, but the repository layer has no targeted write path for them. The task repository contract will gain an explicit method for updating runtime metadata during successful spawn, plus a clear-on-failure behavior when startup aborts before the bridge returns a usable session.

This keeps SQL for task runtime fields in the repository layer instead of leaking raw queries into services and makes the success/failure contract easy to test.

Alternatives considered:
- Reuse generic task update APIs: rejected because the current update request model does not cover runtime fields and would blur user-editable fields with system-managed execution state.
- Write raw SQL directly from `AgentService`: rejected because it bypasses repository boundaries and makes later reuse harder.

### 4. WebSocket delivery switches to the existing hub-based implementation
`src-go/internal/ws/handler.go` already provides authenticated registration into `ws.Hub`, while `src-go/internal/handler/ws.go` only echoes messages back to the sender. The `/ws` route will be rewired to use the hub-backed handler so agent lifecycle broadcasts become observable by clients. Existing tests that assert echo behavior will be rewritten to assert authentication plus server-push delivery instead.

Alternatives considered:
- Keep both WebSocket handlers: rejected because two code paths for the same route would create ambiguous behavior and duplicated auth logic.
- Delay WebSocket rewiring to a later change: rejected because agent lifecycle events would still be emitted into an unused hub.

## Risks / Trade-offs

- [Multi-step spawn can fail halfway through] -> The service must apply compensating actions in a fixed order: update run status to failed, clear task runtime metadata if it was written, and remove the worktree if it was created.
- [Switching `/ws` away from echo behavior will break current tests] -> Update tests to reflect the product intent of authenticated server-push events, not the placeholder echo implementation.
- [Bridge availability now matters to spawn correctness] -> The server may still boot in degraded mode, but spawn requests must fail fast with clear errors when the bridge is unreachable.
- [Adding service dependencies to route registration increases constructor churn] -> Keep the refactor scoped to the agent/WebSocket path and avoid opportunistic rewrites of unrelated handlers.

## Migration Plan

1. Extend server composition in `main.go` to create the hub, bridge client, worktree manager, and service dependencies.
2. Refactor route registration and agent/WebSocket handlers to consume those dependencies.
3. Add task runtime metadata persistence and spawn compensation behavior in the service/repository layer.
4. Update tests for route wiring, WebSocket delivery, and spawn success/failure paths.

Rollback strategy: revert the route wiring to the repository-backed agent handler and the legacy WebSocket echo handler, then remove the new service/repository methods introduced for spawn orchestration.

## Open Questions

- Prompt construction for the bridge is only partially defined by current models; the initial implementation will likely compose task title and description plus caller-selected provider/model until a richer role-config flow lands.
- The first successful `Execute` response is the proposed boundary for promoting a run from `starting` to `running`; if the bridge later requires an explicit async acknowledgement event, this transition point may need to move.
