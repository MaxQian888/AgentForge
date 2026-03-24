# Task Workspace P0 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing project task page into a usable P0 task workspace with a persistent context rail, shared multi-view state, progress-health visibility, recent task alerts, and resilient error/realtime handling.

**Architecture:** Keep the existing project route and `task-store` as the task fact source. Add small focused helpers for planning normalization and context-rail derivation, split the current sheet-only task detail UI into reusable content plus wrappers, and compose the workspace around one central multi-view region with a persistent right-side context rail.

**Tech Stack:** Next.js 16 App Router, React 19, Zustand, Jest + Testing Library, existing WebSocket client/store wiring

---

## File Structure

### Existing files to modify

- `app/(dashboard)/project/page.tsx`
  - Keep the route-level loader thin.
  - Stop owning sheet-only detail behavior.
  - Pass the selected task, notifications, and realtime status into the workspace shell.
- `components/layout/dashboard-shell.tsx`
  - Fetch notifications after authenticated shell bootstrap so the context rail has an initial alert source.
- `components/layout/dashboard-shell.test.tsx`
  - Cover notification bootstrap and avoid regressions in shell startup behavior.
- `components/kanban/task-detail-panel.tsx`
  - Reduce this file to a sheet wrapper around reusable task-detail content.
- `components/kanban/task-detail-panel.test.tsx`
  - Prove invalid planning edits do not mutate persisted task dates.
- `components/tasks/project-task-workspace.tsx`
  - Become the workspace shell that composes the main view region and the context rail.
  - Own the `onTaskSave` pass-through from the page-level `updateTask(...)` mutation into the rail detail form.
- `components/tasks/project-task-workspace.test.tsx`
  - Expand coverage for context rail, filtered-out selection, degraded realtime, and new error states.
- `lib/stores/task-store.ts`
  - Add explicit task-load error state.
  - Preserve existing task CRUD/update behavior.
- `lib/stores/task-store.test.ts`
  - Add load-error coverage and keep current transition/update coverage.
- `lib/stores/task-workspace-store.ts`
  - Add `contextRailDisplay` and keep shared selection/filter state.
- `lib/stores/task-workspace-store.test.ts`
  - Cover the new rail display state.
- `lib/stores/ws-store.test.ts`
  - Add explicit disconnected/degraded behavior coverage instead of inferring only from task updates.

### New files to create

- `components/tasks/task-context-rail.tsx`
  - Compose the three right-rail sections.
- `components/tasks/task-progress-summary.tsx`
  - Render healthy/warning/stalled/unscheduled counts and realtime status.
- `components/tasks/task-recent-alerts.tsx`
  - Render the current project’s warning/stalled/recovery notifications.
- `components/tasks/task-detail-content.tsx`
  - Reusable task detail form/content for both inline rail and sheet wrapper.
- `components/tasks/task-context-rail.test.tsx`
  - Cover summary mode, selected-task mode, hidden-by-filter state, and degraded realtime indicator.
- `components/tasks/task-workspace-main.tsx`
  - Extract the central workspace rendering out of the already-large `project-task-workspace.tsx`.
- `lib/tasks/task-context-rail.ts`
  - Derive selected-task visibility, rail summary counts, and project-scoped recent alerts from tasks/notifications.
- `lib/tasks/task-context-rail.test.ts`
  - Cover derivation rules independently from React rendering.
- `lib/tasks/task-planning.ts`
  - Normalize planning edits into scheduled, unscheduled, single-day, or rejected-invalid states.
- `lib/tasks/task-planning.test.ts`
  - Lock the canonical planning rules before UI changes.

## Implementation Notes

- Prefer `@test-driven-development` for each task below.
- Use `@verification-before-completion` before claiming the workspace is done.
- Do not widen scope into sprint, dependency automation, custom workflow, or cost-center work.
- Keep the context rail responsive by changing presentation, not by creating a separate mobile-only data contract.

## Chunk 1: Task Workspace P0

### Task 1: Lock Workspace Derivations And UI State

**Files:**
- Create: `lib/tasks/task-context-rail.ts`
- Create: `lib/tasks/task-context-rail.test.ts`
- Modify: `lib/stores/task-workspace-store.ts`
- Modify: `lib/stores/task-workspace-store.test.ts`

- [ ] **Step 1: Write the failing derivation and UI-state tests**

```ts
import { buildContextRailState } from "./task-context-rail";

it("keeps a selected task active even when filters hide it", () => {
  const result = buildContextRailState({
    tasks: [visibleTask, hiddenSelectedTask],
    filteredTasks: [visibleTask],
    selectedTaskId: "task-hidden",
    projectId: "project-1",
    notifications: [],
  });

  expect(result.selectionState).toBe("hidden_by_filter");
  expect(result.selectedTask?.id).toBe("task-hidden");
});

it("falls back to summary mode when the selected task is gone from the task source", () => {
  const result = buildContextRailState({
    tasks: [visibleTask],
    filteredTasks: [visibleTask],
    selectedTaskId: "task-missing",
    projectId: "project-1",
    notifications: [],
  });

  expect(result.selectionState).toBe("summary");
  expect(result.selectedTask).toBeNull();
});

it("tracks explicit context rail display state", () => {
  useTaskWorkspaceStore.getState().setContextRailDisplay("collapsed");
  expect(useTaskWorkspaceStore.getState().contextRailDisplay).toBe("collapsed");
});
```

- [ ] **Step 2: Run the targeted state tests to verify failure**

Run:
```bash
pnpm exec jest --runInBand lib/stores/task-workspace-store.test.ts lib/tasks/task-context-rail.test.ts
```

Expected:
- `task-context-rail` module does not exist yet
- workspace store test fails because `setContextRailDisplay` is undefined

- [ ] **Step 3: Implement the minimal state and derivation helpers**

```ts
export type ContextRailDisplay = "expanded" | "collapsed";

export function buildContextRailState(input: BuildContextRailStateInput) {
  const selectedTask = input.tasks.find((task) => task.id === input.selectedTaskId) ?? null;
  const visibleSelectedTask =
    selectedTask && input.filteredTasks.some((task) => task.id === selectedTask.id);

  return {
    selectedTask,
    selectionState: !selectedTask
      ? "summary"
      : visibleSelectedTask
        ? "selected_visible"
        : "hidden_by_filter",
    counts: summarizeTaskHealth(input.tasks),
    alerts: selectProjectProgressAlerts(input.notifications, input.projectId),
  };
}
```

- [ ] **Step 4: Run the targeted state tests to verify pass**

Run:
```bash
pnpm exec jest --runInBand lib/stores/task-workspace-store.test.ts lib/tasks/task-context-rail.test.ts
```

Expected:
- PASS for hidden-by-filter selection
- PASS for context rail display state

- [ ] **Step 5: Commit the state/derivation slice**

```bash
git add lib/tasks/task-context-rail.ts lib/tasks/task-context-rail.test.ts lib/stores/task-workspace-store.ts lib/stores/task-workspace-store.test.ts
git commit -m "feat: add task workspace context rail state helpers"
```

### Task 2: Normalize Planning Rules Before Refactoring The Detail UI

**Files:**
- Create: `lib/tasks/task-planning.ts`
- Create: `lib/tasks/task-planning.test.ts`
- Modify: `components/kanban/task-detail-panel.tsx`
- Create: `components/kanban/task-detail-panel.test.tsx`

- [ ] **Step 1: Write failing planning-rule tests**

```ts
import { normalizePlanningInput } from "./task-planning";

it("treats one provided date as a single-day schedule", () => {
  expect(normalizePlanningInput({ startDate: "2026-03-30", endDate: "" })).toEqual({
    kind: "scheduled",
    plannedStartAt: "2026-03-30T09:00:00.000Z",
    plannedEndAt: "2026-03-30T18:00:00.000Z",
  });
});

it("rejects end-before-start", () => {
  expect(
    normalizePlanningInput({ startDate: "2026-04-02", endDate: "2026-04-01" })
  ).toEqual({ kind: "invalid", reason: "end_before_start" });
});

it("does not persist invalid planning edits from the detail panel", async () => {
  const user = userEvent.setup();
  const updateTask = jest.fn();

  render(<TaskDetailPanel task={task} open onOpenChange={jest.fn()} />);

  await user.clear(screen.getByLabelText("Planned Start"));
  await user.type(screen.getByLabelText("Planned Start"), "2026-04-02");
  await user.clear(screen.getByLabelText("Planned End"));
  await user.type(screen.getByLabelText("Planned End"), "2026-04-01");
  await user.click(screen.getByRole("button", { name: "Save Changes" }));

  expect(updateTask).not.toHaveBeenCalled();
  expect(screen.getByText(/end date cannot be earlier than start date/i)).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the planning tests to verify failure**

Run:
```bash
pnpm exec jest --runInBand lib/tasks/task-planning.test.ts components/kanban/task-detail-panel.test.tsx
```

Expected:
- FAIL because `task-planning.ts` does not exist yet
- FAIL because the detail panel has no invalid-date guard yet

- [ ] **Step 3: Implement the canonical planning helper and wire the current detail panel to it**

```ts
export function normalizePlanningInput({
  startDate,
  endDate,
}: {
  startDate: string;
  endDate: string;
}) {
  if (!startDate && !endDate) return { kind: "unscheduled" as const };

  const normalizedStart = startDate || endDate;
  const normalizedEnd = endDate || startDate;

  if (normalizedEnd < normalizedStart) {
    return { kind: "invalid" as const, reason: "end_before_start" as const };
  }

  return {
    kind: "scheduled" as const,
    plannedStartAt: `${normalizedStart}T09:00:00.000Z`,
    plannedEndAt: `${normalizedEnd}T18:00:00.000Z`,
  };
}
```

- [ ] **Step 4: Run the planning tests to verify pass**

Run:
```bash
pnpm exec jest --runInBand lib/tasks/task-planning.test.ts components/kanban/task-detail-panel.test.tsx
```

Expected:
- PASS for unscheduled, single-day, range, and invalid-date cases
- PASS for preserving the persisted planning window by refusing invalid saves

- [ ] **Step 5: Commit the planning helper slice**

```bash
git add lib/tasks/task-planning.ts lib/tasks/task-planning.test.ts components/kanban/task-detail-panel.tsx components/kanban/task-detail-panel.test.tsx
git commit -m "feat: normalize task planning input rules"
```

### Task 3: Harden Task And Notification Loading For The Workspace

**Files:**
- Modify: `lib/stores/task-store.ts`
- Modify: `lib/stores/task-store.test.ts`
- Modify: `components/layout/dashboard-shell.tsx`
- Modify: `components/layout/dashboard-shell.test.tsx`

- [ ] **Step 1: Write failing store and shell tests**

```ts
it("stores a retryable error when the project task load fails", async () => {
  fetchMock.mockResolvedValueOnce(mockJsonResponse({ message: "boom" }, 500));

  await useTaskStore.getState().fetchTasks("project-1");

  expect(useTaskStore.getState().error).toBe("Unable to load tasks");
});

it("fetches notifications after the dashboard shell authenticates", async () => {
  render(<DashboardShell><div>child</div></DashboardShell>);
  expect(fetchNotificationsMock).toHaveBeenCalled();
});
```

- [ ] **Step 2: Run the targeted store and shell tests to verify failure**

Run:
```bash
pnpm exec jest --runInBand lib/stores/task-store.test.ts components/layout/dashboard-shell.test.tsx
```

Expected:
- FAIL because `task-store` does not track load errors yet
- FAIL because `DashboardShell` does not fetch notifications yet

- [ ] **Step 3: Implement load-error state and notification bootstrap**

```ts
interface TaskState {
  tasks: Task[];
  loading: boolean;
  error: string | null;
}

fetchTasks: async (projectId) => {
  set({ loading: true, error: null });
  try {
    const { data } = await api.get<TaskListResponse>(`/api/v1/projects/${projectId}/tasks`, { token });
    set({ tasks: data.items.map(normalizeTask), error: null });
  } catch {
    set({ error: "Unable to load tasks" });
  } finally {
    set({ loading: false });
  }
};
```

- [ ] **Step 4: Run the targeted store and shell tests to verify pass**

Run:
```bash
pnpm exec jest --runInBand lib/stores/task-store.test.ts components/layout/dashboard-shell.test.tsx
```

Expected:
- PASS for retryable task-load error behavior
- PASS for notification bootstrap after authentication

- [ ] **Step 5: Commit the data-source slice**

```bash
git add lib/stores/task-store.ts lib/stores/task-store.test.ts components/layout/dashboard-shell.tsx components/layout/dashboard-shell.test.tsx
git commit -m "feat: harden task workspace data bootstrap"
```

### Task 4: Extract Reusable Task Detail Content

**Files:**
- Create: `components/tasks/task-detail-content.tsx`
- Modify: `components/kanban/task-detail-panel.tsx`
- Create: `components/tasks/task-context-rail.tsx`
- Create: `components/tasks/task-progress-summary.tsx`
- Create: `components/tasks/task-recent-alerts.tsx`
- Create: `components/tasks/task-context-rail.test.tsx`

- [ ] **Step 1: Write failing component tests for the context rail**

```tsx
it("renders summary mode when no task is selected", () => {
  render(
    <TaskContextRail
      selectionState="summary"
      selectedTask={null}
      counts={{ healthy: 3, warning: 1, stalled: 2, unscheduled: 4 }}
      alerts={[
        {
          id: "alert-1",
          type: "task_progress_stalled",
          title: "Task stalled: Implement timeline view",
          message: "Implement timeline view is stalled.",
          href: "/project?id=project-1#task-task-1",
          read: false,
          createdAt: "2026-03-24T12:00:00.000Z",
        },
      ]}
      realtimeState="live"
    />
  );

  expect(screen.getByText("Progress health")).toBeInTheDocument();
  expect(screen.getByText("Healthy 3")).toBeInTheDocument();
  expect(screen.getByText("Warning 1")).toBeInTheDocument();
  expect(screen.getByText("Stalled 2")).toBeInTheDocument();
  expect(screen.getByText("Unscheduled 4")).toBeInTheDocument();
  expect(screen.getByText("Task stalled: Implement timeline view")).toBeInTheDocument();
});

it("renders a hidden-by-filter message when the selected task is not visible", () => {
  render(
    <TaskContextRail
      selectionState="hidden_by_filter"
      selectedTask={task}
      counts={{ healthy: 0, warning: 1, stalled: 0, unscheduled: 0 }}
      alerts={[
        {
          id: "alert-2",
          type: "task_progress_recovered",
          title: "Task recovered: Calendar polish",
          message: "Calendar polish is healthy again.",
          href: "/project?id=project-1#task-task-2",
          read: false,
          createdAt: "2026-03-24T12:05:00.000Z",
        },
      ]}
      realtimeState="degraded"
    />
  );
  expect(screen.getByText(/outside the current filters/i)).toBeInTheDocument();
  expect(screen.getByText("Task recovered: Calendar polish")).toBeInTheDocument();
  expect(screen.getByText(/realtime updates unavailable/i)).toBeInTheDocument();
});

it("renders selected-task mode with editable and read-only task fields", () => {
  render(
    <TaskContextRail
      selectionState="selected_visible"
      selectedTask={task}
      counts={{ healthy: 0, warning: 1, stalled: 0, unscheduled: 0 }}
      alerts={[]}
      realtimeState="live"
      onTaskSave={jest.fn()}
    />
  );

  expect(screen.getByDisplayValue("Implement timeline view")).toBeInTheDocument();
  expect(screen.getByDisplayValue("2026-03-25")).toBeInTheDocument();
  expect(screen.getByText(/Alice/)).toBeInTheDocument();
  expect(screen.getByText(/No recent update/)).toBeInTheDocument();
  expect(screen.getByText(/Last activity/)).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the targeted component tests to verify failure**

Run:
```bash
pnpm exec jest --runInBand components/tasks/task-context-rail.test.tsx
```

Expected:
- FAIL because the context rail components do not exist yet

- [ ] **Step 3: Implement the reusable detail content and context rail**

```tsx
export function TaskDetailContent({
  task,
  mode,
  onSave,
  onStatusChange,
}: TaskDetailContentProps) {
  if (!task || mode === "summary") {
    return <div className="text-sm text-muted-foreground">Select a task to inspect its details.</div>;
  }

  if (mode === "hidden_by_filter") {
    return (
      <div className="flex flex-col gap-3">
        <div className="text-sm text-muted-foreground">
          This task is outside the current filters, but it remains selected.
        </div>
        <Button type="button">Clear filters</Button>
      </div>
    );
  }

  return (
    <form className="flex flex-col gap-4">
      <Input aria-label="Title" value={task.title} />
      <Select aria-label="Status" value={task.status} onValueChange={(value) => onStatusChange(task.id, value as TaskStatus)} />
      <Select aria-label="Priority" value={task.priority} />
      <Input aria-label="Planned Start" value={task.plannedStartAt?.slice(0, 10) ?? ""} />
      <Input aria-label="Planned End" value={task.plannedEndAt?.slice(0, 10) ?? ""} />
      <Badge>Assignee: {task.assigneeName ?? "Unassigned"}</Badge>
      <Badge>{task.progress?.healthStatus ?? "healthy"}</Badge>
      <div>Reason: {task.progress?.riskReason ?? "none"}</div>
      <div>Last activity: {task.progress?.lastActivityAt ?? "n/a"}</div>
      <div>Source: {task.progress?.lastActivitySource ?? "n/a"}</div>
      <Button
        type="button"
        onClick={() =>
          onSave(task.id, {
            priority,
            plannedStartAt: normalizedPlanning.plannedStartAt,
            plannedEndAt: normalizedPlanning.plannedEndAt,
          })
        }
      >
        Save Changes
      </Button>
    </form>
  );
}

export function TaskContextRail(props: TaskContextRailProps) {
  return (
    <aside className="flex flex-col gap-4">
      <TaskProgressSummary counts={props.counts} realtimeState={props.realtimeState} />
      <TaskDetailContent
        task={props.selectedTask}
        mode={props.selectionState}
        onSave={props.onTaskSave}
        onStatusChange={props.onTaskStatusChange}
      />
      <TaskRecentAlerts alerts={props.alerts} />
    </aside>
  );
}
```

- [ ] **Step 4: Run the targeted component tests to verify pass**

Run:
```bash
pnpm exec jest --runInBand components/tasks/task-context-rail.test.tsx
```

Expected:
- PASS for summary mode
- PASS for hidden-by-filter mode
- PASS for degraded realtime indicator rendering

- [ ] **Step 5: Commit the context rail component slice**

```bash
git add components/tasks/task-detail-content.tsx components/kanban/task-detail-panel.tsx components/tasks/task-context-rail.tsx components/tasks/task-progress-summary.tsx components/tasks/task-recent-alerts.tsx components/tasks/task-context-rail.test.tsx
git commit -m "feat: add task workspace context rail components"
```

### Task 5: Integrate The Unified Workspace Layout And Realtime States

**Files:**
- Create: `components/tasks/task-workspace-main.tsx`
- Modify: `components/tasks/project-task-workspace.tsx`
- Modify: `components/tasks/project-task-workspace.test.tsx`
- Modify: `app/(dashboard)/project/page.tsx`
- Modify: `lib/stores/ws-store.test.ts`

- [ ] **Step 1: Write failing integration tests for the workspace shell**

```tsx
it("shows a retryable load error and keeps the workspace shell mounted", () => {
  render(
    <ProjectTaskWorkspace
      tasks={[]}
      loading={false}
      error="Unable to load tasks"
      realtimeConnected={true}
      notifications={[
        {
          id: "alert-1",
          type: "task_progress_stalled",
          title: "Task stalled: Implement timeline view",
          message: "Implement timeline view is stalled.",
          href: "/project?id=project-1#task-task-1",
          read: false,
          createdAt: "2026-03-24T12:00:00.000Z",
        },
      ]}
      selectedTask={null}
      onRetry={jest.fn()}
      onTaskOpen={jest.fn()}
      onTaskStatusChange={jest.fn()}
      onTaskScheduleChange={jest.fn()}
      onTaskSave={jest.fn()}
    />
  );

  expect(screen.getByText("Unable to load tasks")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
});

it("shows degraded realtime while direct task actions still render", () => {
  render(
    <ProjectTaskWorkspace
      tasks={[task]}
      loading={false}
      error={null}
      realtimeConnected={false}
      notifications={[]}
      selectedTask={task}
      onRetry={jest.fn()}
      onTaskOpen={jest.fn()}
      onTaskStatusChange={jest.fn()}
      onTaskScheduleChange={jest.fn()}
      onTaskSave={jest.fn()}
    />
  );
  expect(screen.getByText(/realtime updates unavailable/i)).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "Board" })).toBeInTheDocument();
});

it("clears the invalid selection when the selected task disappears from the active project set", () => {
  useTaskWorkspaceStore.setState({ selectedTaskId: "task-gone" });

  render(
    <ProjectTaskWorkspace
      tasks={[task]}
      loading={false}
      error={null}
      realtimeConnected={true}
      notifications={[]}
      selectedTask={null}
      onRetry={jest.fn()}
      onTaskOpen={jest.fn()}
      onTaskStatusChange={jest.fn()}
      onTaskScheduleChange={jest.fn()}
      onTaskSave={jest.fn()}
    />
  );

  expect(useTaskWorkspaceStore.getState().selectedTaskId).toBeNull();
});
```

- [ ] **Step 2: Run the workspace integration tests to verify failure**

Run:
```bash
pnpm exec jest --runInBand components/tasks/project-task-workspace.test.tsx lib/stores/ws-store.test.ts
```

Expected:
- FAIL because the workspace does not accept the new rail/realtime props yet
- FAIL because degraded realtime is not rendered

- [ ] **Step 3: Implement the unified layout in the project page and workspace**

```tsx
useEffect(() => {
  if (selectedTaskId && !tasks.some((task) => task.id === selectedTaskId)) {
    selectTask(null);
  }
}, [selectTask, selectedTaskId, tasks]);

<div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
  <TaskWorkspaceMain
    tasks={tasks}
    loading={loading}
    error={error}
    onRetry={onRetry}
    onTaskOpen={onTaskOpen}
    onTaskStatusChange={onTaskStatusChange}
    onTaskScheduleChange={onTaskScheduleChange}
  />
  <TaskContextRail
    selectionState={rail.selectionState}
    selectedTask={rail.selectedTask}
    counts={rail.counts}
    alerts={rail.alerts}
    realtimeState={connected ? "live" : "degraded"}
    onTaskSave={onTaskSave}
    onTaskStatusChange={onTaskStatusChange}
  />
</div>
```

- [ ] **Step 4: Run the workspace integration tests to verify pass**

Run:
```bash
pnpm exec jest --runInBand components/tasks/project-task-workspace.test.tsx lib/stores/ws-store.test.ts
```

Expected:
- PASS for retryable load errors
- PASS for hidden-by-filter context rail handling
- PASS for degraded realtime messaging without losing task actions

- [ ] **Step 5: Run the focused end-to-end verification set**

Run:
```bash
pnpm exec jest --runInBand lib/stores/task-workspace-store.test.ts lib/tasks/task-context-rail.test.ts lib/tasks/task-planning.test.ts lib/stores/task-store.test.ts components/layout/dashboard-shell.test.tsx components/tasks/task-context-rail.test.tsx components/tasks/project-task-workspace.test.tsx lib/stores/ws-store.test.ts
pnpm lint app/(dashboard)/project/page.tsx components/tasks/project-task-workspace.tsx components/tasks/task-context-rail.tsx components/tasks/task-detail-content.tsx components/kanban/task-detail-panel.tsx lib/tasks/task-context-rail.ts lib/tasks/task-planning.ts lib/stores/task-store.ts lib/stores/task-workspace-store.ts components/layout/dashboard-shell.tsx
```

Expected:
- all targeted Jest suites PASS
- ESLint returns no errors for touched task-workspace files

- [ ] **Step 6: Commit the workspace integration slice**

```bash
git add app/(dashboard)/project/page.tsx components/tasks/task-workspace-main.tsx components/tasks/project-task-workspace.tsx components/tasks/project-task-workspace.test.tsx lib/stores/ws-store.test.ts
git commit -m "feat: integrate task workspace p0 context rail"
```

### Task 6: Final Regression Pass

**Files:**
- Modify: none unless verification finds a defect

- [ ] **Step 1: Run the full frontend regression command set**

Run:
```bash
pnpm test -- --runInBand
pnpm build
```

Expected:
- Jest suite passes without introducing new task-workspace regressions
- Next.js production build succeeds

- [ ] **Step 2: If a regression appears, fix the smallest failing slice and rerun only the affected command before repeating the full pass**

```bash
pnpm exec jest --runInBand components/tasks/project-task-workspace.test.tsx
pnpm exec jest --runInBand components/tasks/task-context-rail.test.tsx
pnpm exec jest --runInBand components/kanban/task-detail-panel.test.tsx
pnpm exec jest --runInBand lib/tasks/task-planning.test.ts
pnpm exec jest --runInBand lib/tasks/task-context-rail.test.ts
pnpm exec jest --runInBand lib/stores/task-store.test.ts
pnpm exec jest --runInBand lib/stores/task-workspace-store.test.ts
pnpm exec jest --runInBand lib/stores/ws-store.test.ts
pnpm exec jest --runInBand components/layout/dashboard-shell.test.tsx
pnpm build
```

- [ ] **Step 3: Commit the verification pass if fixes were required**

```bash
git add app/(dashboard)/project/page.tsx components/layout/dashboard-shell.tsx components/layout/dashboard-shell.test.tsx components/kanban/task-detail-panel.tsx components/kanban/task-detail-panel.test.tsx components/tasks/project-task-workspace.tsx components/tasks/project-task-workspace.test.tsx components/tasks/task-context-rail.tsx components/tasks/task-context-rail.test.tsx components/tasks/task-detail-content.tsx components/tasks/task-progress-summary.tsx components/tasks/task-recent-alerts.tsx components/tasks/task-workspace-main.tsx lib/tasks/task-context-rail.ts lib/tasks/task-context-rail.test.ts lib/tasks/task-planning.ts lib/tasks/task-planning.test.ts lib/stores/task-store.ts lib/stores/task-store.test.ts lib/stores/task-workspace-store.ts lib/stores/task-workspace-store.test.ts lib/stores/ws-store.test.ts
git commit -m "fix: close task workspace p0 verification regressions"
```
