jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const messages: Record<string, string> = {
      layerReview: "Layer {layer} Review",
      summary: "Summary",
      noSummary: "No summary available.",
      detailPrLabel: "PR: {value}",
      detailCostLabel: "Cost: ${value}",
      detailStatusLabel: "Status: {value}",
      detailUpdatedLabel: "Updated: {value}",
      executionDetails: "Execution Details",
      executionTrigger: "Trigger",
      executionChangedFiles: "Changed Files",
      executionResults: "Plugin / Dimension Results",
      findingsCount: "Findings ({count})",
      decisions: "Decisions",
      recommendationApprove: "Approve",
      recommendationRequestChanges: "Request Changes",
      recommendationReject: "Reject",
      approveCommentLabel: "Comment (optional)",
      requestChangesCommentLabel: "Comment (optional)",
      approveCommentPlaceholder: "Optional approval comment...",
      requestChangesCommentPlaceholder: "Describe what needs to change...",
      confirmApprove: "Confirm Approve",
      confirmRequestChanges: "Confirm Request Changes",
      cancelTrigger: "Cancel",
      statusPendingHuman: "Pending Human",
      riskHigh: "High",
      detailsOverviewTab: "Overview",
      detailsHistoryTab: "History",
      detailsCommentsTab: "Comments",
      historyEmpty: "No history recorded for this review.",
      commentsEmpty: "No comments yet.",
    };
    const template = messages[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (_, token) => String(values?.[token] ?? ""));
  },
}));

jest.mock("./review-findings-table", () => ({
  ReviewFindingsTable: ({
    findings,
  }: {
    findings: Array<{ message: string }>;
  }) => <div data-testid="review-findings-table">{findings.length} findings</div>,
}));

import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { ReviewDetailPanel } from "./review-detail-panel";
import type { ReviewDTO } from "@/lib/stores/review-store";

const review: ReviewDTO = {
  id: "review-1",
  taskId: "task-1",
  prUrl: "https://github.com/org/repo/pull/1",
  prNumber: 1,
  layer: 2,
  status: "pending_human",
  riskLevel: "high",
  findings: [{ category: "security", severity: "high", message: "Fix auth guard." }],
  summary: "Guard checks are incomplete.",
  recommendation: "request_changes",
  costUsd: 2.5,
  createdAt: "2026-03-25T08:00:00.000Z",
  updatedAt: "2026-03-25T08:30:00.000Z",
};

describe("ReviewDetailPanel", () => {
  it("shows review metadata and supports pending_human actions", async () => {
    const user = userEvent.setup();
    const onApprove = jest.fn();
    const onRequestChanges = jest.fn();

    render(
      <ReviewDetailPanel
        review={review}
        onApprove={onApprove}
        onRequestChanges={onRequestChanges}
      />,
    );

    expect(screen.getByText("Layer 2 Review")).toBeInTheDocument();
    expect(screen.getByText("Guard checks are incomplete.")).toBeInTheDocument();
    expect(screen.getByText("PR: https://github.com/org/repo/pull/1")).toBeInTheDocument();
    expect(screen.getByTestId("review-findings-table")).toHaveTextContent("1 findings");

    await user.click(screen.getByRole("button", { name: "Approve" }));
    await user.type(
      screen.getByPlaceholderText("Optional approval comment..."),
      "Looks good after fixes",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Approve" }));
    expect(onApprove).toHaveBeenCalledWith("review-1", "Looks good after fixes");

    await user.click(screen.getByRole("button", { name: "Request Changes" }));
    await user.type(
      screen.getByPlaceholderText("Describe what needs to change..."),
      "Still missing tests",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Request Changes" }));
    expect(onRequestChanges).toHaveBeenCalledWith("review-1", "Still missing tests");
  });

  it("renders history and comments tabs from execution metadata decisions", async () => {
    const user = userEvent.setup();
    const reviewWithHistory: ReviewDTO = {
      ...review,
      executionMetadata: {
        decisions: [
          {
            actor: "alice",
            action: "approve",
            comment: "Looks ready to ship.",
            timestamp: "2026-03-25T09:00:00.000Z",
          },
          {
            actor: "bob",
            action: "reject",
            comment: "",
            timestamp: "2026-03-25T09:05:00.000Z",
          },
        ],
      },
    };

    render(<ReviewDetailPanel review={reviewWithHistory} />);

    await user.click(screen.getByRole("tab", { name: "History" }));
    expect(screen.getByTestId("review-history-tab")).toHaveTextContent("alice");
    expect(screen.getByTestId("review-history-tab")).toHaveTextContent("bob");

    await user.click(screen.getByRole("tab", { name: "Comments" }));
    expect(screen.getByTestId("review-comments-tab")).toHaveTextContent(
      "Looks ready to ship.",
    );
    // bob's decision has empty comment so it should not appear
    expect(screen.getByTestId("review-comments-tab")).not.toHaveTextContent(
      "bob",
    );
  });
});
