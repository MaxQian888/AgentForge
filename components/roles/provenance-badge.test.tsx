import { render, screen } from "@testing-library/react";
import { ProvenanceBadge } from "./provenance-badge";

describe("ProvenanceBadge", () => {
  it("renders the provenance label with the mapped style", () => {
    render(<ProvenanceBadge provenance="template" className="extra" />);

    const badge = screen.getByText("template");
    expect(badge).toHaveClass("bg-purple-500/15");
    expect(badge).toHaveClass("extra");
  });
});
