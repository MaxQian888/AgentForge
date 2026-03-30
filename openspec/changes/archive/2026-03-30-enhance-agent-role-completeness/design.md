## Context

The AgentForge dashboard implements role authoring (workspace with 6-section editor, context rail, catalog) and agent management (pool monitor, spawn dialog, agent detail). The backend APIs and data models are complete — all required endpoints exist and return the full data structures. However, several frontend surfaces have gaps against the spec requirements:

1. **Role workspace** lacks provenance indicators on advanced fields (REQ-10) and omits security permissions/resource limits editors (REQ-9 partial).
2. **Agent spawn dialog** sends `roleId` in the API payload but has no role selector UI.
3. **Dispatch visibility components** (`dispatch-history-panel`, `dispatch-preflight-dialog`) exist as standalone components but are not integrated into the agents dashboard or agent detail page.
4. **Dead code** (`role-form-dialog.tsx`) adds maintenance overhead.

All changes are frontend-only. No backend API changes are required.

## Goals / Non-Goals

**Goals:**
- Close the gap between spec requirements and frontend implementation for role authoring governance (REQ-9, REQ-10)
- Make dispatch lifecycle visible on the agents page and agent detail (REQ-3, REQ-4 from agent specs)
- Allow operators to select a role when spawning agents
- Remove confirmed dead code

**Non-Goals:**
- Backend changes — all required API endpoints and models already exist
- Role inheritance engine changes — inheritance resolution logic in `role-management.ts` is already correct
- New API endpoints or data models
- Mobile/tablet-first design — existing responsive breakpoints are sufficient
- Changing role-skill provenance (already complete per REQ-12)

## Decisions

### D1: Provenance indicators use badges, not inline text

Provenance for advanced fields (custom settings, MCP servers, knowledge sources) will be shown as small colored badges: `inherited`, `template`, `explicit`. This matches the existing skill resolution provenance pattern already used in `role-workspace-editor.tsx` lines 635-656.

**Alternative considered**: Tooltip-only provenance — rejected because it hides important authoring context behind hover, which conflicts with REQ-10's requirement for visible provenance.

### D2: Provenance computation added to `role-management.ts`

The existing `resolveRoleSkillReferences()` already computes provenance for skills. A new function `computeFieldProvenance(draft, parentManifest, templateManifest)` will return provenance maps for all advanced field categories. This centralizes provenance logic alongside existing role utilities.

**Alternative considered**: Computing provenance inline in the editor component — rejected because it duplicates logic and makes testing harder.

### D3: Security permissions and resource limits go in Governance section

The Governance section already has `permissionMode`, `allowedPaths`, `deniedPaths`, and `requireReview`. The new `permissions` sub-editor (fileAccess, network, codeExecution) and `resourceLimits` sub-editor will be added as collapsible sub-sections within Governance, keeping all security-related editing in one place.

**Alternative considered**: A separate "Advanced Security" section — rejected because it fragments security settings across two sections.

### D4: Role selector in spawn dialog uses existing role store

The spawn dialog will add a `Select` dropdown populated from `useRoleStore().roles`. This is the simplest integration since `fetchRoles()` is already called on workspace mount. If roles aren't loaded, the dialog fetches them on open.

### D5: Dispatch panels integrated via tabs on agents page

The agents page already has pool metrics and agent tables. Dispatch history and preflight will be added as a "Dispatch" tab alongside the existing content, keeping the page organized without overwhelming the default view.

**Alternative considered**: Inline cards on the main agents view — rejected because the page is already dense with pool metrics, diagnostics, runtime catalog, queue table, and agents table.

### D6: Dead code removal is a separate, final task

`role-form-dialog.tsx` removal is deferred to the last implementation task to confirm no dynamic imports or conditional references exist. A codebase grep for all references will be run before deletion.

## Risks / Trade-offs

- **Risk**: Provenance computation may be slow for deeply inherited role chains → **Mitigation**: Provenance is computed once on draft load and updated only on explicit section changes, not on every keystroke.
- **Risk**: Adding permissions/resource limits editors increases Governance section complexity → **Mitigation**: New sub-sections are collapsible and default to collapsed state; only show when values exist.
- **Risk**: Dispatch tab on agents page may go unnoticed → **Mitigation**: Tab shows a badge count of recent dispatch events to draw attention.
- **Trade-off**: Role selector in spawn dialog adds a fetch call → Acceptable because role list is typically small and already cached by the store.
