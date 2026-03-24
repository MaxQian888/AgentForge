jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: () => ({ accessToken: "test-token" }),
  },
}));

import { useProjectStore } from "./project-store";

describe("useProjectStore", () => {
  const fetchMock = jest.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    useProjectStore.setState({
      projects: [],
      currentProject: null,
      loading: false,
    });
  });

  it("creates projects with a generated slug derived from the project name", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: async () => ({
        id: "project-1",
        name: "Agent Forge Alpha",
        slug: "agent-forge-alpha",
        description: "Main delivery stream",
        repoUrl: "",
        defaultBranch: "main",
        createdAt: "2026-03-24T12:00:00.000Z",
      }),
    } as Response);

    await useProjectStore.getState().createProject({
      name: "Agent Forge Alpha",
      description: "Main delivery stream",
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/projects",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        }),
      })
    );

    const [, requestInit] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(requestInit.body))).toEqual({
      name: "Agent Forge Alpha",
      slug: "agent-forge-alpha",
      description: "Main delivery stream",
    });
  });

  it("normalizes project settings and coding-agent catalog from the API", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => [
        {
          id: "project-1",
          name: "Agent Forge Alpha",
          slug: "agent-forge-alpha",
          description: "Main delivery stream",
          repoUrl: "https://github.com/acme/agentforge",
          defaultBranch: "main",
          settings: {
            codingAgent: {
              runtime: "codex",
              provider: "openai",
              model: "gpt-5-codex",
            },
          },
          codingAgentCatalog: {
            defaultRuntime: "claude_code",
            defaultSelection: {
              runtime: "codex",
              provider: "openai",
              model: "gpt-5-codex",
            },
            runtimes: [
              {
                runtime: "codex",
                label: "Codex",
                defaultProvider: "openai",
                compatibleProviders: ["openai", "codex"],
                defaultModel: "gpt-5-codex",
                available: true,
                diagnostics: [],
              },
            ],
          },
          createdAt: "2026-03-24T12:00:00.000Z",
        },
      ],
    } as Response);

    await useProjectStore.getState().fetchProjects();

    expect(useProjectStore.getState().projects[0]).toEqual(
      expect.objectContaining({
        settings: {
          codingAgent: {
            runtime: "codex",
            provider: "openai",
            model: "gpt-5-codex",
          },
        },
        codingAgentCatalog: expect.objectContaining({
          defaultSelection: expect.objectContaining({
            runtime: "codex",
            provider: "openai",
          }),
        }),
      })
    );
  });

  it("sends nested settings payloads when updating coding-agent defaults", async () => {
    useProjectStore.setState({
      projects: [
        {
          id: "project-1",
          name: "Agent Forge Alpha",
          description: "Main delivery stream",
          status: "active",
          taskCount: 0,
          agentCount: 0,
          createdAt: "2026-03-24T12:00:00.000Z",
          settings: {
            codingAgent: {
              runtime: "claude_code",
              provider: "anthropic",
              model: "claude-sonnet-4-5",
            },
          },
        },
      ],
      currentProject: null,
      loading: false,
    });

    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        id: "project-1",
        name: "Agent Forge Alpha",
        slug: "agent-forge-alpha",
        description: "Main delivery stream",
        repoUrl: "",
        defaultBranch: "main",
        settings: {
          codingAgent: {
            runtime: "opencode",
            provider: "opencode",
            model: "opencode-default",
          },
        },
        createdAt: "2026-03-24T12:00:00.000Z",
      }),
    } as Response);

    await useProjectStore.getState().updateProject("project-1", {
      settings: {
        codingAgent: {
          runtime: "opencode",
          provider: "opencode",
          model: "opencode-default",
        },
      },
    });

    const [, requestInit] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(requestInit.body))).toEqual({
      settings: {
        codingAgent: {
          runtime: "opencode",
          provider: "opencode",
          model: "opencode-default",
        },
      },
    });
  });
});
