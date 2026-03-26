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

const usePathnameMock = jest.fn();

jest.mock("next/navigation", () => ({
  usePathname: () => usePathnameMock(),
}));

import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { MobileSidebar, Sidebar } from "./sidebar";

describe("Sidebar", () => {
  beforeEach(() => {
    document.documentElement.className = "";
    usePathnameMock.mockReturnValue("/projects");
  });

  it("highlights the active route and toggles dark mode", async () => {
    const user = userEvent.setup();
    const { container } = render(<Sidebar />);

    const projectsLink = screen.getByRole("link", { name: /Projects/i });
    const dashboardLink = screen.getByRole("link", { name: /^Dashboard$/i });
    expect(projectsLink).toHaveClass("bg-accent");
    expect(dashboardLink).toHaveClass("text-muted-foreground");

    await user.click(container.querySelector("button")!);
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("opens the mobile sheet navigation", async () => {
    const user = userEvent.setup();
    const { container } = render(<MobileSidebar />);

    await user.click(container.querySelector("button")!);

    expect(await screen.findAllByText("AgentForge")).not.toHaveLength(0);
    expect(screen.getAllByRole("link", { name: /Projects/i })[0]).toHaveAttribute(
      "href",
      "/projects",
    );
  });
});
