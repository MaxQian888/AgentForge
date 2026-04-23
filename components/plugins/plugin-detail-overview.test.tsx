import { render, screen } from "@testing-library/react";
import { PluginDetailOverview } from "./plugin-detail-overview";
import type { PluginRecord } from "@/lib/stores/plugin-store";

const plugin: PluginRecord = {
  apiVersion: "plugin.agentforge.dev/v1",
  kind: "ToolPlugin",
  metadata: {
    id: "repo-search",
    name: "Repo Search",
    version: "1.0.0",
    description: "Searches the repository",
  },
  spec: {
    runtime: "mcp",
  },
  permissions: {
    network: {
      required: true,
      domains: ["api.example.com"],
    },
  },
  source: {
    type: "catalog",
    path: "/plugins/repo-search/manifest.yaml",
    registry: "https://registry.agentforge.dev",
    entry: "repo-search",
    version: "1.0.0",
    digest: "sha256:test",
    signature: "sigstore-bundle",
    trust: {
      status: "verified",
      approvalState: "approved",
    },
    release: {
      version: "1.0.0",
      channel: "stable",
      availableVersion: "1.1.0",
      notesUrl: "https://example.com/release-notes",
    },
  },
  lifecycle_state: "active",
  runtime_host: "ts-bridge",
  restart_count: 2,
  resolved_source_path: "/plugins/repo-search/manifest.yaml",
  runtime_metadata: {
    abi_version: "v1",
    compatible: true,
  },
  builtIn: {
    official: true,
    docsRef: "docs/GO_WASM_PLUGIN_RUNTIME.md",
    verificationProfile: "go-wasm",
    starterFamily: "core-starter",
    coreFlows: ["task-delivery", "review-automation"],
    dependencyRefs: ["runtime:go-workflow", "role:code-reviewer"],
    workspaceRefs: ["workspace:workflow", "workspace:reviews"],
    availabilityStatus: "requires_configuration",
    availabilityMessage: "Built-in plugin requires configuration before activation can succeed.",
    readinessStatus: "requires_configuration",
    readinessMessage: "Built-in plugin requires configuration before activation can succeed.",
    nextStep: "Set FEISHU_APP_ID and FEISHU_APP_SECRET before activation.",
    blockingReasons: ["missing_configuration"],
    missingConfiguration: ["FEISHU_APP_ID", "FEISHU_APP_SECRET"],
    installable: true,
  },
  last_health_at: "2026-03-26T00:00:00.000Z",
  last_error: "",
};

describe("PluginDetailOverview", () => {
  it("renders trust, release, runtime detail, and built-in readiness sections", () => {
    render(<PluginDetailOverview plugin={plugin} />);

    expect(screen.getByText("Repo Search")).toBeInTheDocument();
    expect(screen.getByText("verified")).toBeInTheDocument();
    expect(screen.getByText("approved")).toBeInTheDocument();
    expect(screen.getAllByText("Update available: v1.1.0")).toHaveLength(2);
    expect(screen.getByText("Version: 1.0.0")).toBeInTheDocument();
    expect(screen.getByText("Channel: stable")).toBeInTheDocument();
    expect(screen.getByText("Runtime host")).toBeInTheDocument();
    expect(screen.getByText("ts-bridge")).toBeInTheDocument();
    expect(screen.getByText("/plugins/repo-search/manifest.yaml")).toBeInTheDocument();
    expect(screen.getByText("Registry: https://registry.agentforge.dev")).toBeInTheDocument();
    expect(screen.getByText("Entry: repo-search")).toBeInTheDocument();
    expect(screen.getByText("Requested version: 1.0.0")).toBeInTheDocument();
    expect(screen.getByText("Built-in readiness")).toBeInTheDocument();
    expect(screen.getByText("requires_configuration")).toBeInTheDocument();
    expect(screen.getByText("Built-in plugin requires configuration before activation can succeed.")).toBeInTheDocument();
    expect(screen.getByText("Next step: Set FEISHU_APP_ID and FEISHU_APP_SECRET before activation.")).toBeInTheDocument();
    expect(screen.getByText("Missing configuration: FEISHU_APP_ID, FEISHU_APP_SECRET")).toBeInTheDocument();
    expect(screen.getByText("Starter family: core-starter")).toBeInTheDocument();
    expect(screen.getByText("Core flows: task-delivery, review-automation")).toBeInTheDocument();
    expect(screen.getByText("Dependencies: runtime:go-workflow, role:code-reviewer")).toBeInTheDocument();
    expect(screen.getByText("Workspaces: workspace:workflow, workspace:reviews")).toBeInTheDocument();
  });

  it("renders marketplace provenance and a deep-link back to the marketplace workspace", () => {
    render(
      <PluginDetailOverview
        plugin={{
          ...plugin,
          source: {
            ...plugin.source,
            type: "marketplace",
            catalog: "release-train",
            ref: "1.2.3",
          },
        }}
      />,
    );

    expect(screen.getByText("Marketplace provenance")).toBeInTheDocument();
    expect(screen.getByText("Marketplace item: release-train")).toBeInTheDocument();
    expect(screen.getByText("Selected version: 1.2.3")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open in marketplace" })).toHaveAttribute(
      "href",
      "/marketplace?item=release-train",
    );
  });

  it("renders role consumers and a deep-link to the roles workspace", () => {
    render(
      <PluginDetailOverview
        plugin={{
          ...plugin,
          roleConsumers: [
            {
              roleId: "design-lead",
              roleName: "Design Lead",
              referenceType: "external",
              status: "active",
              blocking: false,
            },
          ],
        }}
      />,
    );

    expect(screen.getByText("Role consumers")).toBeInTheDocument();
    expect(screen.getByText("Design Lead (design-lead)")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open roles workspace" })).toHaveAttribute(
      "href",
      "/roles",
    );
  });
});
