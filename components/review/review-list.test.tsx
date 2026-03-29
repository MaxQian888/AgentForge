jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const messages: Record<string, string> = {
      noReviewsYet: "No reviews yet.",
      layerReview: "Layer {layer} Review",
      noSummary: "No summary available.",
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
      statusCompleted: "Completed",
      statusPending: "Pending",
      statusInProgress: "In Progress",
      statusFailed: "Failed",
      riskHigh: "High",
      riskMedium: "Medium",
      riskLow: "Low",
      riskCritical: "Critical",
    };
    const template = messages[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (_, token) => String(values?.[token] ?? ""));
  },
}));

import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { ReviewList } from "./review-list";
import type { ReviewDTO } from "@/lib/stores/review-store";

function makeReview(overrides: Partial<ReviewDTO> = {}): ReviewDTO {
  return {
    id: "review-1",
    taskId: "task-1",
    prUrl: "https://github.com/org/repo/pull/1",
    prNumber: 1,
    layer: 2,
    status: "pending_human",
    riskLevel: "high",
    findings: [],
    summary: "Critical checks completed.",
    recommendation: "request_changes",
    costUsd: 1.25,
    createdAt: "2026-03-25T08:00:00.000Z",
    updatedAt: "2026-03-25T08:30:00.000Z",
    ...overrides,
  };
}

describe("ReviewList", () => {
  it("shows an empty-state message when no reviews exist", () => {
    render(<ReviewList reviews={[]} onSelect={jest.fn()} />);

    expect(screen.getByText("No reviews yet.")).toBeInTheDocument();
  });

  it("lets users select, approve, and request changes on a pending_human review without using prompt dialogs", async () => {
    const user = userEvent.setup();
    const onSelect = jest.fn();
    const onApprove = jest.fn();
    const onRequestChanges = jest.fn();
    const promptSpy = jest.spyOn(window, "prompt");

    render(
      <ReviewList
        reviews={[makeReview()]}
        onSelect={onSelect}
        onApprove={onApprove}
        onRequestChanges={onRequestChanges}
      />,
    );

    await user.click(screen.getByText("Layer 2 Review"));
    expect(onSelect).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Approve" }));
    await user.type(
      screen.getByPlaceholderText("Optional approval comment..."),
      "Ship after quick pass",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Approve" }));
    expect(onApprove).toHaveBeenCalledWith("review-1", "Ship after quick pass");

    await user.click(screen.getByRole("button", { name: "Request Changes" }));
    expect(promptSpy).not.toHaveBeenCalled();
    await user.type(
      screen.getByPlaceholderText("Describe what needs to change..."),
      "Needs fixes",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Request Changes" }));
    expect(onRequestChanges).toHaveBeenCalledWith("review-1", "Needs fixes");

    expect(onSelect).toHaveBeenCalledTimes(1);
    promptSpy.mockRestore();
  });
});
