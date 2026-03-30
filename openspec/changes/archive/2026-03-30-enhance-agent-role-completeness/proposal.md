## Why

The Role workspace and Agent management surfaces have been built out structurally, but several spec-required features remain incomplete or disconnected. Specifically: advanced field provenance indicators (REQ-10) are absent, security permissions and resource limits have no editors (REQ-9 partial gap), the dispatch history/preflight components exist but are not integrated into the agents dashboard, and the spawn dialog lacks a role selector. These gaps prevent operators from fully leveraging the authoring, governance, and dispatch workflows documented in the specifications.

## What Changes

- **Role editor: add security permissions editor** — expose `permissions.fileAccess`, `permissions.network`, `permissions.codeExecution` fields in the Governance section of `role-workspace-editor.tsx`.
- **Role editor: add resource limits editor** — expose `resourceLimits` (token budget, API calls, execution time, cost limits) in the Governance section.
- **Role editor: add provenance indicators** — show inherited / template-derived / explicit badges on advanced fields (custom settings, MCP servers, knowledge sources, skills) per REQ-10.
- **Role context rail: provenance in advanced panel** — distinguish inherited vs explicitly-set values in the Advanced Authoring context panel.
- **Agent spawn dialog: add role selector** — allow operators to pick a role when spawning an agent, sending `roleId` in the spawn request.
- **Agents page: integrate dispatch history panel** — surface `dispatch-history-panel` on the agents dashboard so operators can see dispatch attempt history.
- **Agents page: integrate dispatch preflight dialog** — surface preflight checks from the agents page, not only from the spawn flow.
- **Agent detail page: add dispatch context section** — show dispatch outcome, preflight summary, and budget metadata on the agent detail page.
- **Remove dead code** — remove `role-form-dialog.tsx` if confirmed unused, reducing maintenance burden.

## Capabilities

### New Capabilities

_(none — all changes enhance existing capabilities)_

### Modified Capabilities

- `role-management-panel`: Adding provenance indicators (REQ-10 gap), security permissions editor, and resource limits editor (REQ-9 gap) to the role workspace.
- `agent-pool-control-plane`: Integrating dispatch history panel and preflight dialog into the agents dashboard (REQ-4 visibility gap).
- `agent-spawn-orchestration`: Adding role selector to the spawn dialog so `roleId` parameter is operator-accessible (REQ-1 context gap).
- `agent-task-dispatch`: Surfacing dispatch outcome and preflight context on agent detail page (REQ-3 visibility gap).

## Impact

- **Frontend components**: `role-workspace-editor.tsx`, `role-workspace-context-rail.tsx`, `spawn-agent-dialog.tsx`, `agents/page.tsx`, `agent/page.tsx`
- **Stores**: `role-store.ts` (provenance data), `agent-store.ts` (dispatch context on detail)
- **Utilities**: `role-management.ts` (provenance computation for non-skill fields)
- **Tests**: Corresponding test files for all modified components
- **Dead code**: `role-form-dialog.tsx` and `role-form-dialog.test.tsx` removed if confirmed unused
- **No backend/API changes** — all required endpoints and data already exist
