import { render, screen } from "@testing-library/react";
import { OutputStream } from "./output-stream";

describe("OutputStream", () => {
  const originalScrollIntoView = Element.prototype.scrollIntoView;

  afterEach(() => {
    Element.prototype.scrollIntoView = originalScrollIntoView;
  });

  it("shows an empty-state message and scrolls to the bottom on render", () => {
    const scrollIntoView = jest.fn();
    Element.prototype.scrollIntoView = scrollIntoView;

    render(<OutputStream lines={[]} />);

    expect(screen.getByText("Waiting for output...")).toBeInTheDocument();
    expect(scrollIntoView).toHaveBeenCalled();
  });

  it("renders output lines when available", () => {
    render(<OutputStream lines={["booting", "ready"]} />);

    expect(screen.getByText("booting")).toBeInTheDocument();
    expect(screen.getByText("ready")).toBeInTheDocument();
    expect(screen.queryByText("Waiting for output...")).not.toBeInTheDocument();
  });
});
