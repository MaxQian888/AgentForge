import React from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { WorkflowEditor } from "./workflow-editor";

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

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
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders toolbar with workflow name", () => {
    renderEditor();
    expect(screen.getByDisplayValue("Test Workflow")).toBeInTheDocument();
  });

  it("renders the canvas area", () => {
    renderEditor();
    expect(screen.getByTestId("reactflow")).toBeInTheDocument();
  });

  it("exposes Export and Import controls in the toolbar", () => {
    renderEditor();
    expect(
      screen.getByRole("button", { name: /export workflow/i })
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /import workflow/i })
    ).toBeInTheDocument();
  });

  it("triggers a download when Export is clicked", () => {
    const createObjectURL = jest.fn(() => "blob:mock");
    const revokeObjectURL = jest.fn();
    // jsdom does not implement URL.createObjectURL by default
    (global.URL as unknown as { createObjectURL: typeof createObjectURL }).createObjectURL =
      createObjectURL;
    (global.URL as unknown as { revokeObjectURL: typeof revokeObjectURL }).revokeObjectURL =
      revokeObjectURL;

    renderEditor();
    fireEvent.click(screen.getByRole("button", { name: /export workflow/i }));
    expect(createObjectURL).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:mock");
  });

  it("loads imported workflow JSON into the editor", async () => {
    renderEditor();

    const fileInput = screen.getByTestId(
      "import-workflow-file-input"
    ) as HTMLInputElement;

    const payload = {
      version: 1,
      name: "Imported Flow",
      description: "Desc",
      nodes: [{ id: "n1", type: "trigger", label: "Start", position: { x: 0, y: 0 } }],
      edges: [],
    };

    // jsdom's File polyfill lacks .text() on older versions — stub it.
    const file = new File([JSON.stringify(payload)], "wf.json", {
      type: "application/json",
    });
    Object.defineProperty(file, "text", {
      value: () => Promise.resolve(JSON.stringify(payload)),
    });

    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(screen.getByDisplayValue("Imported Flow")).toBeInTheDocument();
    });
  });
});
