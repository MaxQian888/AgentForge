import type { Task } from "@/lib/stores/task-store";
import {
  getTaskDependencyState,
  summarizeTaskDependencies,
} from "./task-dependencies";

const blockerTask: Task = {
  id: "task-1",
  projectId: "project-1",
  title: "Design task workspace",
  description: "Create the workspace architecture.",
  status: "in_progress",
  priority: "high",
  assigneeId: "member-1",
  assigneeType: "human",
  assigneeName: "Alice",
  cost: 1.5,
  budgetUsd: 5,
  spentUsd: 1.5,
  agentBranch: "",
  agentWorktree: "",
  agentSessionId: "",
  blockedBy: [],
  plannedStartAt: "2026-03-24T09:00:00.000Z",
  plannedEndAt: "2026-03-25T18:00:00.000Z",
  createdAt: "2026-03-24T09:00:00.000Z",
  updatedAt: "2026-03-24T09:00:00.000Z",
};

const blockedTask: Task = {
  ...blockerTask,
  id: "task-2",
  title: "Implement dependency rail",
  status: "blocked",
  assigneeId: null,
  assigneeType: null,
  assigneeName: null,
  cost: 0.5,
  spentUsd: 0.5,
  blockedBy: ["task-1"],
};

const doneBlockerTask: Task = {
  ...blockerTask,
  id: "task-3",
  title: "Ship smart assignment",
  status: "done",
};

const readyTask: Task = {
  ...blockerTask,
  id: "task-4",
  title: "Follow-up polish",
  status: "blocked",
  cost: 0,
  spentUsd: 0,
  blockedBy: ["task-3"],
};

describe("task dependency helpers", () => {
  it("summarizes blocked and ready-to-unblock tasks from dependency state", () => {
    expect(
      summarizeTaskDependencies([
        blockerTask,
        blockedTask,
        doneBlockerTask,
        readyTask,
      ])
    ).toEqual({
      blocked: 1,
      readyToUnblock: 1,
    });
  });

  it("resolves blockers and downstream tasks for a selected task", () => {
    const state = getTaskDependencyState(blockerTask, [
      blockerTask,
      blockedTask,
      doneBlockerTask,
      readyTask,
    ]);

    expect(state.blockers).toEqual([]);
    expect(state.blockedTasks).toEqual([
      expect.objectContaining({
        id: "task-2",
        title: "Implement dependency rail",
      }),
    ]);
  });

  it("marks a task ready to unblock once every blocker is done", () => {
    const state = getTaskDependencyState(readyTask, [
      blockerTask,
      blockedTask,
      doneBlockerTask,
      readyTask,
    ]);

    expect(state.state).toBe("ready_to_unblock");
    expect(state.blockers).toEqual([
      expect.objectContaining({
        id: "task-3",
        title: "Ship smart assignment",
        isComplete: true,
      }),
    ]);
  });
});
