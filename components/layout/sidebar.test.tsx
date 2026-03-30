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

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      appName: "AgentForge",
      "nav.dashboard": "Dashboard",
      "nav.projects": "Projects",
      "nav.projectDashboard": "Project Dashboard",
      "nav.team": "Team",
      "nav.agents": "Agents",
      "nav.teams": "Teams",
      "nav.sprints": "Sprints",
      "nav.reviews": "Reviews",
      "nav.scheduler": "Scheduler",
      "nav.cost": "Cost",
      "nav.memory": "Memory",
      "nav.docs": "Docs",
      "nav.imBridge": "IM Bridge",
      "nav.roles": "Roles",
      "nav.plugins": "Plugins",
      "nav.settings": "Settings",
    };
    return map[key] ?? key;
  },
}));

// Mock next-themes used by ThemeToggle
jest.mock("next-themes", () => ({
  useTheme: () => ({ theme: "light", setTheme: jest.fn() }),
}));

import { render, screen } from "@testing-library/react";
import { SidebarProvider } from "@/components/ui/sidebar";
import { AppSidebar } from "./sidebar";

function renderWithProvider() {
  return render(
    <SidebarProvider>
      <AppSidebar />
    </SidebarProvider>
  );
}

describe("AppSidebar", () => {
  beforeEach(() => {
    usePathnameMock.mockReturnValue("/projects");
  });

  it("renders nav links", () => {
    renderWithProvider();
    expect(screen.getByRole("link", { name: /Projects/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /^Dashboard$/i })).toBeInTheDocument();
  });

  it("marks the active route with data-active", () => {
    renderWithProvider();
    const projectsLink = screen.getByRole("link", { name: /^Projects$/i });
    // The SidebarMenuButton wraps the link, check the parent has data-active
    expect(projectsLink.closest("[data-active='true']")).not.toBeNull();
  });

  it("matches sidebar items by route segment so /teams does not activate /team", () => {
    usePathnameMock.mockReturnValue("/teams/detail");

    renderWithProvider();

    const teamLink = screen.getByRole("link", { name: /^Team$/i });
    const teamsLink = screen.getByRole("link", { name: /^Teams$/i });

    expect(teamLink.closest("[data-active='true']")).toBeNull();
    expect(teamsLink.closest("[data-active='true']")).not.toBeNull();
  });

  it("renders the app name in header", () => {
    renderWithProvider();
    expect(screen.getByText("AgentForge")).toBeInTheDocument();
  });
});
