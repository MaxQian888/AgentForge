import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TaskDetailPanel } from "./task-detail-panel";
import type { Task } from "@/lib/stores/task-store";

const updateTask = jest.fn();
const assignTask = jest.fn();
const transitionTask = jest.fn();
const decomposeTask = jest.fn();
const fetchDispatchPreflight = jest.fn();
const fetchDispatchHistory = jest.fn();
const fetchDocsTree = jest.fn();
const fetchEntityLinks = jest.fn();
const createEntityLink = jest.fn();
const deleteEntityLink = jest.fn();
const fetchTaskComments = jest.fn();
const createTaskComment = jest.fn();
const setTaskCommentResolved = jest.fn();
const deleteTaskComment = jest.fn();
const fetchCustomFieldDefinitions = jest.fn();
const fetchCustomFieldValues = jest.fn();
const fetchMilestones = jest.fn();

jest.mock("@/lib/stores/task-store", () => ({
  useTaskStore: (selector: (state: {
    tasks: Task[];
    updateTask: typeof updateTask;
    assignTask: typeof assignTask;
    transitionTask: typeof transitionTask;
    decomposeTask: typeof decomposeTask;
  }) => unknown) =>
    selector({
      tasks: [
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
          cost: 4.5,
          budgetUsd: 5,
          spentUsd: 4.5,
          agentBranch: "",
          agentWorktree: "",
          agentSessionId: "",
          labels: [],
          blockedBy: [],
          plannedStartAt: "2026-03-30T09:00:00.000Z",
          plannedEndAt: "2026-03-31T18:00:00.000Z",
          progress: null,
          createdAt: "2026-03-24T10:00:00.000Z",
          updatedAt: "2026-03-24T12:00:00.000Z",
        },
        {
          id: "task-2",
          projectId: "project-1",
          title: "Implement API contract",
          description: "Finish the dependency API.",
          status: "done",
          priority: "medium",
          assigneeId: "member-2",
          assigneeType: "human",
          assigneeName: "Bob",
          cost: 1.5,
          budgetUsd: 3,
          spentUsd: 1.5,
          agentBranch: "",
          agentWorktree: "",
          agentSessionId: "",
          labels: [],
          blockedBy: [],
          plannedStartAt: null,
          plannedEndAt: null,
          progress: null,
          createdAt: "2026-03-24T08:00:00.000Z",
          updatedAt: "2026-03-24T09:00:00.000Z",
        },
      ],
      updateTask,
      assignTask,
      transitionTask,
      decomposeTask,
    }),
}));

jest.mock("@/lib/stores/member-store", () => ({
  useMemberStore: (selector: (state: {
    membersByProject: Record<string, unknown[]>;
  }) => unknown) =>
    selector({
      membersByProject: {
        "project-1": [],
      },
    }),
}));

jest.mock("@/lib/stores/docs-store", () => ({
  useDocsStore: (selector: (state: {
    tree: unknown[];
    fetchTree: typeof fetchDocsTree;
  }) => unknown) =>
    selector({
      tree: [],
      fetchTree: fetchDocsTree,
    }),
  flattenDocsTree: () => [],
}));

jest.mock("@/lib/stores/entity-link-store", () => ({
  useEntityLinkStore: (selector: (state: {
    linksByEntity: Record<string, unknown[]>;
    fetchLinks: typeof fetchEntityLinks;
    createLink: typeof createEntityLink;
    deleteLink: typeof deleteEntityLink;
  }) => unknown) =>
    selector({
      linksByEntity: {},
      fetchLinks: fetchEntityLinks,
      createLink: createEntityLink,
      deleteLink: deleteEntityLink,
    }),
}));

jest.mock("@/lib/stores/task-comment-store", () => ({
  useTaskCommentStore: (selector: (state: {
    commentsByTask: Record<string, unknown[]>;
    fetchComments: typeof fetchTaskComments;
    createComment: typeof createTaskComment;
    setResolved: typeof setTaskCommentResolved;
    deleteComment: typeof deleteTaskComment;
  }) => unknown) =>
    selector({
      commentsByTask: {},
      fetchComments: fetchTaskComments,
      createComment: createTaskComment,
      setResolved: setTaskCommentResolved,
      deleteComment: deleteTaskComment,
    }),
}));

jest.mock("@/lib/stores/custom-field-store", () => ({
  useCustomFieldStore: (selector: (state: {
    definitionsByProject: Record<string, unknown[]>;
    valuesByTask: Record<string, unknown[]>;
    fetchDefinitions: typeof fetchCustomFieldDefinitions;
    fetchTaskValues: typeof fetchCustomFieldValues;
  }) => unknown) =>
    selector({
      definitionsByProject: {},
      valuesByTask: {},
      fetchDefinitions: fetchCustomFieldDefinitions,
      fetchTaskValues: fetchCustomFieldValues,
    }),
}));

jest.mock("@/lib/stores/milestone-store", () => ({
  useMilestoneStore: (selector: (state: {
    milestonesByProject: Record<string, unknown[]>;
    fetchMilestones: typeof fetchMilestones;
  }) => unknown) =>
    selector({
      milestonesByProject: {},
      fetchMilestones,
    }),
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (state: {
    agents: unknown[];
    dispatchHistoryByTask: Record<string, unknown[]>;
    fetchDispatchPreflight: typeof fetchDispatchPreflight;
    fetchDispatchHistory: typeof fetchDispatchHistory;
  }) => unknown) =>
    selector({
      agents: [],
      dispatchHistoryByTask: {},
      fetchDispatchPreflight,
      fetchDispatchHistory,
    }),
}));

const task: Task = {
  id: "task-1",
  projectId: "project-1",
  title: "Implement timeline view",
  description: "Build the horizontal planning lane.",
  status: "in_progress",
  priority: "high",
  assigneeId: "member-1",
  assigneeType: "human",
  assigneeName: "Alice",
  cost: 4.5,
  budgetUsd: 5,
  spentUsd: 4.5,
  agentBranch: "",
  agentWorktree: "",
  agentSessionId: "",
  labels: [],
  blockedBy: [],
  plannedStartAt: "2026-03-30T09:00:00.000Z",
  plannedEndAt: "2026-03-31T18:00:00.000Z",
  progress: null,
  createdAt: "2026-03-24T10:00:00.000Z",
  updatedAt: "2026-03-24T12:00:00.000Z",
};

describe("TaskDetailPanel", () => {
  beforeEach(() => {
    updateTask.mockReset();
    updateTask.mockResolvedValue(undefined);
    assignTask.mockReset();
    assignTask.mockResolvedValue(undefined);
    transitionTask.mockReset();
    transitionTask.mockResolvedValue(undefined);
    decomposeTask.mockReset();
    fetchDispatchPreflight.mockReset();
    fetchDispatchPreflight.mockResolvedValue(null);
    fetchDispatchHistory.mockReset();
    fetchDispatchHistory.mockResolvedValue(undefined);
    fetchDocsTree.mockReset();
    fetchEntityLinks.mockReset();
    createEntityLink.mockReset();
    deleteEntityLink.mockReset();
    fetchTaskComments.mockReset();
    createTaskComment.mockReset();
    setTaskCommentResolved.mockReset();
    deleteTaskComment.mockReset();
    fetchCustomFieldDefinitions.mockReset();
    fetchCustomFieldValues.mockReset();
    fetchMilestones.mockReset();
    decomposeTask.mockResolvedValue({
      summary: "Split the task into API and UI follow-ups.",
      subtasks: [
        {
          id: "task-2",
          projectId: "project-1",
          parentId: "task-1",
          executionMode: "agent",
          title: "Implement API contract",
          description: "Finish the dependency API.",
          status: "inbox",
          priority: "medium",
          assigneeId: null,
          assigneeType: null,
          assigneeName: null,
          cost: 0,
          budgetUsd: 0,
          spentUsd: 0,
          agentBranch: "",
          agentWorktree: "",
          agentSessionId: "",
          labels: [],
          blockedBy: [],
          plannedStartAt: null,
          plannedEndAt: null,
          progress: null,
          createdAt: "2026-03-24T14:00:00.000Z",
          updatedAt: "2026-03-24T14:00:00.000Z",
        },
      ],
    });
  });

  it("does not persist invalid planning edits from the detail panel", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(
      <TaskDetailPanel
        task={task}
        open
        onOpenChange={onOpenChange}
      />
    );

    const dateInputs = document.querySelectorAll<HTMLInputElement>('input[type="date"]');

    expect(dateInputs).toHaveLength(2);

    await user.clear(dateInputs[0]);
    await user.type(dateInputs[0], "2026-04-02");
    await user.clear(dateInputs[1]);
    await user.type(dateInputs[1], "2026-04-01");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    expect(updateTask).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(
      screen.getByText(/end date cannot be earlier than start date/i)
    ).toBeInTheDocument();
  });

  it("persists selected blockers from the detail panel", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(
      <TaskDetailPanel
        task={task}
        open
        onOpenChange={onOpenChange}
      />
    );

    await user.click(
      screen.getByRole("checkbox", { name: /Implement API contract/i })
    );
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    expect(updateTask).toHaveBeenCalledWith(
      "task-1",
      expect.objectContaining({
        blockedBy: ["task-2"],
      })
    );
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("triggers AI decomposition and renders the returned summary", async () => {
    const user = userEvent.setup();

    render(
      <TaskDetailPanel
        task={task}
        open
        onOpenChange={jest.fn()}
      />
    );

    await user.click(screen.getByRole("button", { name: "AI Decompose Task" }));

    expect(decomposeTask).toHaveBeenCalledWith("task-1");
    expect(
      await screen.findByText("Split the task into API and UI follow-ups.")
    ).toBeInTheDocument();
    const generatedSubtasks = screen.getByText("Generated subtasks").parentElement;

    expect(generatedSubtasks).not.toBeNull();
    expect(
      within(generatedSubtasks as HTMLElement).getByText("Implement API contract")
    ).toBeInTheDocument();
    expect(
      within(generatedSubtasks as HTMLElement).getByText("Agent-ready")
    ).toBeInTheDocument();
  });
});
