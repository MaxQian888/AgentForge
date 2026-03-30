import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ProjectPage from "./page";
import {
  createDefaultTaskWorkspaceFilters,
  useTaskWorkspaceStore,
} from "@/lib/stores/task-workspace-store";

type Task = import("@/lib/stores/task-store").Task;
type Project = import("@/lib/stores/project-store").Project;

function createMockTask(
  overrides: Partial<Task> & Pick<Task, "id" | "projectId" | "title">,
): Task {
  const { id, projectId, title, ...rest } = overrides;
  return {
    id,
    projectId,
    parentId: null,
    sprintId: null,
    milestoneId: null,
    executionMode: null,
    title,
    description: "",
    status: "inbox",
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
    blockedBy: [],
    plannedStartAt: null,
    plannedEndAt: null,
    progress: null,
    createdAt: "2026-03-24T12:00:00.000Z",
    updatedAt: "2026-03-24T12:00:00.000Z",
    ...rest,
  };
}

function createMockProject(
  overrides: Partial<Project> & Pick<Project, "id" | "name">,
): Project {
  const { id, name, ...rest } = overrides;
  return {
    id,
    name,
    description: "",
    status: "active",
    taskCount: 0,
    agentCount: 0,
    createdAt: "2026-03-24T12:00:00.000Z",
    settings: {
      codingAgent: {
        runtime: "",
        provider: "",
        model: "",
      },
    },
    ...rest,
  };
}

const replace = jest.fn();
const fetchTasks = jest.fn();
const updateTask = jest.fn();
const transitionTask = jest.fn();
const assignTask = jest.fn();
const createTask = jest.fn();
const decomposeTask = jest.fn();
const deleteTask = jest.fn();
const fetchAgents = jest.fn();
const fetchMembers = jest.fn();
const fetchSprints = jest.fn();
const fetchSprintMetrics = jest.fn();
const spawnAgent = jest.fn();
const capturedMembersRefs: unknown[][] = [];
const bulkStatusResults: unknown[] = [];
const bulkAssignResults: unknown[] = [];
const bulkDeleteResults: unknown[] = [];
const searchParamsState = {
  id: "project-1" as string | null,
  member: null as string | null,
  action: null as string | null,
};

const taskState = {
  loading: false,
  error: null as string | null,
  tasks: [] as Task[],
  fetchTasks,
  updateTask,
  transitionTask,
  assignTask,
  createTask,
  decomposeTask,
  deleteTask,
};

const agentState = {
  agents: [],
  fetchAgents,
  spawnAgent,
};

const memberState = {
  membersByProject: {
    "project-1": [{ id: "member-1", name: "Alice" }],
  },
  fetchMembers,
};

const sprintState = {
  sprintsByProject: {
    "project-1": [
      {
        id: "sprint-1",
        name: "Sprint One",
        status: "active",
      },
    ],
  },
  metricsBySprintId: {
    "sprint-1": { sprint: { name: "Sprint One" } },
  },
  metricsLoadingBySprintId: {},
  fetchSprints,
  fetchSprintMetrics,
};

const notificationState = {
  notifications: [],
};

const wsState = {
  connected: true,
};

const projectState = {
  projects: [
    createMockProject({
      id: "project-1",
      name: "AgentForge",
    }),
  ] as Project[],
};

jest.mock("next/navigation", () => ({
  usePathname: () => "/project",
  useRouter: () => ({ replace }),
  useSearchParams: () => ({
    get: (key: string) =>
      key === "id"
        ? searchParamsState.id
        : key === "member"
          ? searchParamsState.member
          : key === "action"
            ? searchParamsState.action
            : null,
    toString: () => {
      const params = new URLSearchParams();
      if (searchParamsState.id) {
        params.set("id", searchParamsState.id);
      }
      if (searchParamsState.member) {
        params.set("member", searchParamsState.member);
      }
      if (searchParamsState.action) {
        params.set("action", searchParamsState.action);
      }
      return params.toString();
    },
  }),
}));

jest.mock("@/lib/stores/task-store", () => ({
  useTaskStore: (selector: (state: typeof taskState) => unknown) => selector(taskState),
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (state: typeof agentState) => unknown) => selector(agentState),
}));

jest.mock("@/lib/stores/sprint-store", () => ({
  useSprintStore: (selector: (state: typeof sprintState) => unknown) => selector(sprintState),
}));

jest.mock("@/lib/stores/member-store", () => ({
  useMemberStore: (selector: (state: typeof memberState) => unknown) => selector(memberState),
}));

jest.mock("@/lib/stores/notification-store", () => ({
  useNotificationStore: (selector: (state: typeof notificationState) => unknown) =>
    selector(notificationState),
}));

jest.mock("@/lib/stores/ws-store", () => ({
  useWSStore: (selector: (state: typeof wsState) => unknown) => selector(wsState),
}));

jest.mock("@/lib/stores/project-store", () => ({
  useProjectStore: (selector: (state: typeof projectState) => unknown) => selector(projectState),
}));

jest.mock("@/components/tasks/project-task-workspace", () => ({
  ProjectTaskWorkspace: ({
    members,
    onRetry,
    onTaskScheduleChange,
    onTaskSave,
    onCreateTask,
    onTaskAssign,
    onBulkStatusChange,
    onBulkAssign,
    onBulkDelete,
    onTaskDelete,
    onSprintFilterChange,
  }: {
    members: unknown[];
    onRetry: () => void;
    onTaskScheduleChange: (taskId: string, changes: Record<string, unknown>) => void;
    onTaskSave: (taskId: string, changes: Record<string, unknown>) => void;
    onCreateTask: () => void;
    onTaskAssign: (taskId: string, assigneeId: string, assigneeType: "human" | "agent") => Promise<void>;
    onBulkStatusChange: (ids: string[], status: string) => Promise<unknown>;
    onBulkAssign: (ids: string[], assigneeId: string, assigneeType: "human" | "agent") => Promise<unknown>;
    onBulkDelete: (ids: string[]) => Promise<unknown>;
    onTaskDelete: (taskId: string) => Promise<void>;
    onSprintFilterChange: (sprintId: string) => void;
  }) => {
    capturedMembersRefs.push(members);
    return (
      <div>
        <div>Project task workspace</div>
        <button type="button" onClick={onRetry}>
          retry-workspace
        </button>
        <button
          type="button"
          onClick={() => onTaskScheduleChange("task-1", { plannedStartAt: "2026-04-01T09:00:00.000Z" })}
        >
          update-schedule
        </button>
        <button
          type="button"
          onClick={() => onTaskSave("task-1", { title: "Retitled task" })}
        >
          save-task
        </button>
        <button type="button" onClick={onCreateTask}>
          open-create-task
        </button>
        <button
          type="button"
          onClick={() => void onTaskAssign("task-1", "member-1", "human")}
        >
          assign-task
        </button>
        <button
          type="button"
          onClick={() =>
            void onBulkStatusChange(["task-1", "task-2"], "completed").then((result) =>
              bulkStatusResults.push(result),
            )
          }
        >
          bulk-status
        </button>
        <button
          type="button"
          onClick={() =>
            void onBulkAssign(["task-1", "task-2"], "member-1", "human").then((result) =>
              bulkAssignResults.push(result),
            )
          }
        >
          bulk-assign
        </button>
        <button
          type="button"
          onClick={() =>
            void onBulkDelete(["task-1", "task-2"]).then((result) => bulkDeleteResults.push(result))
          }
        >
          bulk-delete
        </button>
        <button type="button" onClick={() => void onTaskDelete("task-1")}>
          delete-task
        </button>
        <button type="button" onClick={() => onSprintFilterChange("sprint-1")}>
          filter-sprint
        </button>
      </div>
    );
  },
}));

describe("ProjectPage", () => {
  beforeEach(() => {
    replace.mockReset();
    fetchTasks.mockReset();
    updateTask.mockReset();
    transitionTask.mockReset();
    assignTask.mockReset();
    createTask.mockReset().mockResolvedValue(undefined);
    decomposeTask.mockReset().mockResolvedValue(undefined);
    deleteTask.mockReset().mockResolvedValue(undefined);
    fetchAgents.mockReset();
    fetchMembers.mockReset();
    fetchSprints.mockReset();
    fetchSprintMetrics.mockReset();
    spawnAgent.mockReset();
    capturedMembersRefs.length = 0;
    bulkStatusResults.length = 0;
    bulkAssignResults.length = 0;
    bulkDeleteResults.length = 0;
    searchParamsState.id = "project-1";
    searchParamsState.member = null;
    searchParamsState.action = null;
    taskState.tasks = [
      createMockTask({
        id: "task-1",
        projectId: "project-1",
        title: "First task",
      }),
      createMockTask({
        id: "task-2",
        projectId: "project-1",
        title: "Second task",
      }),
    ];
    memberState.membersByProject["project-1"] = [{ id: "member-1", name: "Alice" }];
    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: null,
      contextRailDisplay: "expanded",
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
      },
    });
  });

  it("keeps project-scoped members referentially stable while member data is still empty", async () => {
    const { rerender } = render(<ProjectPage />);

    await act(async () => {
      rerender(<ProjectPage />);
    });

    expect(capturedMembersRefs).toHaveLength(2);
    expect(capturedMembersRefs[1]).toBe(capturedMembersRefs[0]);
  });

  it("redirects back to the project list when no project id is present", async () => {
    searchParamsState.id = null;

    render(<ProjectPage />);

    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith("/projects");
    });
  });

  it("ignores the legacy create-task route action until the user opens the dialog", () => {
    searchParamsState.action = "create-task";

    render(<ProjectPage />);

    expect(
      screen.queryByText("Capture the task goal and initial priority before the workspace fills in the rest.")
    ).not.toBeInTheDocument();
  });

  it("includes a dialog description for the create task modal", async () => {
    const user = userEvent.setup();
    render(<ProjectPage />);

    await user.click(screen.getByRole("button", { name: "New Task" }));

    await waitFor(() => {
      expect(
        screen.getByText("Capture the task goal and initial priority before the workspace fills in the rest.")
      ).toBeInTheDocument();
    });
  });

  it("consumes the member query parameter into the shared task workspace assignee filter", async () => {
    searchParamsState.member = "member-1";

    render(<ProjectPage />);

    await act(async () => undefined);

    expect(useTaskWorkspaceStore.getState().filters.assigneeId).toBe("member-1");
  });

  it("submits the create-task dialog with the current project context", async () => {
    const user = userEvent.setup();
    render(<ProjectPage />);

    await user.click(screen.getByRole("button", { name: "New Task" }));

    const titleInput = document.querySelector('input') as HTMLInputElement;
    const descriptionInput = document.querySelector("textarea") as HTMLTextAreaElement;
    fireEvent.change(titleInput, { target: { value: "Investigate timeline drift" } });
    fireEvent.change(descriptionInput, { target: { value: "Audit timeline estimates" } });

    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(createTask).toHaveBeenCalledWith({
        projectId: "project-1",
        title: "Investigate timeline drift",
        description: "Audit timeline estimates",
        priority: "medium",
        sprintId: null,
        parentId: null,
        labels: [],
        budgetUsd: 0,
        plannedStartAt: null,
        plannedEndAt: null,
      });
    });
  });

  it("routes workspace callbacks through the task and sprint stores", async () => {
    const user = userEvent.setup();
    transitionTask.mockImplementation((taskId: string) =>
      taskId === "task-2"
        ? Promise.reject(new Error("status blocked"))
        : Promise.resolve(undefined),
    );
    assignTask.mockImplementation((taskId: string) =>
      taskId === "task-2"
        ? Promise.reject(new Error("assign blocked"))
        : Promise.resolve(undefined),
    );
    deleteTask.mockImplementation((taskId: string) =>
      taskId === "task-2"
        ? Promise.reject(new Error("delete blocked"))
        : Promise.resolve(undefined),
    );

    render(<ProjectPage />);

    await user.click(screen.getByRole("button", { name: "retry-workspace" }));
    await user.click(screen.getByRole("button", { name: "update-schedule" }));
    await user.click(screen.getByRole("button", { name: "save-task" }));
    await user.click(screen.getByRole("button", { name: "assign-task" }));
    await user.click(screen.getByRole("button", { name: "bulk-status" }));
    await user.click(screen.getByRole("button", { name: "bulk-assign" }));
    await user.click(screen.getByRole("button", { name: "bulk-delete" }));
    await user.click(screen.getByRole("button", { name: "delete-task" }));
    await user.click(screen.getByRole("button", { name: "filter-sprint" }));
    await user.click(screen.getByRole("button", { name: "open-create-task" }));

    await waitFor(() => {
      expect(fetchTasks).toHaveBeenCalledWith("project-1");
    });
    expect(fetchMembers).toHaveBeenCalledWith("project-1");
    expect(fetchAgents).toHaveBeenCalled();
    expect(fetchSprints).toHaveBeenCalledWith("project-1");
    expect(updateTask).toHaveBeenCalledWith("task-1", {
      plannedStartAt: "2026-04-01T09:00:00.000Z",
    });
    expect(updateTask).toHaveBeenCalledWith("task-1", { title: "Retitled task" });
    expect(assignTask).toHaveBeenCalledWith("task-1", "member-1", "human", "Alice");
    expect(deleteTask).toHaveBeenCalledWith("task-1");
    expect(fetchSprintMetrics).toHaveBeenCalledWith("project-1", "sprint-1");
    expect(screen.getByText("Capture the task goal and initial priority before the workspace fills in the rest.")).toBeInTheDocument();

    await waitFor(() => {
      expect(bulkStatusResults).toEqual([
        {
          failed: [{ taskId: "task-2", message: "status blocked" }],
        },
      ]);
    });
    expect(bulkAssignResults).toEqual([
      {
        failed: [{ taskId: "task-2", message: "assign blocked" }],
      },
    ]);
    expect(bulkDeleteResults).toEqual([
      {
        failed: [{ taskId: "task-2", message: "delete blocked" }],
      },
    ]);
  });
});
