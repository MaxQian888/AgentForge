import { render, screen } from "@testing-library/react";
import { SectionCard } from "./section-card";

describe("SectionCard", () => {
  it("renders title, description, actions, body, and footer", () => {
    render(
      <SectionCard
        title="Recent Activity"
        description="Events from the last 24 hours."
        actions={<button type="button">Refresh</button>}
        footer={<span data-testid="footer">Updated just now</span>}
      >
        <p>Body content</p>
      </SectionCard>,
    );

    expect(screen.getByRole("heading", { name: "Recent Activity" })).toBeInTheDocument();
    expect(screen.getByText("Events from the last 24 hours.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Refresh" })).toBeInTheDocument();
    expect(screen.getByText("Body content")).toBeInTheDocument();
    expect(screen.getByTestId("footer")).toBeInTheDocument();
  });

  it("skips the header when no title, description, or actions are provided", () => {
    render(
      <SectionCard>
        <span data-testid="body-only">body only</span>
      </SectionCard>,
    );

    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
    expect(screen.getByTestId("body-only")).toBeInTheDocument();
  });

  it("honors the `as` prop for semantic markup", () => {
    const { container } = render(
      <SectionCard as="article" title="Article title">
        <p>body</p>
      </SectionCard>,
    );

    expect(container.firstChild?.nodeName).toBe("ARTICLE");
  });
});
