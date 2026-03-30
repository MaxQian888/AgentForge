jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(() => ({ accessToken: "test-token" })),
  },
}));

import { useProjectStore } from "./project-store";

const authStoreModule = jest.requireMock("./auth-store") as {
  useAuthStore: {
    getState: jest.Mock<{ accessToken: string | null }, []>;
  };
};

function mockJsonResponse(data: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => data,
  } as Response;
}

describe("useProjectStore", () => {
  const fetchMock = jest.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: "test-token",
    });
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

  it("falls back to a generic slug when the name has no alphanumeric characters", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse(
        {
          id: "project-2",
          name: "!!!",
          slug: "project",
          description: "",
          repoUrl: "",
          defaultBranch: "main",
          createdAt: "2026-03-24T12:00:00.000Z",
        },
        201,
      ),
    );

    await useProjectStore.getState().createProject({
      name: "!!!",
      description: "",
    });

    const [, requestInit] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(requestInit.body))).toMatchObject({
      slug: "project",
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

  it("normalizes malformed nested project settings into safe defaults", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "project-1",
          name: "Fallback project",
          description: null,
          settings: {
            codingAgent: {
              runtime: 42,
              provider: null,
              model: false,
            },
            budgetGovernance: {
              maxTaskBudgetUsd: "bad",
              autoStopOnExceed: 1,
            },
            reviewPolicy: {
              requiredLayers: ["layer-1", 7],
              enabledPluginDimensions: ["security", 9],
            },
            webhook: {
              url: 12,
              secret: null,
              events: ["created", 7],
              active: "yes",
            },
          },
          codingAgentCatalog: {
            defaultRuntime: 7,
            defaultSelection: null,
            runtimes: [
              {
                runtime: "codex",
                label: "Codex",
                defaultProvider: "openai",
                compatibleProviders: ["openai", 7],
                defaultModel: "gpt-5-codex",
                available: "yes",
                diagnostics: [{ code: 7, message: null, blocking: "true" }],
              },
            ],
          },
        },
      ]),
    );

    await useProjectStore.getState().fetchProjects();

    expect(useProjectStore.getState().projects[0]).toMatchObject({
      description: "",
      defaultBranch: "main",
      settings: {
        codingAgent: {
          runtime: "",
          provider: "",
          model: "",
        },
        budgetGovernance: {
          maxTaskBudgetUsd: 0,
          maxDailySpendUsd: 0,
          alertThresholdPercent: 80,
          autoStopOnExceed: true,
        },
        reviewPolicy: {
          autoTriggerOnPR: false,
          requiredLayers: ["layer-1", "7"],
          minRiskLevelForBlock: "",
          requireManualApproval: false,
          enabledPluginDimensions: ["security", "9"],
        },
        webhook: {
          url: "",
          secret: "",
          events: ["created", "7"],
          active: true,
        },
      },
      codingAgentCatalog: {
        defaultRuntime: "",
        defaultSelection: {
          runtime: "",
          provider: "",
          model: "",
        },
        runtimes: [
          expect.objectContaining({
            runtime: "codex",
            compatibleProviders: ["openai", "7"],
            diagnostics: [
              {
                code: "",
                message: "",
                blocking: true,
              },
            ],
          }),
        ],
      },
    });
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

    const updatedProject = await useProjectStore.getState().updateProject("project-1", {
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
    expect(updatedProject).toEqual(
      expect.objectContaining({
        settings: expect.objectContaining({
          codingAgent: expect.objectContaining({
            runtime: "opencode",
          }),
        }),
      })
    );
  });

  it("rethrows API errors from project updates so pages can render request feedback", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 422,
      json: async () => ({
        message: "Project settings validation failed",
        errors: {
          alertThresholdPercent: "must be between 0 and 100",
        },
      }),
    } as Response);

    await expect(
      useProjectStore.getState().updateProject("project-1", {
        settings: {
          codingAgent: {
            runtime: "codex",
            provider: "openai",
            model: "gpt-5-codex",
          },
        },
      })
    ).rejects.toMatchObject({
      message: "Project settings validation failed",
      status: 422,
      body: expect.objectContaining({
        errors: expect.objectContaining({
          alertThresholdPercent: "must be between 0 and 100",
        }),
      }),
    });
  });

  it("keeps or clears the current project when refreshing the project list", async () => {
    useProjectStore.setState({
      projects: [],
      currentProject: {
        id: "project-1",
        name: "Existing",
        description: "",
        status: "active",
        taskCount: 0,
        agentCount: 0,
        createdAt: "2026-03-24T12:00:00.000Z",
        settings: {
          codingAgent: {
            runtime: "",
            provider: "",
            model: "",
          },
        },
      },
      loading: false,
    });

    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "project-1",
          name: "Existing",
          description: "",
          createdAt: "2026-03-24T12:00:00.000Z",
        },
      ]),
    );

    await useProjectStore.getState().fetchProjects();
    expect(useProjectStore.getState().currentProject?.id).toBe("project-1");

    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "project-2",
          name: "Replacement",
          description: "",
          createdAt: "2026-03-24T12:00:00.000Z",
        },
      ]),
    );

    await useProjectStore.getState().fetchProjects();
    expect(useProjectStore.getState().currentProject).toBeNull();
  });

  it("selects and deletes the current project", async () => {
    useProjectStore.setState({
      projects: [
        {
          id: "project-1",
          name: "Agent Forge Alpha",
          description: "",
          status: "active",
          taskCount: 0,
          agentCount: 0,
          createdAt: "2026-03-24T12:00:00.000Z",
          settings: {
            codingAgent: {
              runtime: "",
              provider: "",
              model: "",
            },
          },
        },
      ],
      currentProject: null,
      loading: false,
    });
    fetchMock.mockResolvedValueOnce(mockJsonResponse({}));

    useProjectStore.getState().setCurrentProject("project-1");
    expect(useProjectStore.getState().currentProject?.id).toBe("project-1");

    await useProjectStore.getState().deleteProject("project-1");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:7777/api/v1/projects/project-1",
      expect.objectContaining({
        method: "DELETE",
      }),
    );
    expect(useProjectStore.getState()).toMatchObject({
      projects: [],
      currentProject: null,
    });
  });

  it("returns early without an auth token", async () => {
    authStoreModule.useAuthStore.getState.mockReturnValue({
      accessToken: null,
    });

    await useProjectStore.getState().fetchProjects();
    await useProjectStore.getState().createProject({
      name: "Skipped",
      description: "",
    });
    await expect(
      useProjectStore.getState().updateProject("project-1", {
        name: "Skipped",
      }),
    ).resolves.toBeUndefined();
    await useProjectStore.getState().deleteProject("project-1");

    expect(fetchMock).not.toHaveBeenCalled();
  });
});
