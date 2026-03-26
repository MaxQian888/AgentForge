const fetchReviewsByTask = jest.fn().mockResolvedValue(undefined);
const triggerReview = jest.fn().mockResolvedValue(undefined);
const approveReview = jest.fn().mockResolvedValue(undefined);
const requestChanges = jest.fn().mockResolvedValue(undefined);

const reviewFixture = {
  id: "review-1",
  taskId: "task-1",
  prUrl: "https://github.com/org/repo/pull/1",
  prNumber: 1,
  layer: 2,
  status: "completed",
  riskLevel: "high",
  findings: [],
  summary: "Review summary",
  recommendation: "approve",
  costUsd: 1.5,
  createdAt: "2026-03-25T08:00:00.000Z",
  updatedAt: "2026-03-25T08:10:00.000Z",
};

const storeState = {
  reviewsByTask: { "task-1": [reviewFixture] },
  loading: false,
  error: null,
  fetchReviewsByTask,
  triggerReview,
  approveReview,
  requestChanges,
};

jest.mock("@/lib/stores/review-store", () => ({
  useReviewStore: (selector?: (state: typeof storeState) => unknown) =>
    typeof selector === "function" ? selector(storeState) : storeState,
}));

jest.mock("./review-list", () => ({
  ReviewList: ({
    reviews,
    onSelect,
  }: {
    reviews: typeof storeState.reviewsByTask["task-1"];
    onSelect: (review: typeof reviewFixture) => void;
  }) => (
    <div>
      <div data-testid="review-count">{reviews.length}</div>
      <button onClick={() => onSelect(reviews[0])}>Open review</button>
    </div>
  ),
}));

jest.mock("./review-detail-panel", () => ({
  ReviewDetailPanel: ({ review }: { review: { id: string } }) => (
    <div data-testid="review-detail">{review.id}</div>
  ),
}));

import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import { TaskReviewSection } from "./task-review-section";

describe("TaskReviewSection", () => {
  beforeEach(() => {
    fetchReviewsByTask.mockClear();
    triggerReview.mockClear();
    approveReview.mockClear();
    requestChanges.mockClear();
  });

  it("loads reviews, triggers new reviews, and switches into the detail view", async () => {
    const user = userEvent.setup();
    render(<TaskReviewSection taskId="task-1" />);

    await waitFor(() => expect(fetchReviewsByTask).toHaveBeenCalledWith("task-1"));
    expect(screen.getByTestId("review-count")).toHaveTextContent("1");

    await user.click(screen.getByRole("button", { name: "Trigger Review" }));
    await user.type(
      screen.getByPlaceholderText("https://github.com/org/repo/pull/123"),
      "https://github.com/org/repo/pull/22",
    );
    await user.click(screen.getByRole("button", { name: "Submit" }));

    await waitFor(() =>
      expect(triggerReview).toHaveBeenCalledWith({
        taskId: "task-1",
        prUrl: "https://github.com/org/repo/pull/22",
        trigger: "manual",
      }),
    );
    expect(fetchReviewsByTask).toHaveBeenCalledTimes(2);

    await user.click(screen.getByRole("button", { name: "Open review" }));
    expect(screen.getByTestId("review-detail")).toHaveTextContent("review-1");

    await user.click(screen.getByRole("button", { name: "Back to list" }));
    expect(screen.getByTestId("review-count")).toBeInTheDocument();
  });
});
