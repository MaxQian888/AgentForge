## 1. Store and data-flow groundwork

- [x] 1.1 Extend `lib/stores/plugin-store.ts` to load marketplace entries and support panel-level filtering/detail selection helpers for installed, built-in, and marketplace plugin sections.
- [x] 1.2 Extend role editing state in the roles page/components so structured drafts can cover metadata, identity, capabilities, knowledge, security, and inheritance fields without falling back to raw JSON.
- [x] 1.3 Extend workflow client state so the dashboard can store a bounded recent activity list sourced from `workflow.trigger_fired` events and expose realtime degradation status to the workflow page.

## 2. Plugin management panel

- [x] 2.1 Rework `app/(dashboard)/plugins/page.tsx` into a multi-section management console with installed, built-in, and marketplace sections plus shared search and filter controls.
- [x] 2.2 Add plugin detail UI that surfaces runtime host, permissions, resolved source path, runtime metadata, restart count, health/error details, and action availability explanations.
- [x] 2.3 Update plugin action components so enable/disable/activate/restart/health/install actions are gated by real lifecycle and runtime support, including browse-only marketplace entries.

## 3. Role configuration panel

- [x] 3.1 Redesign `components/roles/role-form-dialog.tsx` into structured sections for identity, capabilities, knowledge, security, and inheritance fields aligned with the current role manifest contract.
- [x] 3.2 Add template and inheritance entry flows so new roles can start from an existing role or extend one without manually copying every field.
- [x] 3.3 Expand the role list UI to show execution-relevant summaries such as tags, tool/budget constraints, inheritance markers, and safety cues like review or path restrictions.

## 4. Workflow visualization panel

- [x] 4.1 Enhance `components/workflow/workflow-config-panel.tsx` with a readable workflow graph derived from the current transition configuration and a trigger summary aligned to the graph.
- [x] 4.2 Add a recent workflow activity panel that listens for `workflow.trigger_fired` events, keeps a bounded feed, and shows realtime degraded state when live activity is unavailable.
- [x] 4.3 Keep draft visualization, dirty state, save state, and error handling synchronized so unsaved changes remain visible without losing the operator's edits after failures.

## 5. Verification

- [x] 5.1 Add or update focused frontend tests for plugin filtering/details/action gating, role template/inheritance editing, and workflow visualization/activity/degraded-state behavior.
- [x] 5.2 Run the relevant repo validation commands for the touched frontend surface and confirm the new panel flows pass without regressing the existing dashboard pages.
