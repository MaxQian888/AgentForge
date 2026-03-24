jest.mock("@/lib/stores/auth-store", () => ({
  useAuthStore: {
    getState: () => ({ token: "test-token" }),
  },
}));

import { useMemberStore } from "./member-store";

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
          email: "",
          avatarUrl: "",
          skills: ["review"],
          isActive: false,
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
      status: "inactive",
      typeLabel: "Agent",
    });
  });
});
