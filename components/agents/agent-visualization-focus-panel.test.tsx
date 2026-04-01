jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const map: Record<string, string> = {
      "visualization.focus.title.task": "Task Context",
      "visualization.focus.title.dispatch": "Dispatch Context",
      "visualization.focus.title.runtime": "Runtime Context",
      "visualization.focus.clear": "Clear focus",
      "visualization.focus.task.summary": "{agentCount} agents, {queueCount} queue entries",
      "visualization.focus.runtime.availability.available": "Available",
      "visualization.focus.runtime.availability.unavailable": "Unavailable",
      "visualization.focus.runtime.connected": "{agentCount} agents, {dispatchCount} dispatch entries",
      "visualization.focus.loading": "Loading dispatch context...",
      "visualization.focus.empty": "No additional context available.",
      "visualization.focus.section.diagnostics": "Diagnostics",
      "visualization.focus.section.features": "Supported Features",
    };

    const template = map[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (_, token) =>
      String(values?.[token] ?? `{${token}}`),
    );
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { DispatchAttemptRecord } from "@/lib/stores/agent-store";
import type { AgentVisualizationFocus } from "./agent-visualization-model";

type AgentVisualizationFocusPanel =
  typeof import("./agent-visualization-focus-panel").AgentVisualizationFocusPanel;

let AgentVisualizationFocusPanel: AgentVisualizationFocusPanel | undefined;

beforeAll(async () => {
  const mod = await import("./agent-visualization-focus-panel").catch(() => null);
  AgentVisualizationFocusPanel = mod?.AgentVisualizationFocusPanel;
});

describe("AgentVisualizationFocusPanel", () => {
  it("renders task loading state and clear action", async () => {
    const user = userEvent.setup();
    const onClearFocus = jest.fn();
    const FocusPanel = AgentVisualizationFocusPanel;

    expect(typeof FocusPanel).toBe("function");
    const FocusPanelComponent = FocusPanel as NonNullable<
      typeof AgentVisualizationFocusPanel
    >;

    render(
      <FocusPanelComponent
        focus={
          {
            kind: "task",
            nodeId: "task:task-2",
            taskId: "task-2",
            taskTitle: "Review runtime availability",
            agentCount: 1,
            queueCount: 1,
          } as AgentVisualizationFocus
        }
        dispatchHistory={[]}
        dispatchHistoryLoading
        onClearFocus={onClearFocus}
      />,
    );

    expect(screen.getByText("Task Context")).toBeInTheDocument();
    expect(screen.getByText("Loading dispatch context...")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Clear focus" }));
    expect(onClearFocus).toHaveBeenCalled();
  });

  it("renders runtime diagnostics, supported features, and connected summaries", () => {
    const FocusPanel = AgentVisualizationFocusPanel;

    expect(typeof FocusPanel).toBe("function");
    const FocusPanelComponent = FocusPanel as NonNullable<
      typeof AgentVisualizationFocusPanel
    >;

    render(
      <FocusPanelComponent
        focus={
          {
            kind: "runtime",
            nodeId: "runtime:claude_code:anthropic:claude-sonnet-4-5",
            label: "Claude Code",
            runtime: "claude_code",
            provider: "anthropic",
            model: "claude-sonnet-4-5",
            available: false,
            diagnostics: [
              {
                code: "missing_cli",
                message: "CLI missing",
                blocking: true,
              },
            ],
            supportedFeatures: ["session_resume"],
            agentCount: 1,
            dispatchCount: 0,
          } as AgentVisualizationFocus
        }
        dispatchHistory={[] as DispatchAttemptRecord[]}
        dispatchHistoryLoading={false}
        onClearFocus={jest.fn()}
      />,
    );

    expect(screen.getByText("Runtime Context")).toBeInTheDocument();
    expect(screen.getByText("Unavailable")).toBeInTheDocument();
    expect(screen.getByText("Diagnostics")).toBeInTheDocument();
    expect(screen.getByText("CLI missing")).toBeInTheDocument();
    expect(screen.getByText("Supported Features")).toBeInTheDocument();
    expect(screen.getByText("session_resume")).toBeInTheDocument();
    expect(screen.getByText("1 agents, 0 dispatch entries")).toBeInTheDocument();
  });
});
