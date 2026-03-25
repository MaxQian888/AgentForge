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
import { useReviewStore } from "./review-store";

type MockReviewApiClient = {
  get: jest.Mock;
  post: jest.Mock;
};

function makeApiClient(): MockReviewApiClient {
  return {
    get: jest.fn(),
    post: jest.fn(),
  };
}

describe("useReviewStore", () => {
  const mockCreateApiClient = createApiClient as jest.Mock;
  const mockGetAuthState = (useAuthStore.getState as unknown as jest.Mock);

  beforeEach(() => {
    jest.clearAllMocks();
    mockGetAuthState.mockReturnValue({ accessToken: "test-token" });
    useReviewStore.setState({
      reviewsByTask: {},
      loading: false,
      error: null,
    });
  });

  it("fetches reviews for a task and stores them by task id", async () => {
    const api = makeApiClient();
    api.get.mockResolvedValueOnce({
      data: [
        {
          id: "review-1",
          taskId: "task-1",
          prUrl: "https://example.com/pr/1",
          prNumber: 1,
          layer: 2,
          status: "completed",
          riskLevel: "medium",
          findings: [],
          summary: "Looks good",
          recommendation: "approve",
          costUsd: 0.2,
          createdAt: "2026-03-25T10:00:00.000Z",
          updatedAt: "2026-03-25T10:05:00.000Z",
        },
      ],
    });
    mockCreateApiClient.mockReturnValue(api);

    await useReviewStore.getState().fetchReviewsByTask("task-1");

    expect(api.get).toHaveBeenCalledWith("/api/v1/tasks/task-1/reviews", {
      token: "test-token",
    });
    expect(useReviewStore.getState()).toMatchObject({
      loading: false,
      error: null,
      reviewsByTask: {
        "task-1": [
          expect.objectContaining({
            id: "review-1",
            recommendation: "approve",
          }),
        ],
      },
    });
  });

  it("surfaces a fetch error when loading task reviews fails", async () => {
    const api = makeApiClient();
    api.get.mockRejectedValueOnce(new Error("boom"));
    mockCreateApiClient.mockReturnValue(api);

    await useReviewStore.getState().fetchReviewsByTask("task-1");

    expect(useReviewStore.getState()).toMatchObject({
      loading: false,
      error: "Unable to load reviews",
    });
  });

  it("triggers a review run with the canonical payload", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({ data: null });
    mockCreateApiClient.mockReturnValue(api);

    await useReviewStore.getState().triggerReview({
      taskId: "task-1",
      prUrl: "https://example.com/pr/1",
      trigger: "manual",
    });

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/reviews/trigger",
      {
        taskId: "task-1",
        prUrl: "https://example.com/pr/1",
        trigger: "manual",
      },
      { token: "test-token" },
    );
    expect(useReviewStore.getState()).toMatchObject({
      loading: false,
      error: null,
    });
  });

  it("captures approval failures without leaving loading stuck", async () => {
    const api = makeApiClient();
    api.post.mockRejectedValueOnce(new Error("reject"));
    mockCreateApiClient.mockReturnValue(api);

    await useReviewStore.getState().approveReview("review-1", "ship it");

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/reviews/review-1/approve",
      { comment: "ship it" },
      { token: "test-token" },
    );
    expect(useReviewStore.getState()).toMatchObject({
      loading: false,
      error: "Unable to approve review",
    });
  });

  it("submits review rejection reasons to the canonical endpoint", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({ data: null });
    mockCreateApiClient.mockReturnValue(api);

    await useReviewStore
      .getState()
      .rejectReview("review-1", "security issue", "needs patch");

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/reviews/review-1/reject",
      { reason: "security issue", comment: "needs patch" },
      { token: "test-token" },
    );
    expect(useReviewStore.getState()).toMatchObject({
      loading: false,
      error: null,
    });
  });

  it("returns early when review actions are attempted without a token", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null });

    await useReviewStore.getState().triggerReview({
      taskId: "task-1",
      prUrl: "https://example.com/pr/1",
      trigger: "manual",
    });

    expect(mockCreateApiClient).not.toHaveBeenCalled();
  });
});
