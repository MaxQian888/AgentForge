import {
  createDefaultTaskWorkspaceFilters,
  useTaskWorkspaceStore,
} from "./task-workspace-store";

describe("useTaskWorkspaceStore", () => {
  beforeEach(() => {
    localStorage.clear();
    useTaskWorkspaceStore.setState({
      viewMode: "board",
      filters: createDefaultTaskWorkspaceFilters(),
      selectedTaskId: null,
      selectedTaskIds: [],
      contextRailDisplay: "expanded",
      displayOptions: {
        density: "comfortable",
        showDescriptions: true,
        showLinkedDocs: false,
        boardColumnOrder: [
          "inbox",
          "triaged",
          "assigned",
          "in_progress",
          "blocked",
          "in_review",
          "changes_requested",
          "done",
          "cancelled",
          "budget_exceeded",
        ],
        hiddenBoardColumns: [],
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
    store.setDueDateRange({ start: "2026-03-25", end: "2026-03-31" });
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
      dueDateStart: "2026-03-25",
      dueDateEnd: "2026-03-31",
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
      boardColumnOrder: [
        "inbox",
        "triaged",
        "assigned",
        "in_progress",
        "blocked",
        "in_review",
        "changes_requested",
        "done",
        "cancelled",
        "budget_exceeded",
      ],
      hiddenBoardColumns: [],
    });
  });

  it("can clear a custom field filter without resetting the rest of the workspace state", () => {
    const store = useTaskWorkspaceStore.getState();

    store.setCustomFieldFilter("field-risk", "High");
    store.setCustomFieldFilter("field-risk", "all");

    expect(useTaskWorkspaceStore.getState().filters.customFieldFilters).toEqual({});
  });

  it("tracks sprint and label filters directly", () => {
    const store = useTaskWorkspaceStore.getState();

    store.setSprintId("sprint-2");
    store.setLabels(["frontend", "release"]);

    expect(useTaskWorkspaceStore.getState().filters).toMatchObject({
      sprintId: "sprint-2",
      labels: ["frontend", "release"],
    });
  });

  it("applies saved-view config aliases and custom field filters", () => {
    const store = useTaskWorkspaceStore.getState();
    store.setLabels(["existing-label"]);

    store.applySavedViewConfig({
      layout: "timeline",
      filters: [
        { field: "status", value: "done" },
        { field: "priority", value: "low" },
        { field: "assignee_id", value: "member-2" },
        { field: "due_date_start", value: "2026-03-20" },
        { field: "due_date_end", value: "2026-03-28" },
        { field: "sprint_id", value: "sprint-7" },
        { field: "search", value: "release" },
        { field: "cf:field-risk", value: "Critical" },
        { field: "ignored", value: "ignored" },
        null,
      ],
    });

    expect(useTaskWorkspaceStore.getState()).toMatchObject({
      viewMode: "timeline",
      filters: {
        status: "done",
        priority: "low",
        assigneeId: "member-2",
        dueDateStart: "2026-03-20",
        dueDateEnd: "2026-03-28",
        sprintId: "sprint-7",
        search: "release",
        labels: ["existing-label"],
        customFieldFilters: {
          "field-risk": "Critical",
        },
      },
    });
  });

  it("ignores invalid saved-view configs", () => {
    const store = useTaskWorkspaceStore.getState();
    store.setSearch("current");

    store.applySavedViewConfig(null);
    store.applySavedViewConfig("invalid");

    expect(useTaskWorkspaceStore.getState()).toMatchObject({
      viewMode: "board",
      filters: expect.objectContaining({
        search: "current",
      }),
    });
  });

  it("toggles multi-selection and can replace or clear the visible selection", () => {
    const store = useTaskWorkspaceStore.getState();

    store.toggleTaskSelection("task-1");
    store.toggleTaskSelection("task-2");
    store.toggleTaskSelection("task-1");

    expect(useTaskWorkspaceStore.getState().selectedTaskIds).toEqual(["task-2"]);

    store.selectAllVisible(["task-3", "task-4"]);
    expect(useTaskWorkspaceStore.getState().selectedTaskIds).toEqual([
      "task-3",
      "task-4",
    ]);

    store.clearSelection();
    expect(useTaskWorkspaceStore.getState().selectedTaskIds).toEqual([]);
  });

  it("persists board column order and hidden columns", () => {
    const store = useTaskWorkspaceStore.getState();

    store.setBoardColumnOrder([
      "done",
      "inbox",
      "triaged",
      "assigned",
      "in_progress",
      "blocked",
      "in_review",
      "changes_requested",
      "cancelled",
      "budget_exceeded",
    ]);
    store.setHiddenBoardColumns(["blocked", "cancelled"]);

    const state = useTaskWorkspaceStore.getState();

    expect(state.displayOptions.boardColumnOrder?.[0]).toBe("done");
    expect(state.displayOptions.hiddenBoardColumns).toEqual(["blocked", "cancelled"]);
    expect(localStorage.getItem("task-workspace-board-columns")).toContain("\"done\"");
    expect(localStorage.getItem("task-workspace-hidden-columns")).toContain("\"blocked\"");
  });
});
