# AgentForge Task Workspace P0 Design

Date: 2026-03-24
Scope: first-wave project task workspace for multi-view task management, shared filtering, task detail context, progress health, and stall alerts

## Context

AgentForge already has a real project task surface in [app/(dashboard)/project/page.tsx](/d:/Project/AgentForge/app/(dashboard)/project/page.tsx), a shared task store in [lib/stores/task-store.ts](/d:/Project/AgentForge/lib/stores/task-store.ts), workspace UI state in [lib/stores/task-workspace-store.ts](/d:/Project/AgentForge/lib/stores/task-workspace-store.ts), and progress-health events flowing through [lib/stores/ws-store.ts](/d:/Project/AgentForge/lib/stores/ws-store.ts). Recent OpenSpec work also landed the baseline capabilities for:

- multi-view project task workspace
- task progress tracking
- task progress alerting
- assignment-triggered agent dispatch

That means this wave is not a greenfield dashboard redesign. It is a productization pass that tightens the current project task page into a usable P0 task center inside one project context.

The user selected the first-wave scope as:

- Board, List, Timeline, and Calendar in one workspace
- shared filters and retained project context
- task detail and quick operational edits
- progress health and stall alerts

Explicitly out of scope for this wave:

- sprint and burn-down management
- dependency graph and blockedBy automation
- custom workflow configuration
- project or sprint cost center redesign

## Goals

- Keep one project-scoped task workspace instead of splitting task operations across multiple routes.
- Preserve the same task set across Board, List, Timeline, and Calendar views.
- Make progress-health signals visible enough for daily operation without turning the page into a dashboard clone.
- Let users inspect and update the selected task without leaving the current workspace.
- Reuse existing task and websocket contracts wherever possible.

## Non-Goals

- No new sprint planning center, cycle reporting, or velocity analytics.
- No dependency editor, dependency graph, or automatic unblocking flow.
- No custom workflow rule editor.
- No organization-level alert center or configurable threshold management UI.
- No broad dashboard homepage redesign.

## Recommended Direction

Use a unified workspace hub inside the existing project page.

The page keeps the current project-level route and project selector semantics, then strengthens the task workspace into three coordinated regions:

1. a shared control band for filters and view switching
2. a central view region for Board, List, Timeline, and Calendar
3. a persistent right-side context rail for progress summary, selected-task detail, and recent alerts

This direction is preferred over a more dashboard-heavy command center because it fits the repo's current structure, minimizes route churn, and keeps the user focused on project tasks rather than summary chrome.

## Page Structure

### Project Shell

The workspace remains anchored in [app/(dashboard)/project/page.tsx](/d:/Project/AgentForge/app/(dashboard)/project/page.tsx). The route does not split into separate `/board`, `/list`, `/timeline`, or `/calendar` pages.

The top of the page keeps:

- project title
- current project context
- create task action

This avoids resetting task state, selected task, and filters whenever the user changes representation.

### Shared Control Band

Below the page header, the workspace exposes one shared control band with:

- search
- status filter
- priority filter
- assignee filter
- planning filter
- view switch for Board, List, Timeline, and Calendar

These controls are shared across all views. Changing views must not reset filters or search terms.

### Central Multi-View Workspace

The center of the page renders the selected view against the same filtered task set.

- Board remains the main workflow view and keeps drag-and-drop status changes.
- List remains the dense operational view for scanning many tasks quickly.
- Timeline and Calendar operate on explicit planning fields and keep drag-based rescheduling.

The page does not create per-view task copies or separate data loaders.

### Right-Side Context Rail

The current light task sheet evolves into a persistent context rail with three stacked sections:

1. Progress health summary
2. Selected task detail
3. Recent alerts

This makes progress signals visible even when no modal or sheet is open, while still allowing a selected task to receive deeper attention.

Desktop behavior:

- the context rail is expanded by default beside the main workspace
- collapsing the rail is an explicit user action for reclaiming space, not an automatic mode switch

Responsive behavior:

- on narrower layouts, the same content may render in an overlay or stacked panel instead of a permanently visible side rail
- this is a presentation adaptation only; the content contract remains the same

## Task Data And State Model

### One Task Fact Source

[lib/stores/task-store.ts](/d:/Project/AgentForge/lib/stores/task-store.ts) remains the single fact source for task entities. Board, List, Timeline, and Calendar all read from that same normalized task collection.

This store remains responsible for:

- loading project tasks
- create and update actions
- status transitions
- assignment updates
- upserting websocket-driven task changes

### Separate Workspace UI State

[lib/stores/task-workspace-store.ts](/d:/Project/AgentForge/lib/stores/task-workspace-store.ts) remains a UI-only store. It should own:

- `viewMode`
- `filters`
- `selectedTaskId`
- `contextRailDisplay` with explicit `expanded` and `collapsed` states

It should not own full task payloads or duplicate server-backed entity data.

### Derived Context Rail Content

The right-side rail is derived from:

- the currently filtered task set
- the currently selected task id
- incoming progress and notification events

This yields:

- health counts for healthy, warning, stalled, and unscheduled tasks
- the selected-task detail card
- recent alert items for warning, stalled, and recovery transitions

## Interaction Model

### Cross-View Behavior

All four views must preserve:

- active project context
- search and filters
- selected task identity
- task detail access

Opening a task from any view selects the task and refreshes the context rail rather than navigating away.

Selection validity rules:

- if the selected task is hidden by current filters, the selection remains active and the context rail explains that the task is currently outside the visible result set
- if the selected task is deleted, removed from the active project, or no longer exists in the loaded task source, the workspace clears `selectedTaskId` and returns the rail to summary mode
- changing views alone must not clear a valid selection

### Board

Board remains the Kanban-like workflow surface.

- Dragging between status columns updates task workflow status.
- Failed persistence restores or clearly marks the affected card state.
- Risk badges remain visible on cards so stalled work is discoverable without opening each task.

### List

List is the dense comparison view.

Each row should expose at minimum:

- title
- status
- progress health
- priority
- assignee
- planning state

This is the most scan-friendly surface for spotting tasks that are both high priority and operationally at risk.

### Timeline And Calendar

Timeline and Calendar are schedule-aware planning surfaces.

- They rely on `plannedStartAt` and `plannedEndAt`.
- Dragging reschedules the task through the same update path used by task editing.
- Unscheduled tasks remain explicitly visible rather than disappearing.
- Planning validation follows one canonical rule:
  - both dates present: scheduled range
  - both dates absent: unscheduled task
  - only one date present from a detail edit: normalize it into a single-day schedule by copying the provided date to both boundaries
  - end earlier than start: reject the update and preserve the prior persisted planning window

### Selected Task Detail

The selected-task section is an operational detail card, not a full planning console.

Wave-one editable fields:

- status
- priority
- planned start
- planned end

Wave-one display fields:

- assignee
- progress health
- risk reason
- last activity time
- last activity source

This keeps the detail surface practical without expanding into sprint, dependency, or workflow authoring.

## Progress Tracking And Stall Alerts

### Separate Workflow Status From Progress Health

`status` and `progress health` remain separate dimensions.

- Workflow status stays as `inbox`, `triaged`, `assigned`, `in_progress`, `in_review`, `done`
- Progress health stays as `healthy`, `warning`, `stalled`

The UI must not collapse them into one badge or one state label.

### Risk Signal Exposure By View

The same progress-health signal appears at different densities depending on the view:

- Board: inline risk badge on each card
- List: dedicated progress column with short reason text
- Timeline and Calendar: lighter visual signal such as border or accent strip
- Context rail: full detail including timestamps and reason

### Wave-One Risk Reasons

Wave one intentionally limits user-facing risk reasons to:

- `No recent update`
- `No assignee`
- `Awaiting review`

This keeps reasoning, UI language, and backend mapping stable for the first delivery pass.

These labels are presentation mappings from existing machine-readable `riskReason` values rather than a new backend taxonomy.

### Alert Behavior

The recent-alerts section should show:

- warning entries
- stalled entries
- recovery entries

Alert generation follows the existing dedupe rule:

- no duplicate alert while the same condition remains unchanged
- emit a new alert when the condition worsens or returns after recovery

### Update Rules

Task state and progress-health state should update in place from websocket events already flowing through [lib/stores/ws-store.ts](/d:/Project/AgentForge/lib/stores/ws-store.ts), especially:

- `task.updated`
- `task.transitioned`
- `task.assigned`
- `task.progress.updated`
- `task.progress.alerted`
- `task.progress.recovered`

The page should not depend on separate polling just to keep progress indicators current.

If realtime delivery becomes temporarily unavailable:

- the workspace remains usable from the latest fetched task state
- the UI surfaces a non-blocking degraded-realtime indicator
- direct edits and manual refresh remain available
- the page must not imply that alerts are live while disconnected

## Empty States And Error States

The workspace should distinguish five conditions clearly:

1. project has no tasks
2. filters removed all visible tasks
3. an update action failed but the workspace is still usable
4. the initial task load failed
5. realtime updates are temporarily degraded

This prevents users from confusing "no data" with "save failed".

## Verification Strategy

Wave-one validation should prove that the project task workspace is genuinely operable, not just scaffolded.

Core verification path:

- load project tasks
- switch across Board, List, Timeline, and Calendar without losing filters
- render a retryable workspace error state when the initial task load fails
- drag a task in Board and persist status change
- drag a task in Timeline or Calendar and persist planning change
- reject invalid schedule edits such as end-before-start without overwriting the last valid plan
- select a task and see the context rail refresh correctly
- keep a selected task active when it is filtered out and show the hidden-by-filter state
- clear the selection when the selected task is deleted or removed from the active project
- receive websocket-driven task or progress updates and see the workspace update in place
- simulate websocket disconnect and verify degraded-realtime signaling without breaking direct task operations

Frontend testing should focus on:

- workspace UI store behavior
- project task workspace rendering and drag failure handling
- selected-task context rail behavior
- progress badge and recent-alert rendering

Backend work in this wave should reuse existing task, progress, and websocket contracts rather than opening new sprint or workflow domains.

## Done Condition

This design is complete for wave one when AgentForge supports the following product statement:

"Inside one project-scoped task workspace, a user can switch among Board, List, Timeline, and Calendar views, update workflow state and planning through drag interactions, and continuously see progress risk plus recent task alerts without leaving the current project context."
