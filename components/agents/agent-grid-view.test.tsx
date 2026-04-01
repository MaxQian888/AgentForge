jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const map: Record<string, string> = {
      "workspace.agentGridTitle": "Agent Pool",
      "workspace.agentGridDescription": "Live runtime inventory and status cards.",
      "workspace.emptyTitle": "No active agents yet",
      "workspace.emptyDescription": "Start your first agent from the command palette.",
      "workspace.emptyAction": "Spawn your first agent",
      "workspace.cardRuntime": "Runtime",
      "workspace.cardBudget": "Budget",
      "workspace.cardTurns": "Turns",
      "workspace.cardMemory": "Memory",
      "workspace.quickPause": "Pause",
      "workspace.quickResume": "Resume",
      "workspace.quickKill": "Kill",
      "workspace.bulkSelected": "2 selected",
      "workspace.bulkPause": "Pause Selected",
      "workspace.bulkResume": "Resume Selected",
      "workspace.bulkKill": "Terminate Selected",
      "workspace.bulkClear": "Clear Selection",
      "workspace.cardCpu": "CPU",
      "workspace.cardMemoryUsage": "Memory Usage",
      "workspace.telemetryPending": "Awaiting telemetry",
      "status.running": "running",
      "status.paused": "paused",
      "status.failed": "failed",
    };

    if (key === "workspace.cardBudgetValue") {
      return `$${values?.cost ?? 0} / $${values?.budget ?? 0}`;
    }

    return map[key] ?? key;
  },
}));

const openCommandPalette = jest.fn();

jest.mock("@/lib/stores/layout-store", () => ({
  useLayoutStore: (
    selector: (state: { openCommandPalette: typeof openCommandPalette }) => unknown,
  ) => selector({ openCommandPalette }),
}));

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentGridView } from "./agent-grid-view";

describe("AgentGridView", () => {
  beforeEach(() => {
    openCommandPalette.mockReset();
  });

  it("renders agent status cards with runtime identity and budget usage", () => {
    render(
      <AgentGridView
        agents={[
          {
            id: "agent-1",
            taskTitle: "Audit release",
            roleName: "Reviewer",
            status: "running",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
            turns: 18,
            cost: 3.2,
            budget: 10,
            memoryStatus: "available",
          } as never,
          {
            id: "agent-2",
            taskTitle: "Plan roadmap",
            roleName: "Planner",
            status: "paused",
            runtime: "claude_code",
            provider: "anthropic",
            model: "sonnet",
            turns: 7,
            cost: 1.5,
            budget: 5,
            memoryStatus: "warming",
          } as never,
        ]}
      />,
    );

    expect(screen.getByText("Agent Pool")).toBeInTheDocument();
    expect(screen.getByText("Audit release")).toBeInTheDocument();
    expect(screen.getByText("Reviewer")).toBeInTheDocument();
    expect(screen.getAllByText("codex / openai / gpt-5.4").length).toBeGreaterThan(0);
    expect(screen.getByText("$3.2 / $10")).toBeInTheDocument();
    expect(screen.getByText("available")).toBeInTheDocument();
    expect(screen.getByText("warming")).toBeInTheDocument();
  });

  it("renders a spawn call-to-action when the agent pool is empty", async () => {
    const user = userEvent.setup();

    render(<AgentGridView agents={[]} />);

    expect(screen.getByText("No active agents yet")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Spawn your first agent" }));

    expect(openCommandPalette).toHaveBeenCalledTimes(1);
  });

  it("supports pause, resume, and terminate controls with confirmation", async () => {
    const user = userEvent.setup();
    const onPause = jest.fn();
    const onResume = jest.fn();
    const onKill = jest.fn();

    render(
      <AgentGridView
        agents={[
          {
            id: "agent-1",
            taskTitle: "Audit release",
            roleName: "Reviewer",
            status: "running",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
            turns: 18,
            cost: 3.2,
            budget: 10,
            memoryStatus: "available",
          } as never,
          {
            id: "agent-2",
            taskTitle: "Plan roadmap",
            roleName: "Planner",
            status: "paused",
            runtime: "claude_code",
            provider: "anthropic",
            model: "sonnet",
            turns: 7,
            cost: 1.5,
            budget: 5,
            memoryStatus: "warming",
          } as never,
        ]}
        onPause={onPause}
        onResume={onResume}
        onKill={onKill}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Pause" }));
    expect(onPause).toHaveBeenCalledWith("agent-1");

    await user.click(screen.getByRole("button", { name: "Resume" }));
    expect(onResume).toHaveBeenCalledWith("agent-2");

    await user.click(screen.getAllByRole("button", { name: "Kill" })[0]!);
    expect(screen.getByText("Terminate agent?")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Terminate" }));
    expect(onKill).toHaveBeenCalledWith("agent-1");
  });

  it("supports shift-click multi-select and bulk pause actions", async () => {
    const user = userEvent.setup();
    const onPause = jest.fn();
    const onSelectAgent = jest.fn();

    render(
      <AgentGridView
        agents={[
          {
            id: "agent-1",
            taskTitle: "Audit release",
            roleName: "Reviewer",
            status: "running",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
            turns: 18,
            cost: 3.2,
            budget: 10,
            memoryStatus: "available",
          } as never,
          {
            id: "agent-2",
            taskTitle: "Plan roadmap",
            roleName: "Planner",
            status: "running",
            runtime: "claude_code",
            provider: "anthropic",
            model: "sonnet",
            turns: 7,
            cost: 1.5,
            budget: 5,
            memoryStatus: "warming",
          } as never,
        ]}
        onPause={onPause}
        onSelectAgent={onSelectAgent}
      />,
    );

    fireEvent.click(screen.getByTestId("agent-card-agent-1"), {
      shiftKey: true,
    });
    fireEvent.click(screen.getByTestId("agent-card-agent-2"), {
      shiftKey: true,
    });

    expect(screen.getByTestId("agent-bulk-toolbar")).toHaveTextContent("2 selected");

    await user.click(screen.getByRole("button", { name: "Pause Selected" }));

    expect(onPause).toHaveBeenCalledWith("agent-1");
    expect(onPause).toHaveBeenCalledWith("agent-2");
    expect(onSelectAgent).not.toHaveBeenCalled();
  });

  it("renders CPU and memory sparkline charts for running agents and flags high usage", () => {
    render(
      <AgentGridView
        agents={[
          {
            id: "agent-1",
            taskTitle: "Audit release",
            roleName: "Reviewer",
            status: "running",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
            turns: 18,
            cost: 3.2,
            budget: 10,
            memoryStatus: "available",
            resourceUtilization: {
              cpuPercent: 86,
              memoryPercent: 64,
              cpuHistory: [32, 44, 56, 70, 86],
              memoryHistory: [40, 42, 48, 57, 64],
            },
          } as never,
        ]}
      />,
    );

    expect(screen.getByText("CPU")).toBeInTheDocument();
    expect(screen.getByText("Memory Usage")).toBeInTheDocument();
    expect(screen.getByText("86%")).toBeInTheDocument();
    expect(screen.getByText("64%")).toBeInTheDocument();
    expect(screen.getByTestId("agent-resource-cpu-agent-1")).toHaveClass("text-amber-600");
    expect(screen.getByTestId("agent-resource-memory-agent-1")).not.toHaveClass("text-amber-600");
  });

  it("shows a telemetry placeholder when running agents do not have resource history yet", () => {
    render(
      <AgentGridView
        agents={[
          {
            id: "agent-1",
            taskTitle: "Audit release",
            roleName: "Reviewer",
            status: "running",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5.4",
            turns: 18,
            cost: 3.2,
            budget: 10,
            memoryStatus: "available",
          } as never,
        ]}
      />,
    );

    expect(screen.getByTestId("agent-resource-cpu-agent-1")).toHaveTextContent(
      "Awaiting telemetry",
    );
    expect(screen.getByTestId("agent-resource-memory-agent-1")).toHaveTextContent(
      "Awaiting telemetry",
    );
  });
});
