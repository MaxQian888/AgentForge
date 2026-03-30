## 1. Role Provenance Infrastructure

- [x] 1.1 Add `computeFieldProvenance()` function to `lib/roles/role-management.ts` that compares draft fields against parent manifest and template manifest, returning a provenance map (`inherited` | `template` | `explicit`) for custom settings, MCP servers, knowledge sources, memory settings, collaboration, and triggers
- [x] 1.2 Add unit tests for `computeFieldProvenance()` in `lib/roles/role-management.test.ts` covering inherited, template-derived, explicit, and override scenarios
- [x] 1.3 Add `ProvenanceBadge` UI component (small inline badge showing `inherited` / `template` / `explicit` with color coding) to `components/roles/` or `components/ui/`

## 2. Role Editor: Provenance Indicators (REQ-10)

- [x] 2.1 Update `role-workspace-editor.tsx` Capabilities section to display provenance badges on each custom setting row using the provenance map from `computeFieldProvenance()`
- [x] 2.2 Update `role-workspace-editor.tsx` Capabilities section to display provenance badges on each MCP server row
- [x] 2.3 Update `role-workspace-editor.tsx` Knowledge section to display provenance badges on knowledge source rows (shared and private)
- [x] 2.4 Update `role-workspace-editor.tsx` Governance section to display provenance badges on collaboration and trigger rows
- [x] 2.5 Update `role-workspace-context-rail.tsx` Advanced Authoring panel to show per-field provenance with counts of inherited vs explicit values
- [x] 2.6 Update provenance badge from `inherited` to `explicit` when operator modifies an inherited field value

## 3. Role Editor: Security Permissions & Resource Limits (REQ-9)

- [x] 3.1 Add `RoleDraft` fields for permissions (fileAccess.allowedPaths, fileAccess.deniedPaths, network.allowedDomains, codeExecution.sandbox, codeExecution.allowedLanguages) and resource limits (tokenBudget, apiCalls, executionTime, costLimit) in `lib/roles/role-management.ts`
- [x] 3.2 Update `buildRoleDraft()` to populate permissions and resource limits fields from existing manifest
- [x] 3.3 Update `serializeRoleDraft()` to include permissions and resource limits in the serialized manifest payload
- [x] 3.4 Add collapsible Permissions sub-section in Governance tab of `role-workspace-editor.tsx` with fileAccess (path lists), network (domain list), codeExecution (sandbox toggle, languages list) editors
- [x] 3.5 Add collapsible Resource Limits sub-section in Governance tab of `role-workspace-editor.tsx` with numeric inputs for tokenBudget, apiCalls, executionTime, costLimit
- [x] 3.6 Add tests for permissions and resource limits round-trip (build draft → edit → serialize → verify preserved) in `role-management.test.ts`
- [x] 3.7 Add component tests for permissions and resource limits sections in `role-workspace.test.tsx`

## 4. Agent Spawn Dialog: Role Selector

- [x] 4.1 Add role selector `Select` dropdown to `spawn-agent-dialog.tsx` using `useRoleStore().roles`, fetching roles on dialog open if not loaded
- [x] 4.2 Pass selected `roleId` to `onSpawnAgent` callback alongside budget and runtime options
- [x] 4.3 Update `spawn-agent-dialog.test.tsx` with tests for role selection, no-role-selected, and role fetch on open

## 5. Agents Page: Dispatch Visibility

- [x] 5.1 Add tab navigation to `agents/page.tsx` with "Monitor" (existing content) and "Dispatch" tabs
- [x] 5.2 Integrate `dispatch-history-panel` component into the Dispatch tab, wired to `agent-store.fetchDispatchHistory()`
- [x] 5.3 Add preflight check button in the Dispatch tab that opens `dispatch-preflight-dialog` for a selected task/member
- [x] 5.4 Add badge count on Dispatch tab showing recent dispatch event count from `agent-store.dispatchStats`
- [x] 5.5 Update `agents/page.test.tsx` with tests for tab switching, dispatch history display, and preflight dialog integration

## 6. Agent Detail Page: Dispatch Context

- [x] 6.1 Add Dispatch Context section to `agent/page.tsx` showing dispatch outcome (started/queued/blocked/skipped), preflight summary, and budget metadata
- [x] 6.2 Fetch dispatch history for the agent's task via `agent-store.fetchDispatchHistory(taskId)` on page load
- [x] 6.3 Handle manual spawn case (no dispatch context) with "Manual spawn" label and timestamp
- [x] 6.4 Add tests for dispatch context display in agent detail page

## 7. Dead Code Cleanup

- [x] 7.1 Run codebase-wide search confirming zero import references to `role-form-dialog` (excluding test file and the component itself)
- [x] 7.2 Delete `components/roles/role-form-dialog.tsx` and `components/roles/role-form-dialog.test.tsx` if confirmed unused
- [x] 7.3 Verify build and tests pass after removal
