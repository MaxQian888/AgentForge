import userEvent from "@testing-library/user-event";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryPanel } from "./memory-panel";

const loadWorkspace = jest.fn().mockResolvedValue(undefined);
const setFilters = jest.fn();
const resetFilters = jest.fn();
const setPagination = jest.fn();
const fetchMemoryDetail = jest.fn().mockResolvedValue(undefined);
const selectMemory = jest.fn();
const toggleMemorySelection = jest.fn();
const clearSelection = jest.fn();
const deleteMemory = jest.fn().mockResolvedValue(undefined);
const bulkDeleteMemories = jest.fn().mockResolvedValue({ deletedCount: 2 });
const bulkDeleteByCriteria = jest.fn().mockResolvedValue({ deletedCount: 4 });
const cleanupMemories = jest.fn().mockResolvedValue({ deletedCount: 3 });
const exportMemories = jest.fn().mockResolvedValue({
  projectId: "project-1",
  exportedAt: "2026-04-13T12:00:00.000Z",
  entries: [{ id: "memory-1", key: "design-note" }],
});
const exportMemoryEntry = jest.fn().mockResolvedValue({
  id: "memory-1",
  projectId: "project-1",
  scope: "project",
  roleId: "",
  category: "episodic",
  kind: "operator_note",
  tags: ["ops", "release"],
  editable: true,
  key: "design-note",
  content: "Editable note",
  metadata: "{}",
  metadataObject: { kind: "operator_note", tags: ["ops", "release"] },
  relatedContext: [{ type: "task", id: "task-1", label: "Related task" }],
  relevanceScore: 0.9,
  accessCount: 2,
  createdAt: "2026-03-25T08:00:00.000Z",
  updatedAt: "2026-03-25T08:30:00.000Z",
});
const storeMemory = jest.fn().mockResolvedValue(undefined);
const updateMemory = jest.fn().mockResolvedValue(undefined);
const addMemoryTag = jest.fn().mockResolvedValue(undefined);
const removeMemoryTag = jest.fn().mockResolvedValue(undefined);
const buildExportBlob = jest
  .fn()
  .mockImplementation(
    (
      _payload: unknown,
      format: "json" | "csv",
    ): { content: string; mimeType: string; extension: "json" | "csv" } => ({
      content: format === "csv" ? "id,key\nmemory-1,design-note" : "{}",
      mimeType: format === "csv" ? "text/csv" : "application/json",
      extension: format,
    }),
  );
const clearActionFeedback = jest.fn();

const storeState = {
  currentProjectId: "project-1",
  filters: {
    query: "",
    scope: "all",
    category: "all",
    roleId: "",
    tag: "",
    startAt: "",
    endAt: "",
    limit: 20,
  },
  pagination: { page: 1, pageSize: 10 },
  entries: [
    {
      id: "memory-1",
      projectId: "project-1",
      scope: "project" as const,
      roleId: "",
      category: "episodic" as const,
      kind: "operator_note",
      tags: ["ops", "release"],
      editable: true,
      key: "design-note",
      content: "Keep the review queue stable.",
      metadata: "{}",
      metadataObject: { source: "ops" },
      relatedContext: [{ type: "task", id: "task-1", label: "Related task" }],
      relevanceScore: 0.9,
      accessCount: 2,
      createdAt: "2026-03-25T08:00:00.000Z",
      updatedAt: "2026-03-25T08:30:00.000Z",
    },
    {
      id: "memory-2",
      projectId: "project-1",
      scope: "role" as const,
      roleId: "reviewer",
      category: "episodic" as const,
      kind: "",
      tags: ["feedback"],
      editable: false,
      key: "incident-log",
      content: "Remember reviewer escalations.",
      metadata: "{\"source\":\"feedback\"}",
      metadataObject: { source: "feedback", sessionId: "session-7" },
      relatedContext: [{ type: "session", id: "session-7", label: "Related session" }],
      relevanceScore: 0.8,
      accessCount: 4,
      createdAt: "2026-03-25T09:00:00.000Z",
      updatedAt: "2026-03-25T09:45:00.000Z",
    },
  ],
  stats: {
    totalCount: 2,
    approxStorageBytes: 2048,
    byCategory: { semantic: 1, episodic: 1 },
    byScope: { project: 1, role: 1 },
    oldestCreatedAt: "2026-03-25T08:00:00.000Z",
    newestCreatedAt: "2026-03-25T09:00:00.000Z",
    lastAccessedAt: "2026-03-25T10:15:00.000Z",
  },
  detail: null as null | {
    id: string;
    projectId?: string;
    scope?: string;
    roleId?: string;
    category?: string;
    kind?: string;
    tags?: string[];
    editable?: boolean;
    key: string;
    content: string;
    metadata?: string;
    metadataObject?: Record<string, unknown> | null;
    relatedContext?: { type: string; id: string; label?: string }[];
    relevanceScore?: number;
    accessCount?: number;
    createdAt?: string;
    updatedAt?: string;
  },
  selectedMemoryId: null as string | null,
  selectedMemoryIds: [] as string[],
  loading: false,
  statsLoading: false,
  detailLoading: false,
  actionLoading: false,
  error: null as string | null,
  statsError: null as string | null,
  detailError: null as string | null,
  actionError: null as string | null,
  lastMutation: null as null | { type: string; deletedCount: number },
  loadWorkspace,
  setFilters,
  resetFilters,
  setPagination,
  fetchMemoryDetail,
  selectMemory,
  toggleMemorySelection,
  clearSelection,
  storeMemory,
  updateMemory,
  deleteMemory,
  bulkDeleteMemories,
  bulkDeleteByCriteria,
  cleanupMemories,
  exportMemories,
  exportMemoryEntry,
  addMemoryTag,
  removeMemoryTag,
  buildExportBlob,
  clearActionFeedback,
};

const roleStoreState = {
  roles: [
    {
      metadata: {
        id: "reviewer",
        name: "Reviewer",
      },
    },
    {
      metadata: {
        id: "planner",
        name: "Planner",
      },
    },
  ],
  fetchRoles: jest.fn().mockResolvedValue(undefined),
};

jest.mock("@/lib/stores/memory-store", () => ({
  useMemoryStore: (selector: (state: typeof storeState) => unknown) =>
    selector(storeState),
}));

jest.mock("@/lib/stores/role-store", () => ({
  useRoleStore: (selector: (state: typeof roleStoreState) => unknown) =>
    selector(roleStoreState),
}));

jest.mock("@/hooks/use-breakpoint", () => ({
  useBreakpoint: () => ({
    breakpoint: "desktop",
    isMobile: false,
    isTablet: false,
    isDesktop: true,
  }),
}));

describe("MemoryPanel", () => {
  let consoleErrorSpy: jest.SpyInstance;

  beforeEach(() => {
    consoleErrorSpy = jest
      .spyOn(console, "error")
      .mockImplementation((message?: unknown, ...args: unknown[]) => {
        const text = String(message ?? "");
        if (
          text.includes("not wrapped in act") ||
          text.includes("suspended inside an `act` scope")
        ) {
          return;
        }
        jest.requireActual("console").error(message, ...args);
      });

    loadWorkspace.mockClear();
    setFilters.mockClear();
    resetFilters.mockClear();
    setPagination.mockClear();
    fetchMemoryDetail.mockClear();
    selectMemory.mockClear();
    toggleMemorySelection.mockClear();
    clearSelection.mockClear();
    deleteMemory.mockClear();
    bulkDeleteMemories.mockClear();
    bulkDeleteByCriteria.mockClear();
    cleanupMemories.mockClear();
    exportMemories.mockClear();
    exportMemoryEntry.mockClear();
    storeMemory.mockClear();
    updateMemory.mockClear();
    addMemoryTag.mockClear();
    removeMemoryTag.mockClear();
    buildExportBlob.mockClear();
    clearActionFeedback.mockClear();
    roleStoreState.fetchRoles.mockClear();

    storeState.filters = {
      query: "",
      scope: "all",
      category: "all",
      roleId: "",
      tag: "",
      startAt: "",
      endAt: "",
      limit: 20,
    };
    storeState.pagination = { page: 1, pageSize: 10 };
    storeState.selectedMemoryId = null;
    storeState.selectedMemoryIds = [];
    storeState.detail = null;
    storeState.detailLoading = false;
    storeState.actionError = null;
    storeState.lastMutation = null;
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
  });

  it("loads the workspace, renders summary stats, and updates filters", async () => {
    const user = userEvent.setup();
    render(<MemoryPanel projectId="project-1" />);

    await waitFor(() => expect(loadWorkspace).toHaveBeenCalledWith("project-1"));

    expect(screen.getByTestId("memory-stat-total")).toHaveTextContent("2");
    expect(screen.getByText("design-note")).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText("Search memory entries..."), "queue");
    expect(setFilters).toHaveBeenLastCalledWith({ query: "queue" });

    await user.click(screen.getByRole("combobox", { name: "Scope" }));
    await user.click(screen.getByRole("option", { name: "Role scoped" }));
    expect(setFilters).toHaveBeenLastCalledWith({
      scope: "role",
      roleId: "reviewer",
    });

    await user.click(screen.getByRole("combobox", { name: "Result window" }));
    await user.click(screen.getByRole("option", { name: "50" }));
    expect(setFilters).toHaveBeenLastCalledWith({ limit: 50 });
  });

  it("requests detail inspection for the selected entry and renders detail content", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<MemoryPanel projectId="project-1" />);

    await user.click(screen.getByRole("button", { name: "Open design-note" }));

    expect(selectMemory).toHaveBeenCalledWith("memory-1");
    expect(fetchMemoryDetail).toHaveBeenCalledWith("project-1", "memory-1", undefined);

    storeState.selectedMemoryId = "memory-1";
    storeState.detail = {
      id: "memory-1",
      key: "design-note",
      content: "Full design note for reviewers.",
      scope: "project",
      roleId: "",
      category: "episodic",
      tags: [],
      editable: false,
      metadataObject: { source: "ops", taskId: "task-1" },
      relatedContext: [{ type: "task", id: "task-1", label: "Related task" }],
    };
    rerender(<MemoryPanel projectId="project-1" />);

    expect(screen.getByTestId("memory-detail-panel")).toHaveTextContent(
      "Full design note for reviewers.",
    );
    expect(screen.getByText("Related task")).toBeInTheDocument();
    expect(screen.getByText(/source/)).toBeInTheDocument();
  });

  it("supports export and bulk delete flows with confirmation", async () => {
    const user = userEvent.setup();
    storeState.selectedMemoryIds = ["memory-1", "memory-2"];

    render(<MemoryPanel projectId="project-1" />);

    await user.click(screen.getByRole("button", { name: /Export JSON/ }));
    expect(exportMemories).toHaveBeenCalledWith("project-1");
    expect(buildExportBlob).toHaveBeenCalledWith(expect.any(Object), "json");

    await user.click(screen.getByRole("button", { name: "Delete Selected (2)" }));
    await user.click(screen.getByRole("button", { name: "Confirm Bulk Delete" }));

    expect(bulkDeleteMemories).toHaveBeenCalledWith(
      "project-1",
      ["memory-1", "memory-2"],
      undefined,
    );
  });

  it("exports CSV using the shared blob builder", async () => {
    const user = userEvent.setup();
    render(<MemoryPanel projectId="project-1" />);

    await user.click(screen.getByRole("button", { name: /Export CSV/ }));
    expect(exportMemories).toHaveBeenCalledWith("project-1");
    expect(buildExportBlob).toHaveBeenCalledWith(expect.any(Object), "csv");
  });

  it("confirms bulk delete by criteria for the current filter window", async () => {
    const user = userEvent.setup();
    render(<MemoryPanel projectId="project-1" />);

    await user.click(screen.getByRole("button", { name: /Delete Filtered \(2\)/ }));
    await user.click(screen.getByRole("button", { name: "Confirm Filtered Delete" }));

    expect(bulkDeleteByCriteria).toHaveBeenCalledWith("project-1", {
      ids: ["memory-1", "memory-2"],
      roleId: undefined,
    });
  });

  it("paginates the memory list with navigation controls", async () => {
    const user = userEvent.setup();
    storeState.pagination = { page: 1, pageSize: 1 };

    render(<MemoryPanel projectId="project-1" />);

    expect(screen.getByTestId("memory-page-indicator")).toHaveTextContent(
      "Page 1 of 2",
    );
    expect(screen.getByText("design-note")).toBeInTheDocument();
    expect(screen.queryByText("incident-log")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Next page" }));
    expect(setPagination).toHaveBeenCalledWith({ page: 2 });
  });

  it("highlights matching search terms within entries", () => {
    storeState.filters = {
      ...storeState.filters,
      query: "queue",
    };

    render(<MemoryPanel projectId="project-1" />);

    const highlights = screen
      .getAllByText("queue", { selector: "mark" });
    expect(highlights.length).toBeGreaterThan(0);
  });

  it("removes a tag via the remove button on editable entries", async () => {
    const user = userEvent.setup();
    render(<MemoryPanel projectId="project-1" />);

    const removeButton = screen.getAllByRole("button", {
      name: "Remove tag ops",
    })[0];
    await user.click(removeButton);

    expect(removeMemoryTag).toHaveBeenCalledWith("project-1", "memory-1", "ops");
  });

  it("supports note authoring, tag filtering, and editable note actions", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<MemoryPanel projectId="project-1" />);

    fireEvent.change(screen.getByLabelText("Note title"), {
      target: { value: "Release note" },
    });
    fireEvent.change(screen.getByLabelText("Note content"), {
      target: { value: "Remember the rollout checklist." },
    });
    fireEvent.change(screen.getByLabelText("Note tags"), {
      target: { value: "ops, release" },
    });
    await user.click(screen.getByRole("button", { name: "Create Note" }));

    expect(storeMemory).toHaveBeenCalledWith("project-1", {
      key: "Release note",
      content: "Remember the rollout checklist.",
      scope: "project",
      category: "episodic",
      kind: "operator_note",
      tags: ["ops", "release"],
    });

    fireEvent.change(screen.getByLabelText("Tag"), {
      target: { value: "ops" },
    });
    expect(setFilters).toHaveBeenLastCalledWith({ tag: "ops" });

    storeState.selectedMemoryId = "memory-1";
    storeState.detail = {
      id: "memory-1",
      projectId: "project-1",
      scope: "project",
      roleId: "",
      category: "episodic",
      kind: "operator_note",
      tags: ["ops", "release"],
      editable: true,
      key: "design-note",
      content: "Editable note",
      metadata: "{}",
      metadataObject: { kind: "operator_note", tags: ["ops", "release"] },
      relatedContext: [{ type: "task", id: "task-1", label: "Related task" }],
      relevanceScore: 0.9,
      accessCount: 2,
      createdAt: "2026-03-25T08:00:00.000Z",
      updatedAt: "2026-03-25T08:30:00.000Z",
    };
    rerender(<MemoryPanel projectId="project-1" />);

    await user.click(screen.getByRole("button", { name: "Edit Note" }));
    fireEvent.change(screen.getByLabelText("Edit note tags"), {
      target: { value: "ops, release, pinned" },
    });
    await user.click(screen.getByRole("button", { name: "Save Note" }));

    expect(updateMemory).toHaveBeenCalledWith("project-1", "memory-1", {
      key: "design-note",
      content: "Editable note",
      tags: ["ops", "release", "pinned"],
      roleId: undefined,
    });

    await user.click(screen.getByRole("button", { name: "Export Entry" }));
    expect(exportMemoryEntry).toHaveBeenCalledWith(
      "project-1",
      "memory-1",
      undefined,
    );
    expect(buildExportBlob).toHaveBeenCalledWith(expect.any(Object), "json");
  }, 10000);

  it("adds a new tag to the selected memory from the detail panel", async () => {
    const user = userEvent.setup();
    storeState.selectedMemoryId = "memory-1";
    storeState.detail = {
      id: "memory-1",
      projectId: "project-1",
      scope: "project",
      roleId: "",
      category: "episodic",
      kind: "operator_note",
      tags: ["ops"],
      editable: true,
      key: "design-note",
      content: "Editable note",
      metadata: "{}",
      metadataObject: {},
      relatedContext: [],
      relevanceScore: 0.9,
      accessCount: 2,
      createdAt: "2026-03-25T08:00:00.000Z",
      updatedAt: "2026-03-25T08:30:00.000Z",
    };

    render(<MemoryPanel projectId="project-1" />);

    fireEvent.change(screen.getByLabelText("Add tag"), {
      target: { value: "retrospective" },
    });
    await user.click(screen.getByRole("button", { name: /^Add$/ }));

    expect(addMemoryTag).toHaveBeenCalledWith(
      "project-1",
      "memory-1",
      "retrospective",
    );
  });
});
