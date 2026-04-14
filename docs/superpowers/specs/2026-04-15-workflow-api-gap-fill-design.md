# Workflow API Gap Fill Design

**Date:** 2026-04-15
**Status:** Draft
**Scope:** Wire remaining 6 backend workflow API endpoints to frontend UI: human review list + approval, external event sending, template browsing/cloning/execution

## Context

The workflow editor enhancement (completed earlier today) covers the DAG editor. Six backend API endpoints remain unwired:

| Endpoint | Store Function | Gap |
|----------|---------------|-----|
| `GET /workflow-reviews` | missing | No store function, no UI |
| `POST /executions/:id/review` | `resolveReview` | Store exists, no UI |
| `POST /executions/:id/events` | `sendExternalEvent` | Store exists, no UI |
| `GET /workflow-templates` | `fetchTemplates` | Store exists, no UI |
| `POST /workflow-templates/:id/clone` | `cloneTemplate` | Store exists, no UI |
| `POST /workflow-templates/:id/execute` | `executeTemplate` | Store exists, no UI |

## Decision Record

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Placement | New tabs in existing Workflow page | Keeps workflow features colocated, no new routes needed |
| Review interaction | List + inline expand | Fast approval without context switches |
| Template variables | Editable before clone/execute | Backend supports overrides, surfacing them avoids "clone then edit" friction |
| Dual entry for review/events | Reviews Tab + Execution detail view | Global view for reviewers, contextual action for execution watchers |

## 1. Store Changes

### New type in `workflow-store.ts`

```ts
interface WorkflowPendingReview {
  id: string;
  executionId: string;
  nodeId: string;
  projectId: string;
  reviewerId?: string;
  prompt: string;
  context?: Record<string, unknown>;
  decision: string;   // "pending" | "approved" | "rejected"
  comment: string;
  createdAt: string;
  resolvedAt?: string;
}
```

### New state + function

```ts
pendingReviews: WorkflowPendingReview[];
pendingReviewsLoading: boolean;
fetchPendingReviews: (projectId: string) => Promise<void>;
```

`fetchPendingReviews` calls `GET /api/v1/projects/${projectId}/workflow-reviews` and sets `pendingReviews`.

Existing functions (`resolveReview`, `sendExternalEvent`, `fetchTemplates`, `cloneTemplate`, `executeTemplate`) remain unchanged — they only need UI consumers.

## 2. Page Tab Extension

`app/(dashboard)/workflow/page.tsx` gains two new tabs:

```
Config | Workflows | Executions | Reviews | Templates
```

- Reviews tab renders `<WorkflowReviewsTab projectId={...} />`
- Templates tab renders `<WorkflowTemplatesTab projectId={...} />`

## 3. Reviews Tab

### Component: `components/workflow/workflow-reviews-tab.tsx`

Props: `{ projectId: string }`

**Behavior:**
- On mount: call `fetchPendingReviews(projectId)`
- Display pending reviews as a list of expandable cards
- Each card (collapsed) shows:
  - Decision badge (pending=yellow, approved=green, rejected=red)
  - Prompt text (truncated)
  - Execution ID (first 8 chars, monospace)
  - Node ID
  - Created timestamp
- Each card (expanded) shows:
  - Full prompt text
  - Context JSON viewer (collapsible `<pre>` block, if context exists)
  - Decision form:
    - Approve / Reject button pair
    - Optional comment textarea
    - Submit button
  - On submit: call `resolveReview(executionId, nodeId, decision, comment)`
  - On success: toast notification + refresh list
- Empty state when no pending reviews
- Loading skeleton while fetching

## 4. Execution View Enhancement

### Changes to `components/workflow/workflow-execution-view.tsx`

Enhance the existing `NodeCard` component to show inline actions for waiting nodes:

**When node status is `waiting` AND node type is `human_review`:**
- Show amber "Awaiting Review" badge
- Show expandable review form:
  - Approve / Reject buttons
  - Comment textarea
  - Submit calls `resolveReview(executionId, nodeId, decision, comment)`
  - On success: toast + re-fetch execution

**When node status is `waiting` AND node type is `wait_event`:**
- Show slate "Waiting for Event" badge
- Show expandable event form:
  - JSON payload textarea (monospace, placeholder: `{"event": "value"}`)
  - Send Event button
  - Submit calls `sendExternalEvent(executionId, nodeId, parsedPayload)`
  - Validate JSON before sending; show error if invalid
  - On success: toast + re-fetch execution

The execution view already polls every 3 seconds while running/pending. After resolving a review or sending an event, the node should transition out of `waiting` on the next poll cycle.

Note: The execution view currently creates its own `apiClient` directly instead of using the Zustand store. To call `resolveReview` and `sendExternalEvent`, import `useWorkflowStore` and call the store functions. This keeps the API call logic centralized.

## 5. Templates Tab

### Component: `components/workflow/workflow-templates-tab.tsx`

Props: `{ projectId: string }`

**Behavior:**
- On mount: call `fetchTemplates()` (no category filter, get all)
- Category filter bar at top: "All" | "System" | "User" | "Marketplace" — toggle buttons, filter client-side from the full list
- Template card grid (responsive: 1 col on mobile, 2 on md, 3 on lg):
  - Name (bold), description
  - Category badge (system=blue, user=green, marketplace=purple)
  - Node count, edge count
  - Template variables preview: list of key names from `templateVars` (e.g., "runtime, model, budgetUsd")
  - Two action buttons: "Clone" and "Execute"
- Empty state when no templates
- Loading skeleton while fetching

### Shared Component: `components/workflow/workflow-template-vars-dialog.tsx`

Reusable dialog for both Clone and Execute flows.

Props:
```ts
interface TemplateVarsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  template: WorkflowDefinition;
  mode: "clone" | "execute";
  onSubmit: (overrides: Record<string, unknown>, taskId?: string) => Promise<void>;
  loading: boolean;
}
```

**Dialog content:**
- Title: "Clone Template: {name}" or "Execute Template: {name}"
- Dynamic form: iterate `Object.entries(template.templateVars ?? {})`, render one field per key:
  - Key name as label
  - Input pre-filled with default value (from templateVars)
  - All fields are text inputs (values are stringified for display, parsed back on submit)
- If mode is "execute": additional optional Task ID input field
- Submit button: "Clone" or "Execute" (with loading state)
- Cancel button

**Clone flow:**
1. User clicks "Clone" on template card
2. Dialog opens with template vars pre-filled
3. User optionally modifies values, clicks "Clone"
4. Calls `cloneTemplate(templateId, projectId, overrides)`
5. On success: toast, close dialog, switch to Workflows tab, select the new definition

**Execute flow:**
1. User clicks "Execute" on template card
2. Dialog opens with template vars pre-filled + Task ID field
3. User optionally modifies values and task ID, clicks "Execute"
4. Calls `executeTemplate(templateId, projectId, taskId, variables)`
5. On success: toast, close dialog, switch to Executions tab

Tab switching after clone/execute: the workflow page manages the active tab via state. The template tab's callbacks update this state to switch tabs programmatically.

## 6. File Summary

### New files (3)

| File | Responsibility |
|------|----------------|
| `components/workflow/workflow-reviews-tab.tsx` | Reviews Tab: pending review list with inline approval |
| `components/workflow/workflow-templates-tab.tsx` | Templates Tab: template card grid with category filter |
| `components/workflow/workflow-template-vars-dialog.tsx` | Shared dialog: template variable override form for clone/execute |

### Modified files (3)

| File | Changes |
|------|---------|
| `lib/stores/workflow-store.ts` | Add `WorkflowPendingReview` type, `pendingReviews` state, `fetchPendingReviews` function |
| `app/(dashboard)/workflow/page.tsx` | Add Reviews + Templates tabs, manage active tab state for programmatic switching |
| `components/workflow/workflow-execution-view.tsx` | Add inline review form for human_review waiting nodes, inline event form for wait_event waiting nodes |

### Unchanged

- `components/workflow-editor/` — editor module not touched
- `components/workflow/workflow-config-panel.tsx` — not touched

## 7. Out of Scope

- Review assignment (assigning specific reviewers to reviews)
- Template CRUD (creating/editing/deleting templates from the UI — templates are managed via backend seeding and marketplace)
- Template import/export UI
- WebSocket-based real-time review notifications (polling suffices for now)
