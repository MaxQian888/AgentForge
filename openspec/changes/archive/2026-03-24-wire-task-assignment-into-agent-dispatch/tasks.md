## 1. Dispatch orchestration boundary

- [x] 1.1 Introduce a task-centered dispatch orchestration service that can load task/member/project context, validate agent dispatch targets, and produce structured dispatch outcomes.
- [x] 1.2 Route `POST /api/v1/tasks/:id/assign` through the new dispatch orchestration so agent assignments persist assignee changes, keep task state aligned, and attempt dispatch when appropriate.
- [x] 1.3 Route `POST /api/v1/agents/spawn` through the same dispatch orchestration so task-scoped spawn can derive the target agent from the task's current assignee when `memberId` is omitted.

## 2. Dispatch outcome and runtime integration

- [x] 2.1 Reuse the existing `AgentService` runtime startup path for dispatch-triggered starts, keeping worktree allocation, bridge execution, and compensation behavior consistent with the current spawn flow.
- [x] 2.2 Add structured dispatch outcome DTOs and handler response mapping so callers can distinguish `started`, `blocked`, and `skipped` results without inferring from status codes alone.
- [x] 2.3 Emit consistent assignment, dispatch-blocked, and runtime lifecycle feedback through WebSocket and notification surfaces for the relevant task/project scope.

## 3. IM and client contract alignment

- [x] 3.1 Update the IM AgentForge client payloads and response models so task assignment and task-scoped spawn use the canonical backend request/response contract.
- [x] 3.2 Update `/task assign` and `/agent spawn` command handlers to surface truthful dispatch results, including the difference between assignment success and agent startup success.

## 4. Verification

- [x] 4.1 Add focused Go tests for agent assignment dispatch success, blocked preflight outcomes, and task-scoped spawn fallback to the task assignee.
- [x] 4.2 Add focused IM/client tests covering the updated assignment and spawn payloads plus the user-facing started/blocked command responses.
