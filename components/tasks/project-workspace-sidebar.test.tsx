jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string | number>,
  ) => {
    const map: Record<string, string> = {
      "workspace.realtimeLive": "Realtime live",
      "workspace.realtimeDegraded": "Realtime degraded",
      "empty.createTaskAction": "Create Task",
      "sidebar.views": "Views",
      "viewMode.board": "Board",
      "viewMode.list": "List",
      "viewMode.timeline": "Timeline",
      "viewMode.calendar": "Calendar",
      "viewMode.dependencies": "Dependencies",
      "viewMode.roadmap": "Roadmap",
      "sidebar.filters": "Filters",
      "workspace.resetFilters": "Reset filters",
      "filter.searchTasks": "Search tasks",
      "filter.searchPlaceholder": "Search task titles",
      "filter.status": "Status",
      "filter.priority": "Priority",
      "filter.assignee": "Assignee",
      "filter.sprint": "Sprint",
      "filter.planning": "Planning",
      "filter.dependencies": "Dependencies",
      "filter.all": "All",
      "filter.scheduled": "Scheduled",
      "filter.unscheduled": "Unscheduled",
      "sidebar.displayOptions": "Display Options",
      "workspace.comfortable": "Comfortable",
      "workspace.compact": "Compact",
      "workspace.hideDescriptions": "Hide descriptions",
      "workspace.showDescriptions": "Show descriptions",
      "workspace.hideLinkedDocs": "Hide linked docs",
      "workspace.showLinkedDocs": "Show linked docs",
    };
    if (key === "workspace.visibleTasks") {
      return `${values?.count ?? 0} visible tasks`;
    }
    return map[key] ?? key;
  },
}));

jest.mock("@/components/views/view-switcher", () => ({
  ViewSwitcher: ({ projectId }: { projectId: string }) => (
    <div data-testid="view-switcher">{projectId}</div>
  ),
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectWorkspaceSidebar } from "./project-workspace-sidebar";
import { createDefaultTaskWorkspaceFilters, useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";

describe("ProjectWorkspaceSidebar", () => {
  beforeEach(() => {
    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
      },
    });
  });

  it("renders workspace controls and updates shared store filters", async () => {
    const user = userEvent.setup();
    const onCreateTask = jest.fn();
    const onSprintFilterChange = jest.fn();

    render(
      <ProjectWorkspaceSidebar
        projectId="project-1"
        projectName="AgentForge"
        tasks={[
          { id: "task-1", title: "Build dashboard", assigneeId: "member-1", assigneeName: "Alice" },
        ] as never}
        sprints={[{ id: "sprint-1", name: "Sprint 1" }] as never}
        filteredCount={4}
        realtimeConnected
        onCreateTask={onCreateTask}
        onSprintFilterChange={onSprintFilterChange}
      />,
    );

    expect(screen.getByText("AgentForge")).toBeInTheDocument();
    expect(screen.getByText("Realtime live")).toBeInTheDocument();
    expect(screen.getByTestId("view-switcher")).toHaveTextContent("project-1");

    await user.click(screen.getByRole("button", { name: "Create Task" }));
    await user.click(screen.getByRole("button", { name: "Timeline" }));
    expect(useTaskWorkspaceStore.getState().viewMode).toBe("timeline");

    await user.type(screen.getByLabelText("Search tasks"), "dash");
    expect(useTaskWorkspaceStore.getState().filters.search).toBe("dash");

    await user.selectOptions(screen.getByLabelText("Sprint"), "sprint-1");
    expect(useTaskWorkspaceStore.getState().filters.sprintId).toBe("sprint-1");
    expect(onSprintFilterChange).toHaveBeenCalledWith("sprint-1");

    await user.click(screen.getByRole("button", { name: "Compact" }));
    expect(useTaskWorkspaceStore.getState().displayOptions.density).toBe("compact");

    await user.click(screen.getByRole("button", { name: "Hide descriptions" }));
    expect(useTaskWorkspaceStore.getState().displayOptions.showDescriptions).toBe(false);

    expect(onCreateTask).toHaveBeenCalled();
    expect(screen.getByText("4 visible tasks")).toBeInTheDocument();
  });

  it("shows reset controls only when filters are active and supports degraded realtime state", async () => {
    const user = userEvent.setup();
    useTaskWorkspaceStore.setState((state) => ({
      ...state,
      filters: {
        ...state.filters,
        search: "calendar",
      },
    }));

    render(
      <ProjectWorkspaceSidebar
        projectId="project-1"
        projectName="AgentForge"
        tasks={[]}
        sprints={[]}
        filteredCount={0}
        realtimeConnected={false}
      />,
    );

    expect(screen.getByText("Realtime degraded")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Reset filters" }));
    expect(useTaskWorkspaceStore.getState().filters.search).toBe("");
  });
});
