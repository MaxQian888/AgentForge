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
    sprintId: "sprint-1",
    title: "Implement timeline view",
    description: "Build the horizontal planning lane.",
    status: "in_progress",
    priority: "high",
    assigneeId: "member-1",
    assigneeType: "human",
    assigneeName: "Alice",
    cost: 2.5,
    budgetUsd: 6,
    spentUsd: 2.5,
    agentBranch: "",
    agentWorktree: "",
    agentSessionId: "",
    labels: [],
    blockedBy: [],
    plannedStartAt: "2026-03-25T09:00:00.000Z",
    plannedEndAt: "2026-03-27T18:00:00.000Z",
    createdAt: "2026-03-24T09:00:00.000Z",
    updatedAt: "2026-03-24T09:30:00.000Z",
  },
  {
    id: "task-2",
    projectId: "project-1",
    sprintId: "sprint-2",
    title: "Calendar polish",
    description: "Keep unscheduled tasks visible.",
    status: "triaged",
    priority: "medium",
    assigneeId: null,
    assigneeType: null,
    assigneeName: null,
    cost: null,
    budgetUsd: 0,
    spentUsd: 0,
    agentBranch: "",
    agentWorktree: "",
    agentSessionId: "",
    labels: [],
    blockedBy: ["task-1"],
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

  it("can focus the workspace on blocked tasks only", () => {
    const filters = createDefaultTaskWorkspaceFilters();
    filters.dependency = "blocked";

    expect(filterTasksForWorkspace(tasks, filters)).toEqual([
      expect.objectContaining({ id: "task-2" }),
    ]);
  });

  it("can scope the workspace to a single sprint", () => {
    const filters = createDefaultTaskWorkspaceFilters();
    filters.sprintId = "sprint-2";

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
