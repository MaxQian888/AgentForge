import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginWorkflowRunDetail } from "./plugin-workflow-run-detail";
import type { WorkflowPluginRun } from "@/lib/stores/plugin-store";

const run: WorkflowPluginRun = {
  id: "run-12345678",
  plugin_id: "workflow-plugin",
  process: "sequential",
  status: "failed",
  current_step_id: "plan",
  started_at: "2026-03-26T00:00:00.000Z",
  completed_at: "2026-03-26T00:00:01.500Z",
  error: "Workflow halted after planner timeout",
  steps: [
    {
      step_id: "plan",
      role_id: "lead",
      action: "task",
      status: "failed",
      retry_count: 1,
      error: "Planner timed out",
      started_at: "2026-03-26T00:00:00.000Z",
      completed_at: "2026-03-26T00:00:00.500Z",
      attempts: [
        {
          attempt: 1,
          status: "failed",
          started_at: "2026-03-26T00:00:00.000Z",
          completed_at: "2026-03-26T00:00:00.250Z",
          error: "First failure",
          output: {
            reason: "boom",
          },
        },
      ],
    },
  ],
};

describe("PluginWorkflowRunDetail", () => {
  it("renders run metadata, step details, and attempts when expanded", async () => {
    const user = userEvent.setup();

    render(<PluginWorkflowRunDetail run={run} />);

    expect(screen.getByText("sequential")).toBeInTheDocument();
    expect(screen.getByText("(1.5s)")).toBeInTheDocument();
    expect(
      screen.getByText("Workflow halted after planner timeout"),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /plan/i }));

    expect(screen.getByText("Retries: 1")).toBeInTheDocument();
    expect(screen.getByText("First failure")).toBeInTheDocument();
    expect(screen.getByText("250ms")).toBeInTheDocument();
    expect(screen.getByText(/"reason": "boom"/)).toBeInTheDocument();
  });
});
