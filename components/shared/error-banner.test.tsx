import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ErrorBanner } from "./error-banner";

describe("ErrorBanner", () => {
  it("renders the error message and exposes retry when available", async () => {
    const user = userEvent.setup();
    const onRetry = jest.fn();

    render(<ErrorBanner message="Failed to load workspace" onRetry={onRetry} />);

    expect(screen.getByText("Failed to load workspace")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));

    expect(onRetry).toHaveBeenCalled();
  });

  it("omits retry actions when no callback is provided", () => {
    render(<ErrorBanner message="No retry available" />);

    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });
});
