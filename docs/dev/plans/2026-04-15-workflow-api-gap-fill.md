# Workflow API Gap Fill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the remaining 6 backend workflow API endpoints to frontend UI — human review approval, external event sending, and template browsing/cloning/execution.

**Architecture:** Add `WorkflowPendingReview` type and `fetchPendingReviews` to the existing Zustand store, create 3 new components (Reviews Tab, Templates Tab, Template Vars Dialog), enhance the existing execution view with inline review/event forms, and extend the workflow page from 3 to 5 controlled tabs.

**Tech Stack:** React 19, Next.js 16, Tailwind CSS v4, shadcn/ui (Radix), Zustand, Jest + Testing Library

**Spec:** `docs/superpowers/specs/2026-04-15-workflow-api-gap-fill-design.md`

---

## File Structure

### New files (3)

| File | Responsibility |
|------|----------------|
| `components/workflow/workflow-reviews-tab.tsx` | Reviews Tab: pending review list with inline expand approval |
| `components/workflow/workflow-templates-tab.tsx` | Templates Tab: template card grid with category filter, clone/execute buttons |
| `components/workflow/workflow-template-vars-dialog.tsx` | Shared dialog: template variable override form for clone and execute |

### Modified files (3)

| File | Changes |
|------|---------|
| `lib/stores/workflow-store.ts` | Add `WorkflowPendingReview` type, `pendingReviews` + `pendingReviewsLoading` state, `fetchPendingReviews` function |
| `app/(dashboard)/workflow/page.tsx` | Convert to controlled tabs, add Reviews + Templates tabs, import new components |
| `components/workflow/workflow-execution-view.tsx` | Add `waiting` status handling, inline review form for `human_review` nodes, inline event form for `wait_event` nodes, refactor to use store types |

### Test files

| File | Tests |
|------|-------|
| `lib/stores/workflow-store.test.ts` | Add test for `fetchPendingReviews` (existing test file, append) |
| `components/workflow/workflow-reviews-tab.test.tsx` | Review list rendering, expand/collapse, approve/reject actions |

---

## Task 1: Store Changes (fetchPendingReviews)

**Files:**
- Modify: `lib/stores/workflow-store.ts`
- Modify: `lib/stores/workflow-store.test.ts`

- [ ] **Step 1: Add type and state to store**

In `lib/stores/workflow-store.ts`:

After the existing `WorkflowNodeExecution` interface (around line 90), add:

```ts
export interface WorkflowPendingReview {
  id: string;
  executionId: string;
  nodeId: string;
  projectId: string;
  reviewerId?: string;
  prompt: string;
  context?: Record<string, unknown>;
  decision: string;
  comment: string;
  createdAt: string;
  resolvedAt?: string;
}
```

In the `WorkflowState` interface, add after the `resolveReview` declaration:

```ts
pendingReviews: WorkflowPendingReview[];
pendingReviewsLoading: boolean;
fetchPendingReviews: (projectId: string) => Promise<void>;
```

In the store creation (inside `create<WorkflowState>()`), add initial state:

```ts
pendingReviews: [],
pendingReviewsLoading: false,
```

Add the function implementation after the existing `sendExternalEvent`:

```ts
fetchPendingReviews: async (projectId) => {
  const token = useAuthStore.getState().accessToken;
  if (!token) return;
  set({ pendingReviewsLoading: true });
  try {
    const api = createApiClient(API_URL);
    const { data } = await api.get<WorkflowPendingReview[]>(
      `/api/v1/projects/${projectId}/workflow-reviews`,
      { token }
    );
    set({ pendingReviews: data ?? [], pendingReviewsLoading: false });
  } catch {
    set({ pendingReviewsLoading: false, error: "Unable to fetch reviews" });
  }
},
```

- [ ] **Step 2: Add test for fetchPendingReviews**

Append to `lib/stores/workflow-store.test.ts` — follow the existing test pattern in that file. Add a test that verifies `fetchPendingReviews` sets `pendingReviews` state after a successful API call (mock fetch).

- [ ] **Step 3: Run tests**

Run: `pnpm test -- --testPathPattern="workflow-store" --no-coverage`
Expected: All existing + new test pass

- [ ] **Step 4: Commit**

```bash
git add lib/stores/workflow-store.ts lib/stores/workflow-store.test.ts
git commit -m "feat(workflow): add fetchPendingReviews to workflow store"
```

---

## Task 2: Template Vars Dialog (shared component)

**Files:**
- Create: `components/workflow/workflow-template-vars-dialog.tsx`

- [ ] **Step 1: Create the dialog component**

```tsx
"use client";
```

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

Implementation:
- Import `WorkflowDefinition` from `@/lib/stores/workflow-store`
- Title: `mode === "clone" ? "Clone Template" : "Execute Template"` + `: ${template.name}`
- Body: iterate `Object.entries(template.templateVars ?? {})`, for each entry render:
  - `<Label>` with the key name
  - `<Input>` with `defaultValue={String(value)}` and `name={key}`
- If `mode === "execute"`: extra `<Input>` for optional Task ID (label: "Task ID (optional)")
- Use uncontrolled form with `FormData` on submit: read all inputs, build overrides object, parse numeric strings back to numbers
- Submit button: "Clone" or "Execute" with `loading` disabled state
- Cancel button: calls `onOpenChange(false)`
- Use shadcn: `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogFooter`, `Button`, `Input`, `Label`

- [ ] **Step 2: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit 2>&1 | head -20`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add components/workflow/workflow-template-vars-dialog.tsx
git commit -m "feat(workflow): add template vars dialog for clone/execute"
```

---

## Task 3: Templates Tab

**Files:**
- Create: `components/workflow/workflow-templates-tab.tsx`

- [ ] **Step 1: Create the templates tab component**

```tsx
"use client";
```

Props:
```ts
interface WorkflowTemplatesTabProps {
  projectId: string;
  setActiveTab: (tab: string) => void;
}
```

Implementation:
- Import `useWorkflowStore` — use `fetchTemplates`, `templates`, `templatesLoading`, `cloneTemplate`, `selectDefinition`, `executeTemplate`
- Import `TemplateVarsDialog` from `./workflow-template-vars-dialog`
- On mount: `useEffect(() => { fetchTemplates(); }, [fetchTemplates])`
- State: `categoryFilter: string` (default "all"), `dialogState: { open: boolean; template: WorkflowDefinition | null; mode: "clone" | "execute" }`, `dialogLoading: boolean`
- Category filter: row of `<Button variant={selected ? "default" : "outline"}>` for "All", "System", "User", "Marketplace"
- Filter: `templates.filter(t => categoryFilter === "all" || t.category === categoryFilter)`
- Card grid: `grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4`
- Each card:
  - `<Card>` with `<CardHeader>`: name (CardTitle), description (CardDescription)
  - Category badge: system=blue (`bg-blue-500/15 text-blue-700`), user=green, marketplace=purple
  - `<CardContent>`: node count, edge count, templateVars key names preview
  - Two buttons: "Clone" opens dialog with mode="clone", "Execute" opens dialog with mode="execute"
- Clone handler: `async (overrides) => { const def = await cloneTemplate(templateId, projectId, overrides); if (def) { toast.success("Template cloned"); selectDefinition(def); setActiveTab("workflows"); } }`
- Execute handler: `async (overrides, taskId) => { const exec = await executeTemplate(templateId, projectId, taskId, overrides); if (exec) { toast.success("Execution started"); setActiveTab("executions"); } }`
- Empty state: `<EmptyState icon={LayoutTemplate} title="No templates" description="No workflow templates available." />`
- Loading: skeleton cards

- [ ] **Step 2: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit 2>&1 | head -20`

- [ ] **Step 3: Commit**

```bash
git add components/workflow/workflow-templates-tab.tsx
git commit -m "feat(workflow): add templates tab with clone/execute support"
```

---

## Task 4: Reviews Tab

**Files:**
- Create: `components/workflow/workflow-reviews-tab.tsx`
- Create: `components/workflow/workflow-reviews-tab.test.tsx`

- [ ] **Step 1: Create the reviews tab component**

```tsx
"use client";
```

Props:
```ts
interface WorkflowReviewsTabProps {
  projectId: string;
}
```

Implementation:
- Import `useWorkflowStore` — use `pendingReviews`, `pendingReviewsLoading`, `fetchPendingReviews`, `resolveReview`
- On mount: `useEffect(() => { fetchPendingReviews(projectId); }, [projectId, fetchPendingReviews])`
- State: `expandedId: string | null`, `comment: string`, `submitting: boolean`
- List of cards, each review:
  - Collapsed view: decision badge (pending=`bg-yellow-500/15 text-yellow-700`, approved=green, rejected=red), prompt text (truncated to 1 line), execution ID (first 8 chars mono), node ID, `createdAt` formatted
  - Click to expand (`expandedId === review.id`)
  - Expanded view:
    - Full prompt text
    - Context JSON: if `review.context`, render in `<pre className="text-xs bg-muted p-2 rounded overflow-auto max-h-40">` wrapped in `<Collapsible>` with "Show context" trigger
    - Comment `<Textarea>` (placeholder: "Optional comment...")
    - Button row: `<Button onClick={() => handleResolve("approved")}>Approve</Button>` (green variant), `<Button variant="destructive" onClick={() => handleResolve("rejected")}>Reject</Button>`
  - `handleResolve(decision)`: set submitting, call `resolveReview(review.executionId, review.nodeId, decision, comment)`, on success toast + optimistically filter review from local display + re-fetch
- Empty state: `<EmptyState icon={ClipboardCheck} title="No pending reviews" description="All caught up!" />`
- Loading: `<Skeleton>` blocks

- [ ] **Step 2: Write test**

Create `components/workflow/workflow-reviews-tab.test.tsx`:

Mock `useWorkflowStore` to return sample pending reviews. Test:
- Renders review cards with prompt text
- Expanding shows approve/reject buttons
- Calling approve triggers `resolveReview` with correct args

- [ ] **Step 3: Run tests**

Run: `pnpm test -- --testPathPattern="workflow-reviews-tab" --no-coverage`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add components/workflow/workflow-reviews-tab.tsx components/workflow/workflow-reviews-tab.test.tsx
git commit -m "feat(workflow): add reviews tab with inline approval"
```

---

## Task 5: Execution View Enhancement

**Files:**
- Modify: `components/workflow/workflow-execution-view.tsx`

- [ ] **Step 1: Add waiting status and new node type colors**

In the `nodeStatusIcons` map (line 66), add:
```ts
waiting: Hourglass,
```
Import `Hourglass` from `lucide-react`.

In `nodeStatusColors` (line 74), add:
```ts
waiting: "border-amber-400 bg-amber-50 dark:border-amber-600 dark:bg-amber-950",
```

In `nodeTypeColors` (line 84), add:
```ts
human_review: "text-emerald-600 dark:text-emerald-400",
wait_event: "text-slate-600 dark:text-slate-400",
llm_agent: "text-indigo-600 dark:text-indigo-400",
function: "text-cyan-600 dark:text-cyan-400",
loop: "text-pink-600 dark:text-pink-400",
sub_workflow: "text-violet-600 dark:text-violet-400",
```

- [ ] **Step 2: Refactor local types to imports**

Remove the locally declared interfaces `WorkflowExecution`, `WorkflowNodeExecution`, `WorkflowNodeData`, `WorkflowEdgeData` (lines 24-63).

Replace with imports from the store:
```ts
import {
  useWorkflowStore,
  type WorkflowExecution,
  type WorkflowNodeExecution,
  type WorkflowNodeData,
  type WorkflowEdgeData,
} from "@/lib/stores/workflow-store";
```

Remove the direct `createApiClient` and `API_URL` usage. The component will use `useWorkflowStore` for `resolveReview` and `sendExternalEvent`.

- [ ] **Step 3: Add inline review form to NodeCard**

Enhance the `NodeCard` component. After the existing error message display, add:

When `status === "waiting" && node.type === "human_review"`:
- Amber "Awaiting Review" badge
- Expandable form (toggle via local state):
  - Comment textarea
  - Approve button (green) + Reject button (destructive)
  - On submit: call `useWorkflowStore.getState().resolveReview(executionId, nodeId, decision, comment)`
  - On success: toast + trigger re-fetch (call `fetchExecution` from parent)

The `NodeCard` needs the `executionId` prop added (currently only has `node`, `nodeExec`, `isActive`).

- [ ] **Step 4: Add inline event form to NodeCard**

When `status === "waiting" && node.type === "wait_event"`:
- Slate "Waiting for Event" badge
- Expandable form:
  - JSON textarea (monospace, placeholder: `{"event": "value"}`)
  - Send Event button
  - On submit: parse JSON (try/catch, show error toast if invalid), call `useWorkflowStore.getState().sendExternalEvent(executionId, nodeId, payload)`
  - On success: toast + trigger re-fetch

- [ ] **Step 5: Pass executionId to NodeCard**

In the `WorkflowExecutionView` component where `NodeCard` is rendered (around line 305), add `executionId={execution.id}` prop. Update `NodeCard` props interface to include `executionId: string`.

- [ ] **Step 6: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit 2>&1 | head -20`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add components/workflow/workflow-execution-view.tsx
git commit -m "feat(workflow): add inline review/event forms to execution view"
```

---

## Task 6: Page Integration (5 controlled tabs)

**Files:**
- Modify: `app/(dashboard)/workflow/page.tsx`

- [ ] **Step 1: Convert to controlled tabs and add new tab components**

In `WorkflowPage` (line 489):

Add state:
```ts
const [activeTab, setActiveTab] = useState("workflows");
```

Change `<Tabs defaultValue="workflows">` to:
```tsx
<Tabs value={activeTab} onValueChange={setActiveTab}>
```

Add imports:
```ts
import { WorkflowReviewsTab } from "@/components/workflow/workflow-reviews-tab";
import { WorkflowTemplatesTab } from "@/components/workflow/workflow-templates-tab";
```

Add new TabsTrigger entries in TabsList:
```tsx
<TabsTrigger value="reviews">Reviews</TabsTrigger>
<TabsTrigger value="templates">Templates</TabsTrigger>
```

Add new TabsContent entries:
```tsx
<TabsContent value="reviews" className="mt-4">
  <WorkflowReviewsTab projectId={selectedProjectId} />
</TabsContent>
<TabsContent value="templates" className="mt-4">
  <WorkflowTemplatesTab projectId={selectedProjectId} setActiveTab={setActiveTab} />
</TabsContent>
```

- [ ] **Step 2: Run all tests**

Run: `pnpm test --no-coverage`
Expected: All tests pass

- [ ] **Step 3: Type check**

Run: `pnpm exec tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add app/(dashboard)/workflow/page.tsx
git commit -m "feat(workflow): add Reviews and Templates tabs with controlled switching"
```
