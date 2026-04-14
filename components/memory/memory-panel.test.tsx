import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryPanel } from "./memory-panel";

const loadWorkspace = jest.fn().mockResolvedValue(undefined);
const setFilters = jest.fn();
const fetchMemoryDetail = jest.fn().mockResolvedValue(undefined);
const selectMemory = jest.fn();
const toggleMemorySelection = jest.fn();
const clearSelection = jest.fn();
const deleteMemory = jest.fn().mockResolvedValue(undefined);
const bulkDeleteMemories = jest.fn().mockResolvedValue({ deletedCount: 2 });
const cleanupMemories = jest.fn().mockResolvedValue({ deletedCount: 3 });
const exportMemories = jest.fn().mockResolvedValue({
  projectId: "project-1",
  exportedAt: "2026-04-13T12:00:00.000Z",
  entries: [{ id: "memory-1", key: "design-note" }],
});
const clearActionFeedback = jest.fn();

const storeState = {
  currentProjectId: "project-1",
  filters: {
    query: "",
    scope: "all",
    category: "all",
    roleId: "",
    startAt: "",
    endAt: "",
    limit: 20,
  },
  entries: [
    {
      id: "memory-1",
      projectId: "project-1",
      scope: "project" as const,
      roleId: "",
      category: "semantic" as const,
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
    key: string;
    content: string;
    metadataObject?: Record<string, unknown> | null;
    relatedContext?: { type: string; id: string; label?: string }[];
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
  fetchMemoryDetail,
  selectMemory,
  toggleMemorySelection,
  clearSelection,
  deleteMemory,
  bulkDeleteMemories,
  cleanupMemories,
  exportMemories,
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
    fetchMemoryDetail.mockClear();
    selectMemory.mockClear();
    toggleMemorySelection.mockClear();
    clearSelection.mockClear();
    deleteMemory.mockClear();
    bulkDeleteMemories.mockClear();
    cleanupMemories.mockClear();
    exportMemories.mockClear();
    clearActionFeedback.mockClear();
    roleStoreState.fetchRoles.mockClear();

    storeState.filters = {
      query: "",
      scope: "all",
      category: "all",
      roleId: "",
      startAt: "",
      endAt: "",
      limit: 20,
    };
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

    await user.click(screen.getByRole("button", { name: "Export JSON" }));
    expect(exportMemories).toHaveBeenCalledWith("project-1");

    await user.click(screen.getByRole("button", { name: "Delete Selected (2)" }));
    await user.click(screen.getByRole("button", { name: "Confirm Bulk Delete" }));

    expect(bulkDeleteMemories).toHaveBeenCalledWith(
      "project-1",
      ["memory-1", "memory-2"],
      undefined,
    );
  });
});
