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
import { useSkillsStore, type SkillsVerifyResult } from "./skills-store";

type MockSkillsApiClient = {
  get: jest.Mock;
  post: jest.Mock;
};

function makeApiClient(): MockSkillsApiClient {
  return {
    get: jest.fn(),
    post: jest.fn(),
  };
}

describe("useSkillsStore", () => {
  const mockCreateApiClient = createApiClient as jest.Mock;
  const mockGetAuthState = useAuthStore.getState as unknown as jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mockGetAuthState.mockReturnValue({ accessToken: "test-token", token: null });
    useSkillsStore.setState({
      items: [],
      selectedSkill: null,
      loading: false,
      detailLoading: false,
      actionLoading: false,
      error: null,
      filters: {
        family: "all",
        status: "all",
        query: "",
      },
      lastVerificationResult: null,
    });
  });

  it("stores verifier-grade results after verifySkills resolves", async () => {
    const api = makeApiClient();
    const verifyPayload: SkillsVerifyResult = {
      ok: false,
      results: [
        {
          skillId: ".agents/skills/rogue-helper",
          family: "repo-assistant",
          status: "blocked",
          issues: [
            {
              code: "unregistered_skill_package",
              message: "unregistered skill package discovered at .agents/skills/rogue-helper/SKILL.md",
              targetPath: ".agents/skills/rogue-helper/SKILL.md",
              family: "repo-assistant",
              sourceType: "repo-authored",
            },
          ],
        },
      ],
    };
    api.post.mockResolvedValueOnce({ data: verifyPayload });
    api.get.mockResolvedValueOnce({
      data: {
        items: [],
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    await expect(useSkillsStore.getState().verifySkills()).resolves.toEqual(verifyPayload);

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/skills/verify",
      {},
      { token: "test-token" },
    );
    expect(useSkillsStore.getState()).toMatchObject({
      lastVerificationResult: verifyPayload,
      actionLoading: false,
    });
  });
});
