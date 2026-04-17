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
const rejectReview = jest.fn().mockResolvedValue(undefined);

const taskStoreState = {
  tasks: [] as Array<{
    id: string;
    assigneeId: string | null;
    assigneeName: string | null;
    agentBranch: string;
  }>,
};

jest.mock("@/lib/stores/task-store", () => ({
  useTaskStore: (selector?: (state: typeof taskStoreState) => unknown) =>
    typeof selector === "function" ? selector(taskStoreState) : taskStoreState,
}));

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
  rejectReview,
};

jest.mock("@/lib/stores/review-store", () => ({
  useReviewStore: (selector?: (state: typeof storeState) => unknown) =>
    typeof selector === "function" ? selector(storeState) : storeState,
}));

jest.mock("@/components/shared/filter-bar", () => ({
  FilterBar: ({
    onReset,
    onSearch,
    filters,
  }: {
    onReset: () => void;
    onSearch?: (value: string) => void;
    filters: Array<{ onChange: (value: string) => void }>;
  }) => (
    <div>
      <button type="button" onClick={() => filters[0]?.onChange("completed")}>
        filter-status
      </button>
      <button type="button" onClick={() => filters[1]?.onChange("high")}>
        filter-risk
      </button>
      {onSearch ? (
        <input
          aria-label="search-input"
          onChange={(event) => onSearch(event.target.value)}
        />
      ) : null}
      <button type="button" onClick={onReset}>
        reset-filters
      </button>
    </div>
  ),
}));

jest.mock("@/components/review/review-workspace", () => ({
  ReviewWorkspace: ({
    selectedReviewId,
    reviews,
  }: {
    selectedReviewId: string | null;
    reviews: Array<{ id: string }>;
  }) => (
    <div data-testid="review-workspace">
      <span data-testid="review-workspace-selected">
        {selectedReviewId ?? "none"}
      </span>
      <span data-testid="review-workspace-count">{reviews.length}</span>
      <ul>
        {reviews.map((r) => (
          <li key={r.id} data-testid={`workspace-review-${r.id}`}>
            {r.id}
          </li>
        ))}
      </ul>
    </div>
  ),
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
    rejectReview.mockClear();
    taskStoreState.tasks = [];
    storeState.allReviews = [
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
    ];
    storeState.allReviewsLoading = false;
    storeState.error = null;
  });

  it("loads the backlog and forwards the selected review id into the workspace", async () => {
    await act(async () => {
      render(<ReviewsPage searchParams={Promise.resolve({ id: "review-standalone" })} />);
    });

    await waitFor(() =>
      expect(fetchAllReviews).toHaveBeenCalledWith({
        status: undefined,
        riskLevel: undefined,
      }),
    );

    expect(screen.getByTestId("review-workspace")).toHaveTextContent("review-standalone");
  });

  it("refetches reviews when filters change and resets them back to all", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<ReviewsPage searchParams={Promise.resolve({ id: "review-task" })} />);
    });

    fetchAllReviews.mockClear();
    await user.click(screen.getByRole("button", { name: "filter-status" }));
    await waitFor(() =>
      expect(fetchAllReviews).toHaveBeenLastCalledWith({
        status: "completed",
        riskLevel: undefined,
      }),
    );

    await user.click(screen.getByRole("button", { name: "filter-risk" }));
    await waitFor(() =>
      expect(fetchAllReviews).toHaveBeenLastCalledWith({
        status: "completed",
        riskLevel: "high",
      }),
    );

    await user.click(screen.getByRole("button", { name: "reset-filters" }));
    await waitFor(() =>
      expect(fetchAllReviews).toHaveBeenLastCalledWith({
        status: undefined,
        riskLevel: undefined,
      }),
    );
  });

  it("renders the loading and empty states when no reviews are available", async () => {
    storeState.allReviews = [];
    storeState.allReviewsLoading = true;

    let rerender: ReturnType<typeof render>["rerender"];
    await act(async () => {
      ({ rerender } = render(<ReviewsPage searchParams={Promise.resolve({})} />));
    });

    expect(screen.getByText("Loading reviews...")).toBeInTheDocument();

    storeState.allReviewsLoading = false;
    await act(async () => {
      rerender(<ReviewsPage searchParams={Promise.resolve({})} />);
    });

    expect(screen.getByText("No reviews found.")).toBeInTheDocument();
  });

  it("filters reviews by search query against the summary", async () => {
    const user = userEvent.setup();

    await act(async () => {
      render(<ReviewsPage searchParams={Promise.resolve({})} />);
    });

    // Baseline: both reviews pass to workspace
    expect(screen.getByTestId("review-workspace-count")).toHaveTextContent("2");

    await user.type(screen.getByLabelText("search-input"), "Standalone");

    await waitFor(() => {
      expect(screen.getByTestId("review-workspace-count")).toHaveTextContent(
        "1",
      );
    });
    expect(
      screen.getByTestId("workspace-review-review-standalone"),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("workspace-review-review-task"),
    ).not.toBeInTheDocument();
  });
});
