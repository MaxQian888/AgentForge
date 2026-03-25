import { render, screen } from "@testing-library/react";
import { Badge, badgeVariants } from "./badge";

describe("Badge", () => {
  it("renders the default badge styles", () => {
    render(<Badge>Status</Badge>);

    const badge = screen.getByText("Status");
    expect(badge).toHaveAttribute("data-slot", "badge");
    expect(badge).toHaveAttribute("data-variant", "default");
    expect(badge).toHaveClass("bg-primary");
  });

  it("supports asChild composition and custom variants", () => {
    render(
      <Badge asChild variant="outline">
        <a href="/roles">Roles</a>
      </Badge>,
    );

    const link = screen.getByRole("link", { name: "Roles" });
    expect(link).toHaveAttribute("href", "/roles");
    expect(link).toHaveAttribute("data-slot", "badge");
    expect(link).toHaveAttribute("data-variant", "outline");
    expect(link).toHaveClass("border-border");
  });
});

describe("badgeVariants", () => {
  it("returns the right classes for a given variant", () => {
    const className = badgeVariants({ variant: "secondary" });
    expect(className).toContain("bg-secondary");
    expect(className).toContain("text-secondary-foreground");
  });
});
