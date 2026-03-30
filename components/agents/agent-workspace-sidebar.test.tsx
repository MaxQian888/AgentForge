jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string | number>,
  ) => {
    const map: Record<string, string> = {
      "workspace.searchPlaceholder": "Search agents",
      "workspace.groupRunning": "Running",
      "workspace.groupPaused": "Paused",
      "workspace.groupFailed": "Failed",
      "workspace.groupStarting": "Starting",
      "workspace.groupCompleted": "Completed",
      "workspace.groupCancelled": "Cancelled",
      "workspace.groupBudgetExceeded": "Budget Exceeded",
      "empty.noMatch": "No matching agents",
      "empty.noAgents": "No agents",
    };
    if (key === "workspace.poolSummary") {
      return `${values?.active ?? 0}/${values?.max ?? 0} active, ${values?.queued ?? 0} queued`;
    }
    return map[key] ?? key;
  },
}));

jest.mock("./agent-sidebar-item", () => ({
  AgentSidebarItem: ({
    agent,
    selected,
    onSelect,
  }: {
    agent: { id: string; roleName: string; taskTitle: string };
    selected: boolean;
    onSelect: (id: string) => void;
  }) => (
    <button
      type="button"
      data-selected={selected ? "true" : "false"}
      onClick={() => onSelect(agent.id)}
    >
      {agent.roleName} - {agent.taskTitle}
    </button>
  ),
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentWorkspaceSidebar } from "./agent-workspace-sidebar";

describe("AgentWorkspaceSidebar", () => {
  it("renders pool summary, grouped agents, and selection toggles", async () => {
    const user = userEvent.setup();
    const onSelectAgent = jest.fn();

    render(
      <AgentWorkspaceSidebar
        agents={[
          {
            id: "agent-1",
            roleName: "Reviewer",
            taskTitle: "Audit release",
            status: "running",
            runtime: "codex",
          } as never,
          {
            id: "agent-2",
            roleName: "Planner",
            taskTitle: "Plan roadmap",
            status: "paused",
            runtime: "claude",
          } as never,
        ]}
        pool={{ active: 1, max: 4, available: 3, pausedResumable: 1, queued: 2 } as never}
        selectedAgentId="agent-1"
        onSelectAgent={onSelectAgent}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
        bridgeDegraded={false}
      />,
    );

    expect(screen.getByText("1/4 active, 2 queued")).toBeInTheDocument();
    expect(screen.getByText("Running (1)")).toBeInTheDocument();
    expect(screen.getByText("Paused (1)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reviewer - Audit release" })).toHaveAttribute(
      "data-selected",
      "true",
    );

    await user.click(screen.getByRole("button", { name: "Planner - Plan roadmap" }));
    expect(onSelectAgent).toHaveBeenCalledWith("agent-2");
  });

  it("filters agents by search and shows an empty search state when nothing matches", async () => {
    const user = userEvent.setup();

    render(
      <AgentWorkspaceSidebar
        agents={[
          {
            id: "agent-1",
            roleName: "Reviewer",
            taskTitle: "Audit release",
            status: "running",
            runtime: "codex",
          } as never,
        ]}
        pool={null}
        selectedAgentId={null}
        onSelectAgent={jest.fn()}
        onPause={jest.fn()}
        onResume={jest.fn()}
        onKill={jest.fn()}
        bridgeDegraded={false}
      />,
    );

    await user.type(screen.getByPlaceholderText("Search agents"), "planner");
    expect(screen.getByText("No matching agents")).toBeInTheDocument();
  });
});
