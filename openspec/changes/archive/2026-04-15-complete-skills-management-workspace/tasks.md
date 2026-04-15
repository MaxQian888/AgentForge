## 1. Governed skill backend foundations

- [x] 1.1 Introduce Go-native governed skill inventory/detail models that load `internal-skills.yaml`, `skills-lock.json`, and `skills/builtin-bundle.yaml` into one canonical backend seam.
- [x] 1.2 Generalize the existing skill package preview loader so `skills/*`, `.agents/skills/*`, and `.codex/skills/*` can all produce the same structured preview DTO.
- [x] 1.3 Resolve per-skill governance summaries and downstream consumer metadata, including built-in bundle alignment, lock provenance, mirror targets, role-catalog availability, and marketplace handoff state.

## 2. Skills management API and actions

- [x] 2.1 Add `GET /api/v1/skills` and `GET /api/v1/skills/:id` routes plus handler logic for governed skill inventory and detail responses.
- [x] 2.2 Add `POST /api/v1/skills/verify` with family-aware verification support and per-skill machine-readable diagnostics.
- [x] 2.3 Add `POST /api/v1/skills/sync-mirrors` for workflow-mirror targets only, including blocked-action responses for unsupported skill families.

## 3. Skills workspace frontend

- [x] 3.1 Add the `/skills` App Router page, sidebar navigation entry, store wiring, and i18n labels for the new skills management workspace.
- [x] 3.2 Build the responsive skills inventory/detail workspace with family and status filters, health badges, action controls, and empty/loading/error states driven by the backend APIs.
- [x] 3.3 Render structured package preview, governance diagnostics, and downstream handoff actions for built-in runtime, repo-assistant, and workflow-mirror skills without collapsing unsupported actions into silent no-ops.

## 4. Validation and documentation

- [x] 4.1 Add Go tests covering governed skill aggregation, preview generalization, verification diagnostics, and mirror sync behavior.
- [x] 4.2 Add frontend Jest coverage for `/skills` inventory states, detail rendering, action responses, blocked states, and cross-workspace handoffs.
- [x] 4.3 Update maintainer-facing documentation so the internal skill governance guide and related operator docs describe the new `/skills` workspace and its verification/sync workflow.
