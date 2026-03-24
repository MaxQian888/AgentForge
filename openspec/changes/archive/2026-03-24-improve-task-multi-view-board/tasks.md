## 1. Task Planning Data And Shared State

- [x] 1.1 Extend the task data contract across database, Go models/DTOs, and `lib/stores/task-store.ts` with the minimal planning fields needed for Timeline and Calendar views.
- [x] 1.2 Update task fetch/create/update flows so planning fields, unscheduled state, and drag-driven mutations persist through the same project task APIs.
- [x] 1.3 Add a shared project task workspace state model for view mode, filters, search, and selected task context so all views consume one normalized task source.

## 2. Project Task Workspace Shell

- [x] 2.1 Refactor `app/(dashboard)/project/page.tsx` into a shared task workspace shell with view switching for Board, List, Timeline, and Calendar.
- [x] 2.2 Add shared filter/search controls and ensure they remain applied when the user switches task views.
- [x] 2.3 Keep task creation and task detail access working from the new workspace shell without breaking the existing project-scoped flow.

## 3. Multi-View Task Presentations

- [x] 3.1 Adapt the existing Board view to the shared workspace and preserve status-based drag-and-drop with inline failure handling.
- [x] 3.2 Implement a List view that exposes dense task rows with status, priority, assignee, and planning-state visibility.
- [x] 3.3 Implement a Timeline view that renders scheduled tasks on a time range, exposes unscheduled tasks explicitly, and supports drag-based rescheduling.
- [x] 3.4 Implement a Calendar view that places tasks by planning date, keeps unscheduled work visible, and supports drag-based rescheduling.

## 4. Verification

- [x] 4.1 Add or update frontend and API-facing tests covering view switching, shared filters, Board drag persistence, and Timeline/Calendar schedule updates.
- [x] 4.2 Run lint and targeted task-workspace verification for the project page, including empty-state and unscheduled-task behavior across the four views.
