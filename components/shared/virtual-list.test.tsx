import { fireEvent, render, screen } from "@testing-library/react";
import { VirtualList } from "./virtual-list";

describe("VirtualList", () => {
  const items = Array.from({ length: 100 }, (_, index) => `Item ${index}`);

  it("renders only the visible window with overscan", () => {
    render(
      <VirtualList
        data-testid="virtual-list"
        items={items}
        height={100}
        itemHeight={20}
        overscan={1}
        renderItem={(item) => <div>{item}</div>}
      />,
    );

    expect(screen.getByText("Item 0")).toBeInTheDocument();
    expect(screen.getByText("Item 5")).toBeInTheDocument();
    expect(screen.queryByText("Item 8")).not.toBeInTheDocument();
  });

  it("updates the rendered range when the container scrolls", () => {
    render(
      <VirtualList
        data-testid="virtual-list"
        items={items}
        height={100}
        itemHeight={20}
        overscan={1}
        renderItem={(item) => <div>{item}</div>}
      />,
    );

    fireEvent.scroll(screen.getByTestId("virtual-list"), {
      target: { scrollTop: 200 },
    });

    expect(screen.getByText("Item 10")).toBeInTheDocument();
    expect(screen.queryByText("Item 0")).not.toBeInTheDocument();
  });
});
