## Context

`docs/PRD.md` and the long-form architecture docs treat WorkTree management as a first-class runtime subsystem: each agent task gets a canonical isolated workspace, capacity is bounded, stale worktrees can be detected and cleaned up, and retries should not create duplicate checkouts. The current code path is much narrower. `src-go/internal/worktree/manager.go` only shells out to `git worktree add/remove/list`, `AgentService` still derives branch names itself, and `MAX_ACTIVE_AGENTS` is not part of worktree admission control.

The recently completed spawn/runtime changes mean WorkTree state is now on the critical path for real backend execution instead of being dormant scaffolding. Improving this seam needs to stay focused on lifecycle support around the existing Go backend and current `agent/<task-id>` branch convention, rather than expanding into the full PRD roadmap for disk telemetry, multi-agent scheduling, or session-resume orchestration.

## Goals / Non-Goals

**Goals:**
- Introduce a lifecycle-aware WorkTree manager contract that owns canonical task path/branch derivation, reuse detection, and cleanup semantics.
- Enforce active-capacity and path-ownership guardrails before a spawn can proceed into bridge execution.
- Make cleanup idempotent so failed/cancelled runs and operator recovery flows can remove stale worktrees without manual Git surgery.
- Expose enough inspection or garbage-collection surface to support stale-worktree recovery and focused automated tests.
- Keep spawn/task runtime state aligned with Git state when worktree creation is rejected, reused, or released.

**Non-Goals:**
- Implementing the full background cron scheduler, disk-usage telemetry, or Prometheus metrics described in the PRD.
- Changing the public spawn request payload shape or introducing new database tables just for worktree tracking.
- Reworking the TypeScript bridge contract or session-resume behavior beyond the worktree inputs it already accepts.
- Replacing Git CLI usage with a pure `go-git` implementation in this change.

## Decisions

### 1. The Go worktree package becomes the single source of truth for task workspace identity
The manager will own canonical derivation of the managed workspace path and agent branch for a task, instead of letting services pass in arbitrary branch names or infer paths ad hoc. For this change, the canonical branch remains `agent/<task-id>` to stay compatible with the existing runtime schema, tests, and archived spawn behavior, while the canonical workspace path remains `<WORKTREE_BASE_PATH>/<project-slug>/<task-id>`.

This keeps branch/path rules in one place and prevents service-level drift as more runtime flows start touching worktrees.

Alternatives considered:
- Keep branch naming in `AgentService`: rejected because lifecycle rules would stay split across packages.
- Adopt the PRD's optional hashed branch suffix immediately: rejected for now because it would ripple into persisted task metadata and existing tests without solving the current lifecycle gap.

### 2. Allocation is modeled as prepare-or-reuse with explicit guardrail errors
Instead of a write-only `Create(...)` API, the manager should expose an allocation step that can:
- create the canonical worktree when none exists;
- reuse an existing healthy managed worktree for the same task;
- reject allocation when the active managed-worktree count has reached the configured limit;
- reject path-ownership conflicts where the canonical path exists but is not a valid managed worktree for that task.

`MAX_ACTIVE_AGENTS` becomes the admission-control ceiling for newly created managed worktrees, and the manager will inspect current Git worktree state before creating another checkout.

Alternatives considered:
- Always delete and recreate worktrees on every spawn: rejected because it discards partially completed work during retry and amplifies Git churn.
- Hide all failures behind a generic create error: rejected because spawn handlers need deterministic mapping for capacity vs. corruption vs. Git execution failures.

### 3. Cleanup is idempotent and includes Git metadata repair for managed branches
A managed release path will do more than `git worktree remove --force`: it should tolerate missing directories, prune stale Git metadata, and delete the managed branch only when it belongs to the canonical task branch namespace. The manager will also provide an inspection or garbage-collection helper for stale worktrees so startup hooks or operators can repair leaked state without editing `.git/worktrees` manually.

This matches the PRD's emphasis on leaked-worktree recovery while keeping the first implementation synchronous and explicit.

Alternatives considered:
- Limit cleanup to directory removal: rejected because Git metadata and dangling branches would still leak.
- Build a persistent worktree registry table first: rejected because current task/runtime metadata plus Git inspection is enough for this scope.

### 4. `AgentService` stays the orchestration boundary, but delegates lifecycle policy to the manager
Spawn, failure compensation, and cancellation will continue to flow through `AgentService`, yet the worktree-specific policy decisions move behind the manager contract. `AgentService` will consume richer manager results/errors, decide whether the task runtime metadata should be updated or cleared, and surface worktree denial states through consistent API responses.

This keeps HTTP handlers thin and avoids inventing a second orchestration layer just for worktrees.

Alternatives considered:
- Move worktree policy into handlers: rejected because it couples HTTP routing to Git state.
- Create a separate worktree workflow service: viable later, but unnecessary for the current scope.

## Risks / Trade-offs

- [Worktree reuse can mask stale or corrupted state] -> Reuse only when the canonical path and `git worktree list` agree that the workspace is managed for the same task; otherwise fail with a conflict/stale-state error.
- [Using `MAX_ACTIVE_AGENTS` as the initial cap may be conservative] -> Keep the limit configurable and scoped to managed agent worktrees, with room for a later dedicated worktree limit if needed.
- [Branch cleanup can remove the wrong ref if ownership checks are weak] -> Only delete branches that match the canonical managed branch for the target task.
- [Inspection/GC hooks add more surface area to test] -> Keep the API small and cover it with focused unit tests around healthy, missing-directory, and stale-metadata cases.

## Migration Plan

1. Expand `src-go/internal/worktree` to return structured allocation/inspection results and typed guardrail errors while preserving current filesystem layout.
2. Update `AgentService` and related interfaces to use the lifecycle-aware worktree contract for spawn/failure/cancel flows.
3. Add or adjust focused Go tests for prepare/reuse/remove/conflict/GC cases and spawn integration behavior.
4. Thread the needed config usage through server composition; no database migration or public API contract change is expected.

Rollback strategy: revert `AgentService` and `worktree` package changes together, returning to the current create/remove wrapper while preserving existing task/runtime columns.

## Open Questions

- Whether stale-worktree garbage collection should run only through explicit calls in this change or also on backend startup if the startup path remains fast and deterministic.
- Whether completed agent runs should release their managed branch immediately or leave branch retention for the later PR/merge workflow changes.
