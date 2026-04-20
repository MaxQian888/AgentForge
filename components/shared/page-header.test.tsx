jest.mock("next/link", () => ({
  __esModule: true,
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

import { render, screen } from "@testing-library/react";
import { PageHeader } from "./page-header";

describe("PageHeader", () => {
  it("renders breadcrumbs, title, description, actions, and sticky classes", () => {
    const { container } = render(
      <PageHeader
        breadcrumbs={[{ label: "Home", href: "/" }, { label: "Projects" }]}
        title="Projects"
        description="Track active delivery work."
        actions={<button type="button">Create</button>}
        sticky
        className="custom-header"
      />,
    );

    expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Projects" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByText("Track active delivery work.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create" })).toBeInTheDocument();
    expect(container.firstChild).toHaveClass("sticky");
    expect(container.firstChild).toHaveClass("custom-header");
  });

  it("renders the status slot next to the title", () => {
    render(
      <PageHeader
        title="Agents"
        status={<span data-testid="status-ribbon">3 running</span>}
      />,
    );

    expect(screen.getByTestId("status-ribbon")).toHaveTextContent("3 running");
    expect(screen.getByRole("heading", { name: "Agents" })).toBeInTheDocument();
  });

  it("renders the filters slot below the header row", () => {
    render(
      <PageHeader
        title="Tasks"
        filters={<div data-testid="filters-row">filters go here</div>}
      />,
    );

    expect(screen.getByTestId("filters-row")).toBeInTheDocument();
  });
});
