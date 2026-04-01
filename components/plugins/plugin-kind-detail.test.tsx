import { render, screen } from "@testing-library/react";
import { PluginKindDetail } from "./plugin-kind-detail";
import type { PluginRecord } from "@/lib/stores/plugin-store";

function createPlugin(overrides: Partial<PluginRecord>): PluginRecord {
  return {
    apiVersion: "plugin.agentforge.dev/v1",
    kind: "ToolPlugin",
    metadata: {
      id: "plugin-id",
      name: "Plugin",
      version: "1.0.0",
      description: "Plugin description",
      tags: ["alpha"],
    },
    spec: {
      runtime: "mcp",
    },
    permissions: {},
    source: {
      type: "local",
    },
    lifecycle_state: "active",
    restart_count: 0,
    ...overrides,
  };
}

describe("PluginKindDetail", () => {
  it("renders workflow details", () => {
    render(
      <PluginKindDetail
        plugin={createPlugin({
          kind: "WorkflowPlugin",
          spec: {
            runtime: "wasm",
            workflow: {
              process: "sequential",
              roles: [{ id: "lead" }],
              steps: [
                {
                  id: "plan",
                  role: "lead",
                  action: "task",
                  next: ["review"],
                },
              ],
              triggers: [{ event: "task.created" }],
              limits: { maxRetries: 2 },
            },
          },
          roleDependencies: [
            {
              roleId: "lead",
              status: "resolved",
              blocking: false,
              references: ["workflow.roles", "steps.plan.role"],
            },
          ],
        })}
      />,
    );

    expect(screen.getByText("Process mode")).toBeInTheDocument();
    expect(screen.getByText("sequential")).toBeInTheDocument();
    expect(screen.getByText("lead")).toBeInTheDocument();
    expect(screen.getByText("Next: review")).toBeInTheDocument();
    expect(screen.getByText("task.created")).toBeInTheDocument();
    expect(screen.getByText("Max retries: 2")).toBeInTheDocument();
  });

  it("renders workflow role dependency health when bindings drift", () => {
    render(
      <PluginKindDetail
        plugin={createPlugin({
          kind: "WorkflowPlugin",
          spec: {
            runtime: "wasm",
            workflow: {
              process: "sequential",
              roles: [{ id: "reviewer" }],
              steps: [
                {
                  id: "review",
                  role: "reviewer",
                  action: "review",
                },
              ],
            },
          },
          roleDependencies: [
            {
              roleId: "reviewer",
              status: "missing",
              blocking: true,
              message: "Role reviewer no longer resolves from the authoritative role registry",
              references: ["workflow.roles", "steps.review.role"],
            },
          ],
        })}
      />,
    );

    expect(screen.getByText("Role dependency health")).toBeInTheDocument();
    expect(screen.getByText(/reviewer.*missing/i)).toBeInTheDocument();
    expect(
      screen.getByText("Role reviewer no longer resolves from the authoritative role registry"),
    ).toBeInTheDocument();
  });

  it("renders review plugin details", () => {
    render(
      <PluginKindDetail
        plugin={createPlugin({
          kind: "ReviewPlugin",
          spec: {
            runtime: "wasm",
            review: {
              entrypoint: "review.ts",
              triggers: {
                events: ["pull_request.opened"],
                filePatterns: ["**/*.ts"],
              },
              output: {
                format: "markdown",
              },
            },
          },
        })}
      />,
    );

    expect(screen.getByText("review.ts")).toBeInTheDocument();
    expect(screen.getByText("pull_request.opened")).toBeInTheDocument();
    expect(screen.getByText("**/*.ts")).toBeInTheDocument();
    expect(screen.getByText("markdown")).toBeInTheDocument();
  });

  it("renders integration plugin capabilities", () => {
    render(
      <PluginKindDetail
        plugin={createPlugin({
          kind: "IntegrationPlugin",
          spec: {
            runtime: "wasm",
            capabilities: ["im.send", "im.health"],
          },
        })}
      />,
    );

    expect(screen.getByText("Capabilities")).toBeInTheDocument();
    expect(screen.getByText("im.send")).toBeInTheDocument();
    expect(screen.getByText("im.health")).toBeInTheDocument();
  });

  it("renders role metadata when available", () => {
    render(
      <PluginKindDetail
        plugin={createPlugin({
          kind: "RolePlugin",
          metadata: {
            id: "frontend-role",
            name: "Frontend Role",
            version: "1.0.0",
            description: "Builds accessible UI",
            tags: ["frontend", "a11y"],
          },
        })}
      />,
    );

    expect(screen.getByText("frontend")).toBeInTheDocument();
    expect(screen.getByText("a11y")).toBeInTheDocument();
    expect(screen.getByText("Builds accessible UI")).toBeInTheDocument();
  });

  it("renders MCP details for tool plugins", () => {
    render(
      <PluginKindDetail
        plugin={createPlugin({
          kind: "ToolPlugin",
          runtime_metadata: {
            compatible: true,
            mcp: {
              transport: "stdio",
              tool_count: 3,
              resource_count: 2,
              prompt_count: 1,
              last_discovery_at: "2026-03-26T00:00:00.000Z",
              latest_interaction: {
                operation: "call_tool",
                status: "failed",
                target: "search",
                summary: "permission denied",
                error_message: "tool not allowed",
              },
            },
          },
        })}
      />,
    );

    expect(screen.getByText("Transport: stdio")).toBeInTheDocument();
    expect(screen.getByText("Tools: 3")).toBeInTheDocument();
    expect(screen.getByText("Resources: 2")).toBeInTheDocument();
    expect(screen.getByText("Prompts: 1")).toBeInTheDocument();
    expect(screen.getByText("Operation: call_tool")).toBeInTheDocument();
    expect(screen.getByText("Status: failed")).toBeInTheDocument();
    expect(screen.getByText("Target: search")).toBeInTheDocument();
    expect(screen.getByText("Summary: permission denied")).toBeInTheDocument();
    expect(screen.getByText("Error: tool not allowed")).toBeInTheDocument();
  });

  it("shows the generic fallback for unknown plugin kinds at runtime", () => {
    render(
      <PluginKindDetail
        plugin={
          {
            ...createPlugin({}),
            kind: "UnknownPlugin",
          } as unknown as PluginRecord
        }
      />,
    );

    expect(
      screen.getByText("No kind-specific details available for this plugin type."),
    ).toBeInTheDocument();
  });
});
