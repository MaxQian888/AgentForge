jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(),
  },
}));

import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import { useRoleStore, type RoleManifest } from "./role-store";

type MockRoleApiClient = {
  get: jest.Mock;
  post: jest.Mock;
  put: jest.Mock;
  delete: jest.Mock;
};

function makeApiClient(): MockRoleApiClient {
  return {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn(),
  };
}

function makeRole(id = "frontend-developer"): RoleManifest {
  return {
    apiVersion: "agentforge/v1",
    kind: "Role",
    metadata: {
      id,
      name: "Frontend Developer",
      version: "1.0.0",
      description: "Builds UI",
      author: "AgentForge",
      tags: ["frontend"],
    },
    identity: {
      systemPrompt: "Focus on UI quality",
      persona: "Helpful",
      goals: ["Ship polished UX"],
      constraints: ["Keep tests green"],
    },
    capabilities: {
      languages: ["TypeScript"],
      frameworks: ["Next.js"],
    },
    knowledge: {
      repositories: ["app"],
      documents: ["docs/PRD.md"],
      patterns: ["responsive-layouts"],
    },
    security: {
      allowedPaths: ["app/"],
      deniedPaths: ["secrets/"],
      maxBudgetUsd: 5,
      requireReview: true,
    },
  };
}

describe("useRoleStore", () => {
  const mockCreateApiClient = createApiClient as jest.Mock;
  const mockGetAuthState = (useAuthStore.getState as unknown as jest.Mock);

  beforeEach(() => {
    jest.clearAllMocks();
    mockGetAuthState.mockReturnValue({ accessToken: "test-token", token: null });
    useRoleStore.setState({
      roles: [],
      loading: false,
      error: null,
    });
  });

  it("fetches roles with the current access token", async () => {
    const api = makeApiClient();
    api.get.mockResolvedValueOnce({
      data: [makeRole()],
    });
    mockCreateApiClient.mockReturnValue(api);

    await useRoleStore.getState().fetchRoles();

    expect(api.get).toHaveBeenCalledWith("/api/v1/roles", {
      token: "test-token",
    });
    expect(useRoleStore.getState()).toMatchObject({
      roles: [expect.objectContaining({ metadata: expect.objectContaining({ id: "frontend-developer" }) })],
      loading: false,
      error: null,
    });
  });

  it("falls back to the legacy token field when accessToken is missing", async () => {
    const api = makeApiClient();
    api.get.mockResolvedValueOnce({ data: [makeRole("reviewer")] });
    mockCreateApiClient.mockReturnValue(api);
    mockGetAuthState.mockReturnValueOnce({ accessToken: null, token: "legacy-token" });

    await useRoleStore.getState().fetchRoles();

    expect(api.get).toHaveBeenCalledWith("/api/v1/roles", {
      token: "legacy-token",
    });
  });

  it("captures fetch errors with the thrown message", async () => {
    const api = makeApiClient();
    api.get.mockRejectedValueOnce(new Error("roles endpoint unavailable"));
    mockCreateApiClient.mockReturnValue(api);

    await useRoleStore.getState().fetchRoles();

    expect(useRoleStore.getState()).toMatchObject({
      loading: false,
      error: "roles endpoint unavailable",
    });
  });

  it("creates a role and appends it to local state", async () => {
    const api = makeApiClient();
    const createdRole = makeRole("planner");
    const createPayload = { metadata: { ...createdRole.metadata } };
    api.post.mockResolvedValueOnce({ data: createdRole });
    mockCreateApiClient.mockReturnValue(api);

    await expect(useRoleStore.getState().createRole(createPayload)).resolves.toEqual(createdRole);

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/roles",
      createPayload,
      { token: "test-token" },
    );
    expect(useRoleStore.getState().roles).toEqual([
      expect.objectContaining({
        metadata: expect.objectContaining({ id: "planner" }),
      }),
    ]);
  });

  it("updates an existing role in local state", async () => {
    const api = makeApiClient();
    const originalRole = makeRole("frontend-developer");
    const updatedRole = {
      ...originalRole,
      metadata: {
        ...originalRole.metadata,
        name: "Frontend Lead",
      },
    };
    api.put.mockResolvedValueOnce({ data: updatedRole });
    mockCreateApiClient.mockReturnValue(api);
    useRoleStore.setState({ roles: [originalRole] });
    const updatePayload = {
      metadata: {
        ...originalRole.metadata,
        name: "Frontend Lead",
      },
    };

    await expect(
      useRoleStore
        .getState()
        .updateRole("frontend-developer", updatePayload),
    ).resolves.toEqual(updatedRole);

    expect(api.put).toHaveBeenCalledWith(
      "/api/v1/roles/frontend-developer",
      updatePayload,
      { token: "test-token" },
    );
    expect(useRoleStore.getState().roles).toEqual([
      expect.objectContaining({
        metadata: expect.objectContaining({ name: "Frontend Lead" }),
      }),
    ]);
  });

  it("deletes a role and removes it from local state", async () => {
    const api = makeApiClient();
    api.delete.mockResolvedValueOnce({ data: null });
    mockCreateApiClient.mockReturnValue(api);
    useRoleStore.setState({ roles: [makeRole("frontend"), makeRole("reviewer")] });

    await useRoleStore.getState().deleteRole("frontend");

    expect(api.delete).toHaveBeenCalledWith("/api/v1/roles/frontend", {
      token: "test-token",
    });
    expect(useRoleStore.getState().roles).toEqual([
      expect.objectContaining({
        metadata: expect.objectContaining({ id: "reviewer" }),
      }),
    ]);
  });

  it("returns preview payloads from the preview endpoint", async () => {
    const api = makeApiClient();
    const previewPayload = {
      draft: {
        metadata: {
          ...makeRole("preview").metadata,
          id: "",
        },
      },
    };
    const preview = {
      validationIssues: [{ field: "metadata.id", message: "required" }],
    };
    api.post.mockResolvedValueOnce({ data: preview });
    mockCreateApiClient.mockReturnValue(api);

    await expect(useRoleStore.getState().previewRole(previewPayload)).resolves.toEqual(preview);

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/roles/preview",
      previewPayload,
      { token: "test-token" },
    );
  });

  it("returns sandbox payloads from the sandbox endpoint", async () => {
    const api = makeApiClient();
    const sandbox = {
      readinessDiagnostics: [
        { code: "missing_credentials", message: "Missing key", blocking: true },
      ],
      selection: {
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
      },
    };
    api.post.mockResolvedValueOnce({ data: sandbox });
    mockCreateApiClient.mockReturnValue(api);

    await expect(
      useRoleStore.getState().sandboxRole({
        roleId: "frontend-developer",
        input: "Review the task",
        runtime: "codex",
      }),
    ).resolves.toEqual(sandbox);

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/roles/sandbox",
      {
        roleId: "frontend-developer",
        input: "Review the task",
        runtime: "codex",
      },
      { token: "test-token" },
    );
  });

  it("throws when mutating role data without authentication", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null, token: null });
    const createPayload = { metadata: { ...makeRole("planner").metadata } };

    await expect(useRoleStore.getState().createRole(createPayload)).rejects.toThrow(
      "Not authenticated",
    );

    expect(mockCreateApiClient).not.toHaveBeenCalled();
  });
});
