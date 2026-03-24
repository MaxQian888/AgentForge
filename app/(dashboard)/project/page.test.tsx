import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ProjectPage from "./page";

const replace = jest.fn();
const fetchTasks = jest.fn();
const updateTask = jest.fn();
const transitionTask = jest.fn();
const assignTask = jest.fn();
const fetchAgents = jest.fn();
const fetchMembers = jest.fn();
const capturedMembersRefs: unknown[][] = [];

const taskState = {
  loading: false,
  error: null,
  tasks: [],
  fetchTasks,
  updateTask,
  transitionTask,
  assignTask,
  createTask: jest.fn(),
};

const agentState = {
  agents: [],
  fetchAgents,
};

const memberState = {
  membersByProject: {},
  fetchMembers,
};

const notificationState = {
  notifications: [],
};

const wsState = {
  connected: true,
};

const projectState = {
  projects: [
    {
      id: "project-1",
      name: "AgentForge",
      description: "",
      status: "active",
      taskCount: 0,
      agentCount: 0,
      createdAt: "2026-03-24T12:00:00.000Z",
    },
  ],
};

jest.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  useSearchParams: () => ({
    get: (key: string) => (key === "id" ? "project-1" : null),
  }),
}));

jest.mock("@/lib/stores/task-store", () => ({
  useTaskStore: (selector: (state: typeof taskState) => unknown) => selector(taskState),
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (state: typeof agentState) => unknown) => selector(agentState),
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
  ProjectTaskWorkspace: ({ members }: { members: unknown[] }) => {
    capturedMembersRefs.push(members);
    return <div>Project task workspace</div>;
  },
}));

describe("ProjectPage", () => {
  beforeEach(() => {
    replace.mockReset();
    fetchTasks.mockReset();
    updateTask.mockReset();
    transitionTask.mockReset();
    assignTask.mockReset();
    fetchAgents.mockReset();
    fetchMembers.mockReset();
    capturedMembersRefs.length = 0;
  });

  it("keeps project-scoped members referentially stable while member data is still empty", async () => {
    const { rerender } = render(<ProjectPage />);

    await act(async () => {
      rerender(<ProjectPage />);
    });

    expect(capturedMembersRefs).toHaveLength(2);
    expect(capturedMembersRefs[1]).toBe(capturedMembersRefs[0]);
  });

  it("includes a dialog description for the create task modal", async () => {
    const user = userEvent.setup();
    render(<ProjectPage />);

    await user.click(screen.getByRole("button", { name: "New Task" }));

    expect(
      screen.getByText("Capture the task goal and initial priority before the workspace fills in the rest.")
    ).toBeInTheDocument();
  });
});
