import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WorkflowConfigPanel } from "./workflow-config-panel";

const mockFetchWorkflow = jest.fn();
const mockUpdateWorkflow = jest.fn();

const stableConfig = {
  projectId: "proj-1",
  transitions: { inbox: ["triaged"] },
  triggers: [
    { fromStatus: "triaged", toStatus: "assigned", action: "auto_assign" },
  ],
};

const storeState: Record<string, unknown> = {
  config: stableConfig,
  loading: false,
  saving: false,
  error: null,
  recentActivityByProject: {
    "proj-1": [
      {
        taskId: "task-1",
        action: "notify",
        from: "triaged",
        to: "assigned",
        timestamp: "2026-03-24T12:00:00.000Z",
      },
    ],
  },
  fetchWorkflow: mockFetchWorkflow,
  updateWorkflow: mockUpdateWorkflow,
};

jest.mock("@/lib/stores/workflow-store", () => ({
  useWorkflowStore: (selector: (s: Record<string, unknown>) => unknown) =>
    selector(storeState),
  ALL_TASK_STATUSES: [
    "inbox",
    "triaged",
    "assigned",
    "in_progress",
    "blocked",
    "in_review",
    "changes_requested",
    "done",
    "cancelled",
    "budget_exceeded",
  ],
  TRIGGER_ACTIONS: [
    { value: "auto_assign", label: "Auto-assign agent" },
    { value: "notify", label: "Send notification" },
    { value: "dispatch_agent", label: "Dispatch agent run" },
  ],
}));

jest.mock("@/lib/stores/ws-store", () => ({
  useWSStore: (selector: (s: { connected: boolean }) => unknown) =>
    selector({ connected: false }),
}));

describe("WorkflowConfigPanel", () => {
  beforeEach(() => {
    mockFetchWorkflow.mockReset();
    mockUpdateWorkflow.mockReset().mockResolvedValue(true);
    storeState.config = stableConfig;
    storeState.loading = false;
    storeState.saving = false;
    storeState.error = null;
    storeState.recentActivityByProject = {
      "proj-1": [
        {
          taskId: "task-1",
          action: "notify",
          from: "triaged",
          to: "assigned",
          timestamp: "2026-03-24T12:00:00.000Z",
        },
      ],
    };
  });

  it("renders transition editor and trigger editor", () => {
    render(<WorkflowConfigPanel projectId="proj-1" />);

    expect(screen.getByText("Status Transitions")).toBeInTheDocument();
    expect(screen.getByText("Automation Triggers")).toBeInTheDocument();
    expect(screen.getByText("Save Workflow Config")).toBeInTheDocument();
  });

  it("fetches workflow config on mount", () => {
    render(<WorkflowConfigPanel projectId="proj-1" />);
    expect(mockFetchWorkflow).toHaveBeenCalledWith("proj-1");
  });

  it("renders existing trigger rules", () => {
    render(<WorkflowConfigPanel projectId="proj-1" />);
    expect(screen.getByText("Add trigger rule")).toBeInTheDocument();
    expect(screen.getByLabelText("Remove trigger 1")).toBeInTheDocument();
  });

  it("renders a workflow graph, recent activity, and degraded realtime state", () => {
    render(<WorkflowConfigPanel projectId="proj-1" />);

    expect(screen.getByText("Workflow graph")).toBeInTheDocument();
    expect(screen.getByText("Recent activity")).toBeInTheDocument();
    expect(screen.getByText("Realtime degraded")).toBeInTheDocument();
    expect(screen.getAllByText("triaged").length).toBeGreaterThan(0);
    expect(screen.getAllByText("assigned").length).toBeGreaterThan(0);
  });

  it("tracks draft changes, updates trigger rules, and saves workflow config", async () => {
    const user = userEvent.setup();
    render(<WorkflowConfigPanel projectId="proj-1" />);

    await user.click(
      screen.getByLabelText("Allow inbox to assigned"),
    );
    expect(screen.getByText("Draft changes")).toBeInTheDocument();
    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add trigger rule" }));
    const fromSelects = screen.getAllByLabelText(/from status/i);
    const toSelects = screen.getAllByLabelText(/to status/i);
    const actionSelects = screen.getAllByLabelText(/action/i);
    await user.selectOptions(fromSelects[1]!, "blocked");
    await user.selectOptions(toSelects[1]!, "in_review");
    await user.selectOptions(actionSelects[1]!, "dispatch_agent");
    expect(screen.getByText("2 triggers")).toBeInTheDocument();

    await user.click(screen.getByLabelText("Remove trigger 1"));
    expect(screen.getByText("1 trigger")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Save Workflow Config" }));
    expect(mockUpdateWorkflow).toHaveBeenCalledWith("proj-1", {
      transitions: expect.objectContaining({
        inbox: expect.arrayContaining(["triaged", "assigned"]),
      }),
      triggers: [
        {
          fromStatus: "blocked",
          toStatus: "in_review",
          action: "dispatch_agent",
        },
      ],
    });
  });

  it("shows loading and empty activity states", () => {
    storeState.loading = true;
    storeState.recentActivityByProject = { "proj-1": [] };

    render(<WorkflowConfigPanel projectId="proj-1" />);

    expect(screen.getByText("Loading workflow config...")).toBeInTheDocument();
    expect(screen.getByText("Loading...")).toBeInTheDocument();
    expect(
      screen.getByText("No workflow activity received yet for this project."),
    ).toBeInTheDocument();
  });
});
