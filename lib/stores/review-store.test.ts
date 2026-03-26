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
      allReviews: [],
      allReviewsLoading: false,
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

  it("submits request-changes transitions through the canonical endpoint", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({
      data: {
        id: "review-2",
        taskId: "task-1",
        prUrl: "https://example.com/pr/1",
        prNumber: 1,
        layer: 2,
        status: "completed",
        riskLevel: "medium",
        findings: [],
        summary: "needs changes",
        recommendation: "request_changes",
        costUsd: 0.4,
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:15:00.000Z",
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    await useReviewStore.getState().requestChanges("review-2", "please update tests");

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/reviews/review-2/request-changes",
      { comment: "please update tests" },
      { token: "test-token" },
    );
    expect(useReviewStore.getState().allReviews[0]).toEqual(
      expect.objectContaining({
        id: "review-2",
        recommendation: "request_changes",
      }),
    );
  });

  it("posts false-positive findings using findingIds payload", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({
      data: {
        id: "review-3",
        taskId: "task-9",
        prUrl: "https://example.com/pr/9",
        prNumber: 9,
        layer: 2,
        status: "completed",
        riskLevel: "low",
        findings: [{ id: "f1", category: "security", severity: "low", message: "ok", dismissed: true }],
        summary: "false positive recorded",
        recommendation: "approve",
        costUsd: 0.1,
        createdAt: "2026-03-25T10:00:00.000Z",
        updatedAt: "2026-03-25T10:20:00.000Z",
      },
    });
    mockCreateApiClient.mockReturnValue(api);

    await useReviewStore
      .getState()
      .markFalsePositive("review-3", ["f1"], "known acceptable behavior");

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/reviews/review-3/false-positive",
      { findingIds: ["f1"], reason: "known acceptable behavior" },
      { token: "test-token" },
    );
    expect(useReviewStore.getState().allReviews[0]?.id).toBe("review-3");
  });

  it("upserts websocket-driven review updates across all and task slices", () => {
    useReviewStore.setState({
      allReviews: [],
      reviewsByTask: { "task-1": [] },
    });

    useReviewStore.getState().updateReview({
      id: "review-live",
      taskId: "task-1",
      prUrl: "https://example.com/pr/live",
      prNumber: 77,
      layer: 2,
      status: "pending_human",
      riskLevel: "high",
      findings: [],
      summary: "waiting on human approval",
      recommendation: "approve",
      costUsd: 0.7,
      createdAt: "2026-03-26T08:00:00.000Z",
      updatedAt: "2026-03-26T08:10:00.000Z",
      executionMetadata: {
        decisions: [{ actor: "reviewer", action: "approve", comment: "ok", timestamp: "2026-03-26T08:10:00.000Z" }],
      },
    });

    expect(useReviewStore.getState().allReviews[0]).toEqual(
      expect.objectContaining({
        id: "review-live",
        status: "pending_human",
      }),
    );
    expect(useReviewStore.getState().reviewsByTask["task-1"][0]).toEqual(
      expect.objectContaining({
        id: "review-live",
      }),
    );
  });
});
