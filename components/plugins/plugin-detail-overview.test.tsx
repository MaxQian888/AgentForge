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
  last_health_at: "2026-03-26T00:00:00.000Z",
  last_error: "",
};

describe("PluginDetailOverview", () => {
  it("renders trust, release, and runtime detail sections", () => {
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
  });
});
