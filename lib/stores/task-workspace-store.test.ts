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
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
      },
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
    store.setDependency("blocked");
    store.setCustomFieldFilter("field-risk", "High");
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
      dependency: "blocked",
      customFieldFilters: { "field-risk": "High" },
    });
    expect(state.selectedTaskId).toBe("task-1");
    expect(state.contextRailDisplay).toBe("collapsed");
  });

  it("supports dependencies view mode", () => {
    const store = useTaskWorkspaceStore.getState();
    store.setViewMode("dependencies");
    expect(useTaskWorkspaceStore.getState().viewMode).toBe("dependencies");
  });

  it("tracks shared display options separately from filters", () => {
    const store = useTaskWorkspaceStore.getState();

    store.setDensity("compact");
    store.setShowDescriptions(false);
    store.setShowLinkedDocs(true);
    store.setSearch("calendar");
    store.resetFilters();

    const state = useTaskWorkspaceStore.getState();

    expect(state.filters).toEqual(createDefaultTaskWorkspaceFilters());
    expect(state.displayOptions).toEqual({
      density: "compact",
      showDescriptions: false,
      showLinkedDocs: true,
    });
  });

  it("can clear a custom field filter without resetting the rest of the workspace state", () => {
    const store = useTaskWorkspaceStore.getState();

    store.setCustomFieldFilter("field-risk", "High");
    store.setCustomFieldFilter("field-risk", "all");

    expect(useTaskWorkspaceStore.getState().filters.customFieldFilters).toEqual({});
  });
});
