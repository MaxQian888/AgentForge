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
});
