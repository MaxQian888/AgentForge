## 1. Go execution context contract

- [x] 1.1 Expand `src-go/internal/bridge/client.go` request and status DTOs so Bridge-bound execute or resume calls can carry bounded `team_id` / `team_role` metadata plus the richer normalized role fields required by the updated Bridge contract.
- [x] 1.2 Extend Go-side role execution profile projection to include runtime-facing `tools`, `knowledge_context`, and `output_filters` while keeping unsupported advanced metadata out of the Bridge payload.
- [x] 1.3 Introduce a shared Go-side execution-context builder so ordinary spawn, resume, and Team-managed execution paths all construct the same canonical Bridge request shape.

## 2. Spawn and Team lifecycle propagation

- [x] 2.1 Update `AgentService` spawn and resume flows to use the shared execution-context builder and propagate optional Team context before Bridge startup instead of relying on post-start inference.
- [x] 2.2 Update `TeamService` planner, coder, reviewer, and retry flows so Team-managed runs bind the correct `team_id` / `team_role` and reuse the same canonical runtime or role selection across each phase.
- [x] 2.3 Review any queued admission or later-start path touched by spawn orchestration and ensure it preserves the canonical runtime or role context needed for delayed execution and diagnostics.

## 3. Bridge runtime continuity and validation

- [x] 3.1 Extend `src-bridge` execute and resume schemas or runtime types to validate bounded Team context and the richer normalized `role_config` fields without breaking non-Team callers.
- [x] 3.2 Preserve the resolved runtime, role, and Team context in Bridge runtime state, status responses, and session snapshots so pause or resume and diagnostics stay identity-stable.
- [x] 3.3 Align execute-path consumers with the expanded role payload so plugin selection, knowledge injection, and output filtering use the projected fields and unsupported Team roles fail fast.

## 4. Verification and documentation

- [x] 4.1 Add focused Go tests for role projection, spawn or resume request construction, Team phase propagation, and any delayed-start behavior touched by the change.
- [x] 4.2 Add focused Bridge tests for schema validation, status or snapshot identity, and execute-path consumption of `tools`, `knowledge_context`, and `output_filters`.
- [x] 4.3 Update role and orchestration documentation to describe the new execution-context contract, Team context boundaries, and the targeted verification path for this TSBridge seam.
