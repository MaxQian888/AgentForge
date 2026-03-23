## Why

The backend already exposes `/api/v1/agents/spawn` and includes bridge, worktree, and agent service building blocks, but the runtime path is still disconnected. `main.go` only wires repositories into route handlers, so a spawn request can create an `agent_runs` row but cannot allocate a worktree, call the TypeScript bridge, persist session metadata, or broadcast the real lifecycle.

## What Changes

- Wire agent runtime dependencies in `src-go/cmd/server/main.go`, including the WebSocket hub, bridge client, worktree manager, and agent-oriented services.
- Replace the repository-only agent spawn path with a service-driven orchestration flow that creates a worktree, starts bridge execution, and records agent runtime metadata on the related task and run.
- Define failure handling and cleanup behavior for partial startup failures so the backend does not leave stale `starting` runs or orphaned worktrees behind.
- Add targeted tests for the new dependency wiring and spawn orchestration behavior.

## Capabilities

### New Capabilities
- `agent-spawn-orchestration`: Spawning an agent provisions an isolated worktree, starts bridge execution, stores branch/worktree/session identifiers, and emits lifecycle updates that match the actual runtime state.

### Modified Capabilities

## Impact

- Affected code: `src-go/cmd/server/main.go`, `src-go/internal/server/routes.go`, `src-go/internal/handler/agent_handler.go`, `src-go/internal/service/agent_service.go`, `src-go/internal/bridge/client.go`, `src-go/internal/worktree/manager.go`, task persistence and WebSocket event plumbing.
- Affected APIs: `POST /api/v1/agents/spawn`, agent status/cancel flows, task metadata updates for `agent_branch`, `agent_worktree`, and `agent_session_id`.
- Affected configuration: `BRIDGE_URL`, `WORKTREE_BASE_PATH`, `REPO_BASE_PATH`, and related startup wiring assumptions in the Go server.
