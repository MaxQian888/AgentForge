## Why

The backend now has a real agent spawn path and bridge runtime, but `src-go/internal/worktree` still only wraps bare `git worktree add/remove/list` calls. The PRD expects richer WorkTree behavior around canonical task paths, capacity guardrails, safe cleanup, and stale-state recovery, so tightening this seam now prevents orphaned worktrees, branch drift, and fragile retries as agent usage expands.

## What Changes

- Add a lifecycle-oriented WorkTree capability that standardizes canonical task workspace paths, deterministic agent branch ownership, active-capacity checks, and safe cleanup/prune behavior.
- Extend the existing agent spawn orchestration contract so runtime startup acquires worktrees through the managed lifecycle API and fails cleanly when allocation is unsafe.
- Add focused verification around create/reuse/remove/conflict/cleanup flows so backend runtime state stays aligned with Git state.

## Capabilities

### New Capabilities
- `agent-worktree-lifecycle`: Manage per-task worktree allocation, inspection, cleanup, and stale-state recovery guardrails for agent execution workspaces.

### Modified Capabilities
- `agent-spawn-orchestration`: Spawn requests must provision or reuse task worktrees through the managed lifecycle contract and keep run/task metadata clean when worktree allocation is denied.

## Impact

- Affected code: `src-go/internal/worktree/*`, `src-go/internal/service/agent_service.go`, agent route error handling, server composition/config wiring, and related Go tests.
- Affected APIs: `POST /api/v1/agents/spawn` error semantics for worktree capacity/conflict failures; no request-payload changes are planned.
- Affected systems: Git worktree metadata under `WORKTREE_BASE_PATH` / `REPO_BASE_PATH`, runtime cleanup paths, and operator recovery flows for leaked worktrees.
