import React from "react";
import { render, screen } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { WorkflowEditor } from "./workflow-editor";

jest.mock("@xyflow/react", () => ({
  ReactFlow: ({ children }: { children: React.ReactNode }) => <div data-testid="reactflow">{children}</div>,
  Background: () => null,
  Controls: () => null,
  MiniMap: () => null,
  useReactFlow: () => ({ getNodes: () => [], getEdges: () => [], fitView: jest.fn(), screenToFlowPosition: jest.fn(), setNodes: jest.fn() }),
  ReactFlowProvider: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Handle: () => null,
  Position: { Top: "top", Bottom: "bottom" },
  MarkerType: { ArrowClosed: "arrowclosed" },
  BackgroundVariant: { Dots: "dots" },
  getBezierPath: () => ["", 0, 0],
  EdgeLabelRenderer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  applyNodeChanges: (_changes: unknown, nodes: unknown) => nodes,
  applyEdgeChanges: (_changes: unknown, edges: unknown) => edges,
}));

const mockDefinition = {
  id: "wf-1",
  projectId: "p-1",
  name: "Test Workflow",
  description: "A test",
  status: "active",
  category: "user",
  nodes: [],
  edges: [],
  version: 1,
  createdAt: "2026-01-01",
  updatedAt: "2026-01-01",
};

function renderEditor(props?: Partial<React.ComponentProps<typeof WorkflowEditor>>) {
  return render(
    <TooltipProvider>
      <WorkflowEditor
        definition={mockDefinition}
        onSave={jest.fn()}
        onExecute={jest.fn()}
        {...props}
      />
    </TooltipProvider>
  );
}

describe("WorkflowEditor", () => {
  it("renders toolbar with workflow name", () => {
    renderEditor();
    expect(screen.getByDisplayValue("Test Workflow")).toBeInTheDocument();
  });

  it("renders the canvas area", () => {
    renderEditor();
    expect(screen.getByTestId("reactflow")).toBeInTheDocument();
  });
});
