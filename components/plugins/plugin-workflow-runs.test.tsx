import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginWorkflowRuns } from "./plugin-workflow-runs";
import type { PluginRecord, WorkflowPluginRun } from "@/lib/stores/plugin-store";

const fetchWorkflowRuns = jest.fn();
const startWorkflowRun = jest.fn();

const storeState: {
  workflowRuns: Record<string, WorkflowPluginRun[]>;
  fetchWorkflowRuns: typeof fetchWorkflowRuns;
  startWorkflowRun: typeof startWorkflowRun;
} = {
  workflowRuns: {},
  fetchWorkflowRuns,
  startWorkflowRun,
};

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (
    selector: (state: typeof storeState) => unknown,
  ) => selector(storeState),
}));

jest.mock("./plugin-workflow-run-detail", () => ({
  PluginWorkflowRunDetail: ({ run }: { run: WorkflowPluginRun }) => (
    <div>Run detail for {run.id}</div>
  ),
}));

const workflowPlugin: PluginRecord = {
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "WorkflowPlugin",
  metadata: {
    id: "workflow-plugin",
    name: "Workflow Plugin",
    version: "1.0.0",
  },
  spec: {
    runtime: "wasm",
    workflow: {
      process: "sequential",
      steps: [{ id: "plan", role: "lead", action: "task" }],
    },
  },
  permissions: {},
  source: {
    type: "builtin",
  },
  lifecycle_state: "active",
  restart_count: 0,
};

const toolPlugin: PluginRecord = {
  ...workflowPlugin,
  kind: "ToolPlugin",
  spec: {
    runtime: "mcp",
  },
};

describe("PluginWorkflowRuns", () => {
  beforeEach(() => {
    storeState.workflowRuns = {};
    fetchWorkflowRuns.mockReset().mockResolvedValue(undefined);
    startWorkflowRun.mockReset().mockResolvedValue(undefined);
  });

  it("explains when workflow runs are unavailable for non-workflow plugins", () => {
    render(<PluginWorkflowRuns plugin={toolPlugin} />);

    expect(
      screen.getByText("Workflow runs are only available for WorkflowPlugin plugins."),
    ).toBeInTheDocument();
  });

  it("fetches workflow runs on mount and shows an empty state", async () => {
    render(<PluginWorkflowRuns plugin={workflowPlugin} />);

    await waitFor(() => {
      expect(fetchWorkflowRuns).toHaveBeenCalledWith("workflow-plugin");
    });
    expect(screen.getByText("No workflow runs yet.")).toBeInTheDocument();
  });

  it("validates trigger JSON, starts new runs, and expands run details", async () => {
    const user = userEvent.setup();

    storeState.workflowRuns = {
      "workflow-plugin": [
        {
          id: "run-alpha-001",
          plugin_id: "workflow-plugin",
          process: "sequential",
          status: "running",
          current_step_id: "plan",
          started_at: "2026-03-26T02:00:00.000Z",
          steps: [],
        },
      ],
    };

    render(<PluginWorkflowRuns plugin={workflowPlugin} />);

    await user.click(screen.getByRole("button", { name: "Start Run" }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByText("Trigger Payload (JSON)")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Start Run" }));
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "{bad" },
    });
    await user.click(screen.getByRole("button", { name: "Start" }));
    expect(screen.getByText("Invalid JSON trigger payload")).toBeInTheDocument();

    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: '{"source":"manual"}' },
    });
    await user.click(screen.getByRole("button", { name: "Start" }));

    await waitFor(() => {
      expect(startWorkflowRun).toHaveBeenCalledWith("workflow-plugin", {
        source: "manual",
      });
    });

    const runButton = screen.getByText("Step: plan").closest("button");
    expect(runButton).not.toBeNull();

    await user.click(runButton!);
    expect(screen.getByText("Run detail for run-alpha-001")).toBeInTheDocument();
  });
});
