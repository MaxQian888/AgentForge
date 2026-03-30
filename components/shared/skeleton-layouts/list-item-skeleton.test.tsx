import { render } from "@testing-library/react";
import { ListItemSkeleton } from "./list-item-skeleton";

describe("ListItemSkeleton", () => {
  it("renders the requested number of list placeholders", () => {
    const { container } = render(
      <ListItemSkeleton count={3} className="list-skeleton" />,
    );

    expect(container.firstChild).toHaveClass("list-skeleton");
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(12);
  });
});
