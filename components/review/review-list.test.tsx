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
    status: "completed",
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

  it("lets users select, approve, and reject a completed review", async () => {
    const user = userEvent.setup();
    const onSelect = jest.fn();
    const onApprove = jest.fn();
    const onReject = jest.fn();
    const promptSpy = jest.spyOn(window, "prompt").mockReturnValue("Needs fixes");

    render(
      <ReviewList
        reviews={[makeReview()]}
        onSelect={onSelect}
        onApprove={onApprove}
        onReject={onReject}
      />,
    );

    await user.click(screen.getByText("Layer 2 Review"));
    expect(onSelect).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Approve" }));
    expect(onApprove).toHaveBeenCalledWith("review-1");
    expect(onSelect).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Reject" }));
    expect(onReject).toHaveBeenCalledWith("review-1", "Needs fixes");

    promptSpy.mockRestore();
  });
});
