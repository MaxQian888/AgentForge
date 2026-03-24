import type { Task } from "@/lib/stores/task-store";
import {
  createDefaultTaskWorkspaceFilters,
  filterTasksForWorkspace,
  getRescheduledPlanningWindow,
} from "./task-workspace";

const tasks: Task[] = [
  {
    id: "task-1",
    projectId: "project-1",
    title: "Implement timeline view",
    description: "Build the horizontal planning lane.",
    status: "in_progress",
    priority: "high",
    assigneeId: "member-1",
    assigneeType: "human",
    assigneeName: "Alice",
    cost: 2.5,
    spentUsd: 2.5,
    plannedStartAt: "2026-03-25T09:00:00.000Z",
    plannedEndAt: "2026-03-27T18:00:00.000Z",
    createdAt: "2026-03-24T09:00:00.000Z",
    updatedAt: "2026-03-24T09:30:00.000Z",
  },
  {
    id: "task-2",
    projectId: "project-1",
    title: "Calendar polish",
    description: "Keep unscheduled tasks visible.",
    status: "triaged",
    priority: "medium",
    assigneeId: null,
    assigneeType: null,
    assigneeName: null,
    cost: null,
    spentUsd: 0,
    plannedStartAt: null,
    plannedEndAt: null,
    createdAt: "2026-03-24T10:00:00.000Z",
    updatedAt: "2026-03-24T10:15:00.000Z",
  },
];

describe("task workspace helpers", () => {
  it("filters tasks consistently across search and planning state", () => {
    const filters = createDefaultTaskWorkspaceFilters();
    filters.search = "calendar";
    filters.planning = "unscheduled";

    expect(filterTasksForWorkspace(tasks, filters)).toEqual([
      expect.objectContaining({ id: "task-2" }),
    ]);
  });

  it("preserves task duration when rescheduling onto a new day", () => {
    expect(
      getRescheduledPlanningWindow(
        tasks[0],
        "2026-03-30"
      )
    ).toEqual({
      plannedStartAt: "2026-03-30T09:00:00.000Z",
      plannedEndAt: "2026-04-01T18:00:00.000Z",
    });
  });
});
