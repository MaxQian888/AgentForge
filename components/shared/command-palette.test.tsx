const pushMock = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({
    push: pushMock,
  }),
}));

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "nav.group.workspace": "Workspace",
      "nav.group.project": "Project",
      "nav.group.operations": "Operations",
      "nav.group.configuration": "Configuration",
      "nav.dashboard": "Dashboard",
      "nav.projects": "Projects",
      "nav.projectDashboard": "Project Dashboard",
      "nav.team": "Team",
      "nav.agents": "Agents",
      "nav.teams": "Teams",
      "nav.sprints": "Sprints",
      "nav.reviews": "Reviews",
      "nav.cost": "Cost",
      "nav.scheduler": "Scheduler",
      "nav.workflow": "Workflow",
      "nav.memory": "Memory",
      "nav.roles": "Roles",
      "nav.plugins": "Plugins",
      "nav.settings": "Settings",
      "nav.imBridge": "IM Bridge",
      "nav.docs": "Docs",
      "commandPalette.placeholder": "Search commands",
      "commandPalette.noResults": "No results",
      "commandPalette.actions": "Quick Actions",
      "commandPalette.createProject": "Create project",
      "commandPalette.createTask": "Create task",
      "commandPalette.spawnAgent": "Spawn agent",
      "commandPalette.createTeam": "Create team",
    };
    return map[key] ?? key;
  },
}));

jest.mock("@/components/ui/command", () => ({
  CommandDialog: ({
    open,
    children,
  }: {
    open: boolean;
    children?: React.ReactNode;
  }) => (open ? <div data-testid="command-dialog">{children}</div> : null),
  CommandInput: ({ placeholder }: { placeholder?: string }) => (
    <input aria-label="command-input" placeholder={placeholder} />
  ),
  CommandList: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  CommandEmpty: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  CommandGroup: ({
    heading,
    children,
  }: {
    heading?: React.ReactNode;
    children?: React.ReactNode;
  }) => (
    <section>
      <h2>{heading}</h2>
      {children}
    </section>
  ),
  CommandItem: ({
    children,
    onSelect,
  }: {
    children?: React.ReactNode;
    onSelect?: (value: string) => void;
  }) => (
    <button type="button" onClick={() => onSelect?.("")}>
      {children}
    </button>
  ),
  CommandSeparator: () => <hr />,
}));

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CommandPalette } from "./command-palette";

describe("CommandPalette", () => {
  beforeEach(() => {
    pushMock.mockReset();
  });

  it("navigates to selected routes and closes the palette", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(<CommandPalette open onOpenChange={onOpenChange} />);

    expect(screen.getByTestId("command-dialog")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Search commands")).toBeInTheDocument();
    expect(screen.getByText("Workspace")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Projects" }));

    expect(pushMock).toHaveBeenCalledWith("/projects");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("toggles open state from the keyboard shortcut", () => {
    const onOpenChange = jest.fn();

    render(<CommandPalette open={false} onOpenChange={onOpenChange} />);

    fireEvent.keyDown(document, { key: "k", ctrlKey: true });

    expect(onOpenChange).toHaveBeenCalledWith(true);
  });
});
