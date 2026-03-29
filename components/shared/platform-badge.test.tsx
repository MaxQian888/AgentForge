import { render, screen } from "@testing-library/react";
import { PlatformBadge } from "./platform-badge";

describe("PlatformBadge", () => {
  it("renders the platform label from shared definitions", () => {
    render(<PlatformBadge platform="qqbot" />);

    expect(screen.getByText("QQ Bot")).toBeInTheDocument();
    expect(screen.getByTestId("platform-badge-qqbot")).toHaveAttribute(
      "data-platform",
      "qqbot",
    );
  });
});
