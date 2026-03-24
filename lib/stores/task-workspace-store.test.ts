import {
  createDefaultTaskWorkspaceFilters,
  useTaskWorkspaceStore,
} from "./task-workspace-store";

describe("useTaskWorkspaceStore", () => {
  beforeEach(() => {
    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: null,
      contextRailDisplay: "expanded",
    });
  });

  it("keeps shared view, filter, and selection state for the task workspace", () => {
    const store = useTaskWorkspaceStore.getState();

    store.setViewMode("calendar");
    store.setSearch("timeline");
    store.setStatus("in_progress");
    store.setPriority("high");
    store.setAssigneeId("member-1");
    store.setPlanning("scheduled");
    store.selectTask("task-1");
    store.setContextRailDisplay("collapsed");

    const state = useTaskWorkspaceStore.getState();

    expect(state.viewMode).toBe("calendar");
    expect(state.filters).toMatchObject({
      search: "timeline",
      status: "in_progress",
      priority: "high",
      assigneeId: "member-1",
      planning: "scheduled",
    });
    expect(state.selectedTaskId).toBe("task-1");
    expect(state.contextRailDisplay).toBe("collapsed");
  });
});
