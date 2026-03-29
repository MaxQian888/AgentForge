jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ token: "test-token" }),
  },
}));

import { useMemberStore } from "./member-store";
import type { CreateMemberInput, UpdateMemberInput } from "./member-store";

describe("useMemberStore", () => {
  const fetchMock = jest.fn();
  const mockJsonResponse = (data: unknown) =>
    ({
      ok: true,
      status: 200,
      json: async () => data,
    }) as Response;

  beforeEach(() => {
    fetchMock.mockReset();
    global.fetch = fetchMock as unknown as typeof fetch;
    useMemberStore.setState({
      membersByProject: {},
      loadingByProject: {},
      errorByProject: {},
    });
  });

  it("fetches and normalizes members for a project", async () => {
    fetchMock.mockResolvedValueOnce(
      mockJsonResponse([
        {
          id: "member-1",
          projectId: "project-1",
          name: "Alice",
          type: "human",
          role: "frontend-developer",
          status: "active",
          email: "alice@example.com",
          avatarUrl: "",
          skills: ["react", "testing"],
          isActive: true,
          createdAt: "2026-03-20T10:00:00.000Z",
        },
        {
          id: "member-2",
          projectId: "project-1",
          name: "Review Bot",
          type: "agent",
          role: "code-reviewer",
          status: "suspended",
          email: "",
          imPlatform: "feishu",
          imUserId: "ou_review_bot",
          avatarUrl: "",
          agentConfig: JSON.stringify({
            roleId: "frontend-developer",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5-codex",
          }),
          skills: ["review"],
          isActive: true,
          createdAt: "2026-03-20T10:00:00.000Z",
        },
      ])
    );

    await useMemberStore.getState().fetchMembers("project-1");

    const members = useMemberStore.getState().membersByProject["project-1"];
    expect(members).toHaveLength(2);
    expect(members[0]).toMatchObject({
      id: "member-1",
      status: "active",
      typeLabel: "Human",
    });
    expect(members[1]).toMatchObject({
      id: "member-2",
      status: "suspended",
      isActive: false,
      typeLabel: "Agent",
      imPlatform: "feishu",
      imUserId: "ou_review_bot",
      roleBindingLabel: "frontend-developer",
      readinessState: "ready",
      agentProfile: expect.objectContaining({
        runtime: "codex",
        provider: "openai",
      }),
    });
  });

  it("serializes and persists agent profile changes during create and update", async () => {
    fetchMock
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "member-2",
          projectId: "project-1",
          name: "Review Bot",
          type: "agent",
          role: "code-reviewer",
          status: "inactive",
          email: "",
          imPlatform: "feishu",
          imUserId: "ou_review_bot",
          avatarUrl: "",
          agentConfig: JSON.stringify({
            roleId: "frontend-developer",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5-codex",
            maxBudgetUsd: 6.5,
          }),
          skills: ["review", "typescript"],
          isActive: true,
          createdAt: "2026-03-20T10:00:00.000Z",
        }),
      )
      .mockResolvedValueOnce(
        mockJsonResponse({
          id: "member-2",
          projectId: "project-1",
          name: "Review Bot",
          type: "agent",
          role: "code-reviewer",
          status: "suspended",
          email: "",
          imPlatform: "slack",
          imUserId: "U12345",
          avatarUrl: "",
          agentConfig: JSON.stringify({
            roleId: "frontend-developer",
            runtime: "codex",
            provider: "openai",
            model: "gpt-5-codex",
            maxBudgetUsd: 7,
            notes: "keep reviews concise",
          }),
          skills: ["review", "typescript", "security"],
          isActive: true,
          createdAt: "2026-03-20T10:00:00.000Z",
        }),
      );

    const createInput: CreateMemberInput = {
      name: "Review Bot",
      type: "agent",
      role: "code-reviewer",
      status: "inactive",
      imPlatform: "feishu",
      imUserId: "ou_review_bot",
      skills: ["review", "typescript"],
      agentProfile: {
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: "6.5",
        notes: "",
      },
    };

    const updateInput: UpdateMemberInput = {
      status: "suspended",
      imPlatform: "slack",
      imUserId: "U12345",
      skills: ["review", "typescript", "security"],
      agentProfile: {
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: "7",
        notes: "keep reviews concise",
      },
    };

    await useMemberStore.getState().createMember("project-1", createInput);

    await useMemberStore.getState().updateMember("member-2", "project-1", updateInput);

    const createBody = JSON.parse((fetchMock.mock.calls[0]?.[1] as RequestInit)?.body as string);
    expect(createBody).toMatchObject({
      type: "agent",
      status: "inactive",
      imPlatform: "feishu",
      imUserId: "ou_review_bot",
      skills: ["review", "typescript"],
      agentConfig: JSON.stringify({
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: 6.5,
      }),
    });

    const updateBody = JSON.parse((fetchMock.mock.calls[1]?.[1] as RequestInit)?.body as string);
    expect(updateBody).toMatchObject({
      status: "suspended",
      imPlatform: "slack",
      imUserId: "U12345",
      skills: ["review", "typescript", "security"],
      agentConfig: JSON.stringify({
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: 7,
        notes: "keep reviews concise",
      }),
    });
  });
});
