jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      prUrlLabel: "PR URL",
      invalidPrUrl: "Enter a valid PR URL.",
      submitTrigger: "Submit",
      cancelTrigger: "Cancel",
    };
    return map[key] ?? key;
  },
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ReviewTriggerForm } from "./review-trigger-form";

describe("ReviewTriggerForm", () => {
  it("renders nothing when the form is closed", () => {
    const { container } = render(
      <ReviewTriggerForm
        open={false}
        onOpenChange={jest.fn()}
        onSubmit={jest.fn()}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("validates pull request URLs and submits normalized values", async () => {
    const user = userEvent.setup();
    const onSubmit = jest.fn().mockResolvedValue(undefined);
    const onOpenChange = jest.fn();

    render(
      <ReviewTriggerForm
        open
        onOpenChange={onOpenChange}
        onSubmit={onSubmit}
      />,
    );

    const input = screen.getByPlaceholderText(
      "https://github.com/org/repo/pull/123",
    );

    await user.type(input, "not-a-url");
    await user.click(screen.getByRole("button", { name: "Submit" }));
    expect(screen.getByText("Enter a valid PR URL.")).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, "  https://github.com/org/repo/pull/42  ");
    await user.click(screen.getByRole("button", { name: "Submit" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        "https://github.com/org/repo/pull/42",
      );
    });

    await user.type(input, "https://github.com/org/repo/pull/99");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
