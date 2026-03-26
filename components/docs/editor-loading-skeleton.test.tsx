import { render } from "@testing-library/react";
import { EditorLoadingSkeleton } from "./editor-loading-skeleton";

describe("EditorLoadingSkeleton", () => {
  it("renders the expected loading placeholders", () => {
    const { container } = render(<EditorLoadingSkeleton />);

    expect(container.firstChild).toHaveClass("min-h-[360px]");
    expect(container.querySelectorAll(".animate-pulse")).toHaveLength(6);
  });
});
