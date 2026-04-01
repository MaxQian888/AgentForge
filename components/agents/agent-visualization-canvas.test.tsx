import type { ComponentType, ReactNode } from "react";

jest.mock("@xyflow/react", () => {
  return {
    ReactFlow: ({
      nodes,
      nodeTypes,
      children,
    }: {
      nodes: Array<{
        id: string;
        type?: string;
        data: unknown;
        selected?: boolean;
      }>;
      nodeTypes?: Record<string, ComponentType<unknown>>;
      children?: ReactNode;
    }) => (
      <div data-testid="react-flow">
        {nodes.map((node) => {
          const NodeComponent = nodeTypes?.[
            node.type ?? "default"
          ] as ComponentType<{
            id: string;
            data: unknown;
            selected?: boolean;
          }> | undefined;
          return NodeComponent ? (
            <NodeComponent
              key={node.id}
              id={node.id}
              data={node.data}
              selected={node.selected}
            />
          ) : (
            <div key={node.id}>{node.id}</div>
          );
        })}
        {children}
      </div>
    ),
    Background: () => <div data-testid="react-flow-background" />,
    Controls: () => <div data-testid="react-flow-controls" />,
    MiniMap: () => <div data-testid="react-flow-minimap" />,
    Panel: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
    Handle: () => <div data-testid="react-flow-handle" />,
    Position: {
      Left: "left",
      Right: "right",
    },
  };
});

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "visualization.loading": "Loading agent visualization...",
      "visualization.empty.noAgents": "No agents running. Spawn an agent from a task to get started.",
      "visualization.empty.noMatch": "No agents match the selected team member.",
      "visualization.legend.title": "Flow Legend",
      "visualization.legend.task": "Task",
      "visualization.legend.dispatch": "Dispatch",
      "visualization.legend.agent": "Agent",
      "visualization.legend.runtime": "Runtime",
      "visualization.degraded.title": "Runtime diagnostics degraded",
      "visualization.degraded.description": "Visualization is showing the latest available snapshot.",
      "visualization.focus.clear": "Clear focus",
      "status.running": "running",
      "dispatchStatus.blocked": "Blocked",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AgentVisualizationModel } from "./agent-visualization-model";

type AgentVisualizationCanvas =
  typeof import("./agent-visualization-canvas").AgentVisualizationCanvas;

let AgentVisualizationCanvas: AgentVisualizationCanvas | undefined;

beforeAll(async () => {
  const mod = await import("./agent-visualization-canvas").catch(() => null);
  AgentVisualizationCanvas = mod?.AgentVisualizationCanvas;
});

describe("AgentVisualizationCanvas", () => {
  const populatedModel: AgentVisualizationModel = {
    nodes: [
      {
        id: "task:task-2",
        type: "agentVisualization",
        position: { x: 0, y: 0 },
        data: {
          kind: "task",
          title: "Review runtime availability",
          subtitle: "task-2",
          metadata: ["1 agent", "1 queued"],
          tone: "default",
        },
      },
      {
        id: "dispatch:queue-1",
        type: "agentVisualization",
        position: { x: 1, y: 0 },
        data: {
          kind: "dispatch",
          title: "blocked",
          subtitle: "budget",
          metadata: ["high"],
          badges: ["codex"],
          tone: "danger",
        },
      },
      {
        id: "agent:agent-2",
        type: "agentVisualization",
        position: { x: 2, y: 0 },
        data: {
          kind: "agent",
          title: "Reviewer",
          subtitle: "Review runtime availability",
          metadata: ["codex", "openai / gpt-5.4"],
          badges: ["running"],
          tone: "warning",
          budgetPct: 82,
        },
      },
      {
        id: "runtime:codex:openai:gpt-5.4",
        type: "agentVisualization",
        position: { x: 3, y: 0 },
        data: {
          kind: "runtime",
          title: "Codex",
          subtitle: "codex",
          metadata: ["openai", "gpt-5.4"],
          badges: ["openai", "gpt-5.4"],
          tone: "success",
        },
      },
    ],
    edges: [],
    focusByNodeId: {},
    summary: {
      agentCount: 1,
      queueCount: 1,
      runtimeCount: 1,
      taskCount: 1,
      hasGraphData: true,
      isFiltered: false,
      isDegraded: true,
    },
  };

  it("renders degraded state, legend, and agent node selection", async () => {
    const user = userEvent.setup();
    const onSelectAgent = jest.fn();
    const onSelectVisualizationNode = jest.fn();
    const Canvas = AgentVisualizationCanvas;

    expect(typeof Canvas).toBe("function");
    const CanvasComponent = Canvas as NonNullable<typeof AgentVisualizationCanvas>;

    render(
      <CanvasComponent
        model={populatedModel}
        loading={false}
        requestedMemberId={null}
        selectedAgentId={null}
        selectedVisualizationNodeId={null}
        onSelectAgent={onSelectAgent}
        onSelectVisualizationNode={onSelectVisualizationNode}
      />,
    );

    expect(screen.getByText("Runtime diagnostics degraded")).toBeInTheDocument();
    expect(screen.getByText("Flow Legend")).toBeInTheDocument();
    expect(
      screen.getByText("task-2"),
    ).toBeInTheDocument();
    expect(screen.getByText("Reviewer")).toBeInTheDocument();
    expect(screen.getByText("Codex")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Reviewer/i }));
    expect(onSelectVisualizationNode).toHaveBeenCalledWith("agent:agent-2");

    await user.dblClick(screen.getByRole("button", { name: /Reviewer/i }));
    expect(onSelectAgent).toHaveBeenCalledWith("agent-2");

    const taskButton = screen
      .getAllByRole("button")
      .find((button) => button.textContent?.includes("task-2"));
    expect(taskButton).toBeDefined();
    await user.click(taskButton!);
    expect(onSelectVisualizationNode).toHaveBeenCalledWith("task:task-2");
  });

  it("renders a scoped empty state when the current member filter has no graph data", () => {
    const Canvas = AgentVisualizationCanvas;
    expect(typeof Canvas).toBe("function");
    const CanvasComponent = Canvas as NonNullable<typeof AgentVisualizationCanvas>;

    render(
      <CanvasComponent
        model={{
          nodes: [],
          edges: [],
          focusByNodeId: {},
          summary: {
            agentCount: 0,
            queueCount: 0,
            runtimeCount: 0,
            taskCount: 0,
            hasGraphData: false,
            isFiltered: true,
            isDegraded: false,
          },
        }}
        loading={false}
        requestedMemberId="member-2"
        selectedAgentId={null}
        selectedVisualizationNodeId={null}
        onSelectAgent={jest.fn()}
        onSelectVisualizationNode={jest.fn()}
      />,
    );

    expect(
      screen.getByText("No agents match the selected team member."),
    ).toBeInTheDocument();
  });

  it("renders an explicit loading state before graph data is available", () => {
    const Canvas = AgentVisualizationCanvas;
    expect(typeof Canvas).toBe("function");
    const CanvasComponent = Canvas as NonNullable<typeof AgentVisualizationCanvas>;

    render(
      <CanvasComponent
        model={{
          nodes: [],
          edges: [],
          focusByNodeId: {},
          summary: {
            agentCount: 0,
            queueCount: 0,
            runtimeCount: 0,
            taskCount: 0,
            hasGraphData: false,
            isFiltered: false,
            isDegraded: false,
          },
        }}
        loading
        requestedMemberId={null}
        selectedAgentId={null}
        selectedVisualizationNodeId={null}
        onSelectAgent={jest.fn()}
        onSelectVisualizationNode={jest.fn()}
      />,
    );

    expect(
      screen.getByText("Loading agent visualization..."),
    ).toBeInTheDocument();
  });
});
