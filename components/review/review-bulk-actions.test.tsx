jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const messages: Record<string, string> = {
      bulkSelected: "{count} selected",
      bulkApprove: "Approve all",
      bulkReject: "Reject all",
      bulkBlock: "Block all",
      bulkClear: "Clear selection",
      bulkNoEligible: "No eligible reviews.",
      bulkConfirmApproveTitle: "Approve {count} reviews?",
      bulkConfirmRejectTitle: "Reject {count} reviews?",
      bulkConfirmBlockTitle: "Block {count} reviews?",
      bulkConfirmApproveDescription: "Approve eligible reviews only.",
      bulkConfirmRejectDescription: "Reject eligible reviews only.",
      bulkConfirmBlockDescription: "Block eligible reviews only.",
      bulkPartialSkipped:
        "{skipped} of {count} selected reviews were skipped.",
      bulkReasonPlaceholder: "Reason for all selected reviews...",
      blockCommentLabel: "Blocking reason (required)",
      rejectCommentLabel: "Reject reason (required)",
      rejectReasonRequired: "A reject reason is required.",
      blockReasonRequired: "A blocking reason is required.",
      confirmApprove: "Confirm Approve",
      confirmReject: "Confirm Reject",
      confirmBlock: "Confirm Block",
    };
    const template = messages[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (_, token) =>
      String(values?.[token] ?? ""),
    );
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ReviewBulkActions } from "./review-bulk-actions";

describe("ReviewBulkActions", () => {
  it("renders nothing when no reviews are selected", () => {
    const { container } = render(
      <ReviewBulkActions
        selectedCount={0}
        eligibleCount={0}
        onBulkApprove={jest.fn()}
        onBulkReject={jest.fn()}
        onBulkBlock={jest.fn()}
        onClearSelection={jest.fn()}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("disables actions when there are no eligible reviews", () => {
    render(
      <ReviewBulkActions
        selectedCount={3}
        eligibleCount={0}
        onBulkApprove={jest.fn()}
        onBulkReject={jest.fn()}
        onBulkBlock={jest.fn()}
        onClearSelection={jest.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Approve all" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Reject all" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Block all" })).toBeDisabled();
    expect(screen.getByText("No eligible reviews.")).toBeInTheDocument();
  });

  it("confirms bulk approve and calls the provided handler", async () => {
    const user = userEvent.setup();
    const onBulkApprove = jest.fn().mockResolvedValue(undefined);

    render(
      <ReviewBulkActions
        selectedCount={2}
        eligibleCount={2}
        onBulkApprove={onBulkApprove}
        onBulkReject={jest.fn()}
        onBulkBlock={jest.fn()}
        onClearSelection={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Approve all" }));
    expect(
      await screen.findByText("Approve 2 reviews?"),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Confirm Approve" }));
    expect(onBulkApprove).toHaveBeenCalledTimes(1);
  });

  it("requires a reason before running bulk reject", async () => {
    const user = userEvent.setup();
    const onBulkReject = jest.fn().mockResolvedValue(undefined);

    render(
      <ReviewBulkActions
        selectedCount={3}
        eligibleCount={3}
        onBulkApprove={jest.fn()}
        onBulkReject={onBulkReject}
        onBulkBlock={jest.fn()}
        onClearSelection={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Reject all" }));
    await user.click(screen.getByRole("button", { name: "Confirm Reject" }));
    expect(onBulkReject).not.toHaveBeenCalled();
    expect(screen.getByTestId("review-bulk-reason-error")).toHaveTextContent(
      "A reject reason is required.",
    );

    await user.type(
      screen.getByPlaceholderText("Reason for all selected reviews..."),
      "Security issues",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Reject" }));
    expect(onBulkReject).toHaveBeenCalledWith("Security issues");
  });

  it("surfaces skipped counts when some reviews are not eligible", async () => {
    const user = userEvent.setup();

    render(
      <ReviewBulkActions
        selectedCount={5}
        eligibleCount={2}
        onBulkApprove={jest.fn()}
        onBulkReject={jest.fn()}
        onBulkBlock={jest.fn()}
        onClearSelection={jest.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Approve all" }));
    expect(
      screen.getByText("3 of 5 selected reviews were skipped."),
    ).toBeInTheDocument();
  });
});
