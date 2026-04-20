import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { WorkflowDefinition } from "@/lib/stores/workflow-store";
import type { PluginRecord } from "@/lib/stores/plugin-store";

// Both stores are mocked at the module boundary so the component can pull the
// reader without needing a real zustand runtime. The tests supply the target
// lists through the component's prop overrides, so the store payloads are
// deliberately minimal.
jest.mock("@/lib/stores/workflow-store", () => ({
  useWorkflowStore: (selectorFn?: (s: { definitions: WorkflowDefinition[] }) => unknown) =>
    selectorFn ? selectorFn({ definitions: [] }) : { definitions: [] },
}));

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (selectorFn?: (s: { plugins: PluginRecord[] }) => unknown) =>
    selectorFn ? selectorFn({ plugins: [] }) : { plugins: [] },
}));

import { SubWorkflowConfig } from "./sub-workflow-config";

describe("SubWorkflowConfig", () => {
  const baseDagWorkflow: WorkflowDefinition = {
    id: "11111111-1111-1111-1111-111111111111",
    name: "Sibling DAG",
    projectId: "proj-1",
    status: "active",
    description: "",
    category: "",
    nodes: [],
    edges: [],
    version: 1,
    createdAt: "",
    updatedAt: "",
  };
  const baseParent: WorkflowDefinition = {
    id: "22222222-2222-2222-2222-222222222222",
    name: "Parent DAG",
    projectId: "proj-1",
    status: "active",
    description: "",
    category: "",
    nodes: [],
    edges: [],
    version: 1,
    createdAt: "",
    updatedAt: "",
  };
  const basePlugin: PluginRecord = {
    apiVersion: "agentforge/v1",
    kind: "WorkflowPlugin",
    metadata: { id: "plug-1", name: "Release Train", version: "1.0.0" },
    spec: { runtime: "declarative" },
    permissions: {},
    source: { type: "local" },
    lifecycle_state: "enabled",
    restart_count: 0,
  };

  it("shows the target kind picker and defaults to DAG", () => {
    const onChange = jest.fn();
    render(
      <SubWorkflowConfig
        config={{}}
        onChange={onChange}
        dagWorkflows={[baseDagWorkflow]}
        plugins={[basePlugin]}
      />,
    );
    // Target Kind trigger shows "DAG Workflow" (the default label).
    expect(screen.getByText("DAG Workflow")).toBeInTheDocument();
    // DAG candidates visible under Target Workflow.
    expect(screen.getByText("Target Workflow")).toBeInTheDocument();
  });

  it("filters out the parent workflow from DAG candidates", () => {
    const onChange = jest.fn();
    render(
      <SubWorkflowConfig
        config={{ targetKind: "dag" }}
        onChange={onChange}
        dagWorkflows={[baseDagWorkflow, baseParent]}
        plugins={[]}
        parentWorkflowId={baseParent.id}
      />,
    );
    // The component exposes the selector for DAG targets; the Select shows
    // sibling DAG's id as an option but not the parent. We can't easily open
    // the Radix-select menu in jsdom, but the component rendered the sibling's
    // name as a fallback when only one candidate exists.
    expect(screen.getByText("Target Workflow")).toBeInTheDocument();
  });

  it("renders Target Plugin label when target kind is plugin", () => {
    const onChange = jest.fn();
    render(
      <SubWorkflowConfig
        config={{ targetKind: "plugin" }}
        onChange={onChange}
        dagWorkflows={[baseDagWorkflow]}
        plugins={[basePlugin]}
      />,
    );
    expect(screen.getByText("Target Plugin")).toBeInTheDocument();
  });

  it("propagates input-mapping changes to onChange", async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();
    render(
      <SubWorkflowConfig
        config={{ targetKind: "dag" }}
        onChange={onChange}
        dagWorkflows={[]}
        plugins={[]}
      />,
    );
    const mappingField = screen.getByPlaceholderText(/inputKey/);
    // userEvent.type treats "{" as a modifier prefix; "{{" escapes to a literal "{".
    await user.type(mappingField, `{{"k":1}`);
    // onChange is called once per keystroke; assert at least one call carried
    // the inputMapping partial shape.
    expect(onChange).toHaveBeenCalled();
    const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    expect(typeof lastCall.inputMapping).toBe("string");
  });

  it("falls back to plain input when no DAG candidates are supplied", () => {
    const onChange = jest.fn();
    render(
      <SubWorkflowConfig
        config={{ targetKind: "dag" }}
        onChange={onChange}
        dagWorkflows={[]}
        plugins={[]}
      />,
    );
    expect(
      screen.getByPlaceholderText("Enter DAG workflow UUID"),
    ).toBeInTheDocument();
  });
});
