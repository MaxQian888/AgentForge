jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      sectionTitle: "Reviews",
      triggerReview: "Trigger Review",
      backToList: "Back to list",
      loading: "Loading reviews...",
    };
    return map[key] ?? key;
  },
}));

jest.mock("./review-list", () => ({
  ReviewList: ({
    reviews,
    onSelect,
  }: {
    reviews: Array<{ id: string }>;
    onSelect: (review: { id: string }) => void;
  }) => (
    <div>
      <div data-testid="review-list-count">{reviews.length}</div>
      {reviews[0] ? (
        <button type="button" onClick={() => onSelect(reviews[0])}>
          Open review
        </button>
      ) : null}
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
});
