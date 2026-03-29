jest.mock("next-intl", () => ({
  useTranslations: () => {
    const messages: Record<string, string> = {
      title: "Reviews",
      filterStatus: "Status",
      filterRiskLevel: "Risk Level",
      all: "All",
      statusPending: "Pending",
      statusInProgress: "In Progress",
      statusCompleted: "Completed",
      statusFailed: "Failed",
      statusPendingHuman: "Pending Human",
      riskCritical: "Critical",
      riskHigh: "High",
      riskMedium: "Medium",
      riskLow: "Low",
      loading: "Loading reviews...",
      emptyState: "No reviews found.",
      triggerReview: "Trigger Review",
      submitTrigger: "Submit",
      cancelTrigger: "Cancel",
      prUrlLabel: "PR URL",
      invalidPrUrl: "Enter a valid pull request URL.",
      backToList: "Back to list",
      layerReview: "Layer {layer} Review",
      noSummary: "No summary available.",
      recommendationApprove: "Approve",
      recommendationRequestChanges: "Request Changes",
      recommendationReject: "Reject",
      recommendationUnknown: "Unknown Recommendation",
      statusUnknown: "Unknown Status",
      riskUnknown: "Unknown Risk",
      riskInfo: "Info",
      approveCommentLabel: "Comment (optional)",
      requestChangesCommentLabel: "Comment (optional)",
      approveCommentPlaceholder: "Optional approval comment...",
      requestChangesCommentPlaceholder: "Describe what needs to change...",
      confirmApprove: "Confirm Approve",
      confirmRequestChanges: "Confirm Request Changes",
      detailPrLabel: "PR: {value}",
      detailCostLabel: "Cost: ${value}",
      detailStatusLabel: "Status: {value}",
      detailUpdatedLabel: "Updated: {value}",
      executionDetails: "Execution Details",
      executionTrigger: "Trigger",
      executionChangedFiles: "Changed Files",
      executionResults: "Plugin / Dimension Results",
      decisions: "Decisions",
      noReviewsYet: "No reviews yet.",
      noFindingsReported: "No findings reported.",
      findingSeverity: "Severity",
      findingCategory: "Category",
      findingSource: "Source",
      findingFileLine: "File:Line",
      findingMessage: "Message",
      findingSuggestion: "Suggestion",
    };

    return (key: string, values?: Record<string, string | number>) =>
      (messages[key] ?? key).replace(/\{(\w+)\}/g, (_, token) => String(values?.[token] ?? ""));
  },
}));

const fetchAllReviews = jest.fn().mockResolvedValue(undefined);
const triggerReview = jest.fn().mockResolvedValue(undefined);
const approveReview = jest.fn().mockResolvedValue(undefined);
const requestChanges = jest.fn().mockResolvedValue(undefined);

const storeState = {
  allReviews: [
    {
      id: "review-task",
      taskId: "task-1",
      prUrl: "https://github.com/org/repo/pull/1",
      prNumber: 1,
      layer: 2,
      status: "pending_human",
      riskLevel: "high",
      findings: [],
      summary: "Task-bound review summary",
      recommendation: "request_changes",
      costUsd: 1.25,
      createdAt: "2026-03-25T08:00:00.000Z",
      updatedAt: "2026-03-25T08:30:00.000Z",
    },
    {
      id: "review-standalone",
      taskId: "",
      prUrl: "https://github.com/org/repo/pull/99",
      prNumber: 99,
      layer: 2,
      status: "completed",
      riskLevel: "medium",
      findings: [],
      summary: "Standalone review summary",
      recommendation: "approve",
      costUsd: 0.75,
      createdAt: "2026-03-26T08:00:00.000Z",
      updatedAt: "2026-03-26T08:20:00.000Z",
    },
  ],
  allReviewsLoading: false,
  error: null,
  fetchAllReviews,
  triggerReview,
  approveReview,
  requestChanges,
};

jest.mock("@/lib/stores/review-store", () => ({
  useReviewStore: (selector?: (state: typeof storeState) => unknown) =>
    typeof selector === "function" ? selector(storeState) : storeState,
}));

import userEvent from "@testing-library/user-event";
import { act, render, screen, waitFor } from "@testing-library/react";
import ReviewsPage from "./page";

describe("ReviewsPage", () => {
  beforeEach(() => {
    fetchAllReviews.mockClear();
    triggerReview.mockClear();
    approveReview.mockClear();
    requestChanges.mockClear();
  });

  it("loads the backlog, opens the detail for the review id in search params, and supports standalone triggers", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<ReviewsPage searchParams={Promise.resolve({ id: "review-standalone" })} />);
    });

    await waitFor(() =>
      expect(fetchAllReviews).toHaveBeenCalledWith({
        status: undefined,
        riskLevel: undefined,
      }),
    );

    expect(
      screen.getByText("PR: https://github.com/org/repo/pull/99"),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Trigger Review" }));
    await user.type(
      screen.getByPlaceholderText("https://github.com/org/repo/pull/123"),
      "https://git.example.com/org/repo/pull/77",
    );
    await user.click(screen.getByRole("button", { name: "Submit" }));

    await waitFor(() =>
      expect(triggerReview).toHaveBeenCalledWith({
        prUrl: "https://git.example.com/org/repo/pull/77",
        trigger: "manual",
      }),
    );
  });

  it("resyncs the selected review when the search param id changes", async () => {
    let rerender: ReturnType<typeof render>["rerender"];
    await act(async () => {
      ({ rerender } = render(
        <ReviewsPage searchParams={Promise.resolve({ id: "review-task" })} />,
      ));
    });

    expect(
      await screen.findByText("PR: https://github.com/org/repo/pull/1"),
    ).toBeInTheDocument();

    await act(async () => {
      rerender(<ReviewsPage searchParams={Promise.resolve({ id: "review-standalone" })} />);
    });

    expect(
      await screen.findByText("PR: https://github.com/org/repo/pull/99"),
    ).toBeInTheDocument();
  });
});
