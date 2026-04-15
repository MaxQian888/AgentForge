## 1. Role Governance Backend

- [x] 1.1 Add a unified role reference aggregation seam in Go that collects current role consumers across installed plugin/workflow bindings, team member role bindings, queued execution requests, and advisory historical run records with stable blocking metadata.
- [x] 1.2 Expose the aggregated role reference contract through a dedicated role-reference read path and reuse it in `DELETE /api/v1/roles/:id` so delete conflicts return structured blocker details instead of a generic string.

## 2. Role Management Surface

- [x] 2.1 Update the roles workspace/catalog delete flow to request and render authoritative blocking versus advisory role consumers before destructive confirmation.
- [x] 2.2 Add focused role-management tests covering grouped blocker display, advisory-only history display, and structured delete-conflict handling.

## 3. Team Member Role Binding Governance

- [x] 3.1 Add authoritative role-registry validation for agent-member create/update flows so unknown bound `roleId` values are rejected with field-level feedback.
- [x] 3.2 Update team roster, readiness summaries, and attention/edit flows to distinguish missing role setup from stale role binding, with targeted frontend and backend tests.

## 4. Spawn And Dispatch Preflight

- [x] 4.1 Resolve the effective spawn role binding from either the explicit request `roleId` or the target member's saved agent profile before queue admission or runtime startup, and block stale bindings consistently.
- [x] 4.2 Add focused spawn/dispatch tests to verify stale effective role bindings return actionable preflight errors and do not create queue entries or agent runs.

## 5. Docs And Verification

- [x] 5.1 Update operator-facing role/runtime docs and API references to describe role reference governance, delete blockers, stale member bindings, and preflight failure semantics.
- [x] 5.2 Run focused verification for the touched role, member/team, and spawn/dispatch seams and record any remaining unrelated repo-wide blockers separately from this change.
