const pushMock = jest.fn();
const pathnameMock = jest.fn();
const searchParamsMock = jest.fn();
const setThemeMock = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({
    push: pushMock,
  }),
  usePathname: () => pathnameMock(),
  useSearchParams: () => searchParamsMock(),
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
      "commandPalette.recent": "Recent",
      "commandPalette.history": "Recent Commands",
      "commandPalette.context": "Context",
      "commandPalette.createProject": "Create project",
      "commandPalette.createTask": "Create task",
      "commandPalette.spawnAgent": "Spawn agent",
      "commandPalette.createTeam": "Create team",
      "commandPalette.toggleTheme": "Toggle Theme",
    };
    return map[key] ?? key;
  },
}));

jest.mock("@/lib/theme/provider", () => ({
  useTheme: () => ({
    theme: "light",
    resolvedTheme: "light",
    setTheme: setThemeMock,
  }),
}));

jest.mock("@/components/ui/command", () => ({
  CommandDialog: ({
    open,
    children,
  }: {
    open: boolean;
    children?: React.ReactNode;
    commandProps?: unknown;
  }) => (open ? <div data-testid="command-dialog">{children}</div> : null),
  CommandInput: ({
    placeholder,
    value,
    onValueChange,
  }: {
    placeholder?: string;
    value?: string;
    onValueChange?: (value: string) => void;
  }) => (
    <input
      aria-label="command-input"
      placeholder={placeholder}
      value={value}
      onChange={(event) => onValueChange?.(event.target.value)}
    />
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
import { useLayoutStore } from "@/lib/stores/layout-store";
import { CommandPalette } from "./command-palette";

describe("CommandPalette", () => {
  beforeEach(() => {
    pushMock.mockReset();
    pathnameMock.mockReturnValue("/");
    searchParamsMock.mockReturnValue(new URLSearchParams());
    setThemeMock.mockReset();
    useLayoutStore.setState({
      breadcrumbs: [],
      commandPaletteOpen: false,
      recentCommands: [],
    });
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

  it("shows recently used commands after a selection is recorded", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    const { rerender } = render(<CommandPalette open onOpenChange={onOpenChange} />);

    await user.click(screen.getByRole("button", { name: "Projects" }));

    rerender(<CommandPalette open onOpenChange={onOpenChange} />);

    expect(screen.getByText("Recent")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Projects" })).toHaveLength(2);
  });

  it("keeps contextual theme commands available on the settings page", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();
    pathnameMock.mockReturnValue("/settings");

    render(<CommandPalette open onOpenChange={onOpenChange} />);

    await user.type(screen.getByLabelText("command-input"), "theme");

    expect(screen.getByText("Context")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Toggle Theme" }));

    expect(setThemeMock).toHaveBeenCalledWith("dark");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("matches fuzzy navigation queries with common typos", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(<CommandPalette open onOpenChange={onOpenChange} />);

    await user.type(screen.getByLabelText("command-input"), "agnets");

    expect(screen.getByRole("button", { name: "Agents" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Projects" })).not.toBeInTheDocument();
  });
});
