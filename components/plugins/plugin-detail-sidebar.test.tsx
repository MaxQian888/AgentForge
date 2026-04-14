import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginDetailSidebar } from "./plugin-detail-sidebar";
import type { PluginRecord } from "@/lib/stores/plugin-store";

jest.mock("./plugin-detail-overview", () => ({
  PluginDetailOverview: ({ plugin }: { plugin: PluginRecord }) => (
    <div>Overview {plugin.metadata.id}</div>
  ),
}));

jest.mock("./plugin-event-timeline", () => ({
  PluginEventTimeline: ({ pluginId }: { pluginId: string }) => (
    <div>Events {pluginId}</div>
  ),
}));

jest.mock("./plugin-kind-detail", () => ({
  PluginKindDetail: ({ plugin }: { plugin: PluginRecord }) => (
    <div>Kind {plugin.kind}</div>
  ),
}));

jest.mock("./plugin-mcp-panel", () => ({
  PluginMCPPanel: ({ plugin }: { plugin: PluginRecord }) => (
    <div>MCP {plugin.metadata.id}</div>
  ),
}));

jest.mock("./plugin-workflow-runs", () => ({
  PluginWorkflowRuns: ({ plugin }: { plugin: PluginRecord }) => (
    <div>Workflow {plugin.metadata.id}</div>
  ),
}));

const workflowPlugin: PluginRecord = {
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "WorkflowPlugin",
  metadata: {
    id: "release-workflow",
    name: "Release Workflow",
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
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "repo-search",
    name: "Repo Search",
    version: "1.0.0",
  },
  spec: {
    runtime: "mcp",
  },
  permissions: {},
  source: {
    type: "local",
    path: "/plugins/repo-search/manifest.yaml",
  },
  lifecycle_state: "active",
  runtime_metadata: {
    compatible: true,
    mcp: {
      transport: "stdio",
      tool_count: 1,
      resource_count: 0,
      prompt_count: 0,
    },
  },
  restart_count: 0,
};

describe("PluginDetailSidebar", () => {
  it("shows an empty-state prompt when no plugin is selected", () => {
    render(<PluginDetailSidebar plugin={null} />);

    expect(
      screen.getByText("Select an installed plugin to inspect operational details."),
    ).toBeInTheDocument();
  });

  it("shows kind and workflow tabs for workflow plugins", async () => {
    const user = userEvent.setup();

    render(<PluginDetailSidebar plugin={workflowPlugin} />);

    expect(screen.getByRole("tab", { name: "Details" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Events" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Contributions" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Workflow" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "MCP" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Workflow" }));
    expect(screen.getByText("Workflow release-workflow")).toBeInTheDocument();
  });

  it("shows the MCP tab for MCP-backed tool plugins", async () => {
    const user = userEvent.setup();

    render(<PluginDetailSidebar plugin={toolPlugin} />);

    expect(screen.getByRole("tab", { name: "MCP" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Workflow" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "MCP" }));
    expect(screen.getByText("MCP repo-search")).toBeInTheDocument();
  });
});
