## 1. Trigger contract normalization

- [x] 1.1 Extend the task workflow trigger model and request/response shaping so project workflow config uses canonical action names (`dispatch_agent`, `start_workflow`, `notify`, `auto_transition`) while continuing to normalize supported legacy aliases.
- [x] 1.2 Update task workflow config validation, handler wiring, and focused tests so invalid `start_workflow` configs or unknown actions return explicit non-success outcomes instead of silent no-ops.

## 2. Starter trigger profile truth

- [x] 2.1 Extend workflow plugin trigger profile parsing and validation so built-in starters can declare structured task-driven trigger profiles in addition to existing manual triggers.
- [x] 2.2 Update built-in starter manifests and related validation/tests so `task-delivery-flow` exposes at least one executable task-driven profile and manual-only starters remain truthfully unavailable for task-triggered activation.

## 3. Task-triggered workflow execution

- [x] 3.1 Add a canonical workflow-start action path in `TaskWorkflowService` that validates starter/profile availability, constructs task-scoped trigger payloads, and starts workflow runs through the existing workflow plugin runtime.
- [x] 3.2 Add duplicate-run and dependency guards for task-triggered starter activation so equivalent active runs are blocked or skipped before a second run is created.
- [x] 3.3 Expand task workflow trigger results and websocket payloads to include normalized action, outcome, reason metadata, and workflow starter/run lineage for task-triggered starts.

## 4. Consumer alignment

- [x] 4.1 Align workflow config consumers and docs with the canonical trigger vocabulary and structured trigger outcomes, including the existing workflow config store/panel contract and relevant task/workflow API documentation.
- [x] 4.2 Reuse existing task progress or adjacent activity seams so successful task-triggered workflow orchestration registers meaningful task activity without inventing a parallel audit surface.

## 5. Verification

- [x] 5.1 Add or tighten focused Go tests for task workflow trigger normalization, task-triggered workflow starts, duplicate-run blocking, and structured trigger outcome payloads.
- [x] 5.2 Add or update workflow starter/runtime tests covering task-driven trigger profile validation and truthfulness for supported versus manual-only starters.
- [x] 5.3 Run targeted verification for the affected Go task/workflow packages and any manifest or bundle validation paths touched by the starter profile changes, then record any remaining out-of-scope gaps truthfully.
