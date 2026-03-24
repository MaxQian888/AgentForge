import { render } from "@testing-library/react";
import { Separator } from "./separator";

describe("Separator", () => {
  it("renders horizontal and vertical separators with the expected slot metadata", () => {
    const { container } = render(
      <>
        <Separator className="my-4" />
        <Separator orientation="vertical" decorative={false} className="mx-2" />
      </>
    );

    const separators = container.querySelectorAll('[data-slot="separator"]');
    expect(separators).toHaveLength(2);
    expect(separators[0]).toHaveAttribute("data-orientation", "horizontal");
    expect(separators[0]).toHaveClass("my-4");
    expect(separators[1]).toHaveAttribute("data-orientation", "vertical");
    expect(separators[1]).toHaveAttribute("aria-orientation", "vertical");
    expect(separators[1]).toHaveClass("mx-2");
  });
});
