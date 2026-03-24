import { render, screen } from "@testing-library/react";
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
});
