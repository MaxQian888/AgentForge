jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      sectionTitle: "Reviews",
      triggerReview: "Trigger Review",
      backToList: "Back to list",
      loading: "Loading reviews...",
      transitionInvalidTitle: "Invalid transition",
      transitionInvalidApprove:
        "Only reviews awaiting human decision can be approved.",
      transitionInvalidReject:
        "Only reviews awaiting human decision can be rejected.",
      transitionInvalidBlock:
        "Only reviews awaiting human decision can be blocked.",
      transitionInvalidRequestChanges:
        "Only reviews awaiting human decision can request changes.",
    };
    return map[key] ?? key;
  },
}));

jest.mock("./review-list", () => ({
  ReviewList: ({
    reviews,
    onSelect,
    onApprove,
    onToggleSelect,
    selectedIds,
  }: {
    reviews: Array<{ id: string }>;
    onSelect: (review: { id: string }) => void;
    onApprove?: (id: string, comment?: string) => void | Promise<void>;
    onToggleSelect?: (id: string) => void;
    selectedIds?: Set<string>;
  }) => (
    <div>
      <div data-testid="review-list-count">{reviews.length}</div>
      <div data-testid="review-list-selected">
        {selectedIds ? Array.from(selectedIds).join(",") : ""}
      </div>
      {reviews[0] ? (
        <button type="button" onClick={() => onSelect(reviews[0])}>
          Open review
        </button>
      ) : null}
      {onToggleSelect && reviews[0] ? (
        <button
          type="button"
          onClick={() => onToggleSelect(reviews[0].id)}
        >
          Toggle select
        </button>
      ) : null}
      {onApprove && reviews[0] ? (
        <button type="button" onClick={() => onApprove(reviews[0].id)}>
          Approve first
        </button>
      ) : null}
    </div>
  ),
}));

jest.mock("./review-bulk-actions", () => ({
  ReviewBulkActions: ({
    selectedCount,
    eligibleCount,
    onBulkApprove,
    onClearSelection,
  }: {
    selectedCount: number;
    eligibleCount: number;
    onBulkApprove: () => void | Promise<void>;
    onClearSelection: () => void;
  }) =>
    selectedCount === 0 ? null : (
      <div data-testid="review-bulk-actions">
        <span data-testid="bulk-count">{selectedCount}</span>
        <span data-testid="bulk-eligible">{eligibleCount}</span>
        <button type="button" onClick={() => void onBulkApprove()}>
          Bulk approve
        </button>
        <button type="button" onClick={onClearSelection}>
          Clear selection
        </button>
      </div>
    ),
}));

jest.mock("./review-detail-panel", () => ({
  ReviewDetailPanel: ({
    review,
  }: {
    review: { id: string };
  }) => <div data-testid="review-detail">{review.id}</div>,
}));

jest.mock("./review-trigger-form", () => ({
  ReviewTriggerForm: ({
    open,
    onSubmit,
  }: {
    open: boolean;
    onSubmit: (prUrl: string) => void;
  }) =>
    open ? (
      <div data-testid="review-trigger-form">
        <button type="button" onClick={() => onSubmit("https://github.com/org/repo/pull/9")}>
          Submit trigger
        </button>
      </div>
    ) : null,
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ReviewWorkspace } from "./review-workspace";

type ReviewDTO = import("@/lib/stores/review-store").ReviewDTO;

const reviews: ReviewDTO[] = [
  {
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
  },
];

describe("ReviewWorkspace", () => {
  it("renders loading and error states before the review list", () => {
    const { rerender } = render(
      <ReviewWorkspace reviews={[]} loading error="Unable to load reviews" />,
    );

    expect(screen.getByText("Unable to load reviews")).toBeInTheDocument();
    expect(screen.getByText("Loading reviews...")).toBeInTheDocument();

    rerender(<ReviewWorkspace reviews={[]} loading={false} error={null} />);
    expect(screen.getByTestId("review-list-count")).toHaveTextContent("0");
  });

  it("toggles the trigger form, submits manual review input, and opens detail views", async () => {
    const user = userEvent.setup();
    const onTriggerReview = jest.fn().mockResolvedValue(undefined);

    render(
      <ReviewWorkspace
        reviews={[...reviews]}
        onTriggerReview={onTriggerReview}
        triggerTaskId="task-1"
        triggerProjectId="project-1"
      />,
    );

    expect(screen.getByText("Reviews")).toBeInTheDocument();
    expect(screen.getByTestId("review-list-count")).toHaveTextContent("1");

    await user.click(screen.getByRole("button", { name: "Trigger Review" }));
    expect(screen.getByTestId("review-trigger-form")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Submit trigger" }));
    await waitFor(() => {
      expect(onTriggerReview).toHaveBeenCalledWith({
        taskId: "task-1",
        projectId: "project-1",
        prUrl: "https://github.com/org/repo/pull/9",
        trigger: "manual",
      });
    });
    expect(screen.queryByTestId("review-trigger-form")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open review" }));
    expect(screen.getByTestId("review-detail")).toHaveTextContent("review-1");

    await user.click(screen.getByRole("button", { name: "Back to list" }));
    expect(screen.getByTestId("review-list-count")).toHaveTextContent("1");
  });

  it("surfaces a transition error when approving a review not awaiting human decision", async () => {
    const user = userEvent.setup();
    const onApproveReview = jest.fn().mockResolvedValue(undefined);

    render(
      <ReviewWorkspace
        reviews={[...reviews]}
        onApproveReview={onApproveReview}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Approve first" }));
    expect(onApproveReview).not.toHaveBeenCalled();
    expect(screen.getByTestId("review-transition-error")).toHaveTextContent(
      "Only reviews awaiting human decision can be approved.",
    );
  });

  it("renders the bulk toolbar when selection is enabled and forwards the approve action", async () => {
    const user = userEvent.setup();
    const onApproveReview = jest.fn().mockResolvedValue(undefined);
    const pendingHumanReviews: ReviewDTO[] = [
      { ...reviews[0], id: "review-ph", status: "pending_human" },
    ];

    render(
      <ReviewWorkspace
        reviews={pendingHumanReviews}
        enableBulkActions
        onApproveReview={onApproveReview}
      />,
    );

    // No selection yet - bulk toolbar hidden
    expect(screen.queryByTestId("review-bulk-actions")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Toggle select" }));
    expect(screen.getByTestId("review-bulk-actions")).toBeInTheDocument();
    expect(screen.getByTestId("bulk-count")).toHaveTextContent("1");
    expect(screen.getByTestId("bulk-eligible")).toHaveTextContent("1");

    await user.click(screen.getByRole("button", { name: "Bulk approve" }));
    expect(onApproveReview).toHaveBeenCalledWith("review-ph");
  });
});
