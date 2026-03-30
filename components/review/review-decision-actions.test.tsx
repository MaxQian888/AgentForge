jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      recommendationApprove: "Approve",
      recommendationRequestChanges: "Request Changes",
      approveCommentLabel: "Comment (optional)",
      requestChangesCommentLabel: "Comment (optional)",
      approveCommentPlaceholder: "Optional approval comment...",
      requestChangesCommentPlaceholder: "Describe what needs to change...",
      confirmApprove: "Confirm Approve",
      confirmRequestChanges: "Confirm Request Changes",
      cancelTrigger: "Cancel",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ReviewDecisionActions } from "./review-decision-actions";

describe("ReviewDecisionActions", () => {
  it("renders nothing when no decision callbacks are provided", () => {
    const { container } = render(<ReviewDecisionActions reviewId="review-1" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("supports approval and trims the submitted comment", async () => {
    const user = userEvent.setup();
    const onApprove = jest.fn().mockResolvedValue(undefined);

    render(
      <ReviewDecisionActions reviewId="review-1" onApprove={onApprove} compact />,
    );

    await user.click(screen.getByRole("button", { name: "Approve" }));
    await user.type(
      screen.getByPlaceholderText("Optional approval comment..."),
      "  Ship it  ",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Approve" }));

    expect(onApprove).toHaveBeenCalledWith("review-1", "Ship it");
    expect(screen.queryByRole("button", { name: "Confirm Approve" })).not.toBeInTheDocument();
  });

  it("supports request-changes flows and can cancel back to the chooser", async () => {
    const user = userEvent.setup();
    const onRequestChanges = jest.fn().mockResolvedValue(undefined);

    render(
      <ReviewDecisionActions
        reviewId="review-2"
        onRequestChanges={onRequestChanges}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Request Changes" }));
    await user.type(
      screen.getByPlaceholderText("Describe what needs to change..."),
      "Needs more tests",
    );
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getByRole("button", { name: "Request Changes" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Request Changes" }));
    await user.type(
      screen.getByPlaceholderText("Describe what needs to change..."),
      "Needs more tests",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Request Changes" }));

    expect(onRequestChanges).toHaveBeenCalledWith("review-2", "Needs more tests");
  });
});
