import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DocsPageDetailClient } from "./page-client";
import { useAuthStore } from "@/lib/stores/auth-store";

type DocsPage = import("@/lib/stores/knowledge-store").KnowledgeAsset;

const push = jest.fn();
const setProjectId = jest.fn();
const fetchTree = jest.fn();
const fetchPageWorkspace = jest.fn();
const createPageFromTemplate = jest.fn();
const createTemplateFromPage = jest.fn();
const createVersion = jest.fn();
const restoreVersion = jest.fn();
const createComment = jest.fn();
const setCommentResolved = jest.fn();
const movePage = jest.fn();
const toggleFavorite = jest.fn();
const togglePinned = jest.fn();
const updatePage = jest.fn();
const resolvePageContext = jest.fn();
const fetchLinks = jest.fn();
const createLink = jest.fn();
const deleteLink = jest.fn();
const fetchTasks = jest.fn();
const apiPost = jest.fn();
const writeTextMock = jest.fn().mockResolvedValue(undefined);
const dashboardState = {
  selectedProjectId: null as string | null,
};

const currentPage: DocsPage = {
  id: "page-1",
  projectId: "project-1",
  kind: "wiki_page",
  spaceId: "space-1",
  parentId: null,
  title: "Runbook",
  contentJson: '[{"type":"paragraph","content":"Latest draft"}]',
  contentText: "Latest draft",
  path: "/runbook",
  sortOrder: 0,
  templateCategory: null,
  isPinned: false,
  createdBy: "user-1",
  updatedBy: "user-1",
  createdAt: "2026-03-26T12:00:00.000Z",
  updatedAt: "2026-03-26T12:05:00.000Z",
  deletedAt: null,
  version: 1,
};

const sharedVersion = {
  id: "version-1",
  assetId: "page-1",
  versionNumber: 2,
  name: "Shared snapshot",
  kindSnapshot: "wiki_page",
  contentJson: '[{"type":"paragraph","content":"Shared snapshot"}]',
  createdBy: "user-1",
  createdAt: "2026-03-26T12:03:00.000Z",
};

const docsStoreState = {
  projectId: null as string | null,
  tree: [],
  currentAsset: currentPage as DocsPage | null,
  comments: [],
  versions: [sharedVersion],
  templates: [],
  favorites: [],
  recentAccess: [],
  loading: false,
  saving: false,
  error: null,
  setProjectId,
  fetchTree,
  fetchPageWorkspace,
  resolvePageContext,
  createPageFromTemplate,
  createTemplateFromPage,
  createVersion,
  restoreVersion,
  createComment,
  setCommentResolved,
  movePage,
  toggleFavorite,
  togglePinned,
  updatePage,
};

const entityLinks = [
  {
    id: "link-1",
    sourceType: "wiki_page",
    sourceId: "page-1",
    targetType: "task",
    targetId: "task-1",
    linkType: "design",
    anchorBlockId: "block-1",
  },
  {
    id: "link-2",
    sourceType: "task",
    sourceId: "task-1",
    targetType: "wiki_page",
    targetId: "page-1",
    linkType: "mention",
    anchorBlockId: null,
  },
];

jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(() => ({
    post: apiPost,
  })),
}));

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) =>
    selector(dashboardState),
}));

jest.mock("@/lib/stores/knowledge-store", () => ({
  flattenKnowledgeTree: (tree: Array<Record<string, unknown>>) => tree,
  useKnowledgeStore: () => docsStoreState,
}));

jest.mock("@/components/docs/docs-sidebar-panel", () => ({
  DocsSidebarPanel: ({
    onMovePage,
    onToggleFavorite,
    onTogglePinned,
  }: {
    onMovePage: (pageId: string, parentId: string | null, sortOrder: number) => void;
    onToggleFavorite: (pageId: string, favorite: boolean) => void;
    onTogglePinned: (pageId: string, pinned: boolean) => void;
  }) => (
    <div data-testid="docs-sidebar-panel">
      <button type="button" onClick={() => onMovePage("page-1", "parent-1", 2)}>
        move-page
      </button>
      <button type="button" onClick={() => onToggleFavorite("page-1", true)}>
        toggle-favorite
      </button>
      <button type="button" onClick={() => onTogglePinned("page-1", true)}>
        toggle-pinned
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/block-editor", () => ({
  BlockEditor: ({
    value,
    editable,
    onCreateTasksFromSelection,
    onChange,
  }: {
    value: string;
    editable?: boolean;
    onCreateTasksFromSelection?: (blockIds: string[]) => void;
    onChange?: (value: string, contentText: string) => void;
  }) => (
    <div data-testid="block-editor">
      {JSON.stringify({ value, editable })}
      <button type="button" onClick={() => onCreateTasksFromSelection?.(["block-1"])}>
        create-selection-tasks
      </button>
      <button
        type="button"
        onClick={() => onChange?.('[{"id":"block-1","content":"Updated"}]', "Updated")}
      >
        update-doc-content
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/comments-panel", () => ({
  CommentsPanel: ({
    onCreateComment,
    onResolve,
    onReopen,
    onCopyLink,
  }: {
    onCreateComment: (body: string) => Promise<void>;
    onResolve: (commentId: string) => Promise<void>;
    onReopen: (commentId: string) => Promise<void>;
    onCopyLink: (commentId: string) => Promise<void>;
  }) => (
    <div data-testid="comments-panel">
      <button type="button" onClick={() => void onCreateComment("Investigate")}>
        create-comment
      </button>
      <button type="button" onClick={() => void onResolve("comment-1")}>
        resolve-comment
      </button>
      <button type="button" onClick={() => void onReopen("comment-1")}>
        reopen-comment
      </button>
      <button type="button" onClick={() => void onCopyLink("comment-1")}>
        copy-comment-link
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/related-tasks-panel", () => ({
  RelatedTasksPanel: ({
    onAddTask,
    onRemoveTask,
  }: {
    onAddTask: () => void;
    onRemoveTask: (linkId: string) => void;
  }) => (
    <div data-testid="related-tasks-panel">
      <button type="button" onClick={onAddTask}>
        add-task-link
      </button>
      <button type="button" onClick={() => onRemoveTask("link-1")}>
        remove-task-link
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/task-link-picker", () => ({
  TaskLinkPicker: ({
    onPick,
  }: {
    onPick: (taskId: string) => void;
  }) => (
    <div data-testid="task-link-picker">
      <button type="button" onClick={() => onPick("task-1")}>
        pick-task
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/decompose-tasks-dialog", () => ({
  DecomposeTasksDialog: ({
    onConfirm,
  }: {
    onConfirm: (payload: { blockIds: string[]; parentTaskId: string | null }) => void;
  }) => (
    <div data-testid="decompose-tasks-dialog">
      <button
        type="button"
        onClick={() => onConfirm({ blockIds: ["block-1"], parentTaskId: "task-1" })}
      >
        confirm-decompose
      </button>
    </div>
  ),
}));

jest.mock("@/components/shared/backlinks-panel", () => ({
  BacklinksPanel: () => <div data-testid="backlinks-panel" />,
}));

jest.mock("@/components/docs/editor-toolbar", () => ({
  EditorToolbar: ({
    readonly,
    onSaveVersion,
    onSaveTemplate,
    onShareVersion,
  }: {
    readonly?: boolean;
    onSaveVersion?: () => void;
    onSaveTemplate?: () => void;
    onShareVersion?: () => void;
  }) => (
    <div data-testid="editor-toolbar">
      {JSON.stringify({ readonly })}
      <button type="button" onClick={() => onSaveVersion?.()}>
        save-version
      </button>
      <button type="button" onClick={() => onSaveTemplate?.()}>
        save-template
      </button>
      <button type="button" onClick={() => onShareVersion?.()}>
        share-version
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/template-picker", () => ({
  TemplatePicker: ({
    onPick,
  }: {
    onPick: (selection: { templateId: string; title: string; parentId?: string | null }) => void;
  }) => (
    <button
      type="button"
      onClick={() => onPick({ templateId: "template-1", title: "Template Draft", parentId: "page-2" })}
    >
      pick-template
    </button>
  ),
}));

jest.mock("@/components/docs/version-history-panel", () => ({
  VersionHistoryPanel: ({
    readonly,
    onSelect,
    onRestore,
    onShare,
  }: {
    readonly?: boolean;
    onSelect: (versionId: string) => void;
    onRestore: (versionId: string) => void;
    onShare: (versionId: string) => void;
  }) => (
    <div data-testid="version-history-panel">
      {JSON.stringify({ readonly })}
      <button type="button" onClick={() => onSelect("version-1")}>
        select-version
      </button>
      <button type="button" onClick={() => onRestore("version-1")}>
        restore-version
      </button>
      <button type="button" onClick={() => onShare("version-1")}>
        share-version-link
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/version-viewer", () => ({
  VersionViewer: ({ version }: { version: { id: string } | null }) => (
    <div data-testid="version-viewer">{version?.id ?? "none"}</div>
  ),
}));

jest.mock("@/lib/stores/entity-link-store", () => ({
  useEntityLinkStore: (
    selector: (state: {
      linksByEntity: Record<string, unknown>;
      fetchLinks: jest.Mock;
      createLink: jest.Mock;
      deleteLink: jest.Mock;
    }) => unknown,
  ) =>
    selector({
      linksByEntity: { "wiki_page:page-1": entityLinks },
      fetchLinks,
      createLink,
      deleteLink,
    }),
}));

jest.mock("@/lib/stores/task-store", () => ({
  useTaskStore: (
    selector?: (state: { tasks: Array<Record<string, unknown>>; fetchTasks: jest.Mock }) => unknown,
  ) => {
    const state = {
      tasks: [
        {
          id: "task-1",
          title: "Task One",
          status: "todo",
          assigneeName: "Alice",
          plannedEndAt: "2026-03-30T00:00:00.000Z",
        },
      ],
      fetchTasks,
    };
    return typeof selector === "function" ? selector(state) : state;
  },
}));

describe("DocsPageDetailClient", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    docsStoreState.currentAsset = currentPage;
    docsStoreState.projectId = null;
    dashboardState.selectedProjectId = null;
    setProjectId.mockImplementation((nextProjectId: string) => {
      docsStoreState.projectId = nextProjectId;
    });
    resolvePageContext.mockResolvedValue("project-2");
    window.history.pushState({}, "", "/docs/page-1?version=version-1&readonly=1");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: writeTextMock,
      },
    });
    apiPost.mockReset().mockResolvedValue(undefined);
    writeTextMock.mockReset().mockResolvedValue(undefined);
    fetchLinks.mockReset();
    createLink.mockReset();
    deleteLink.mockReset();
    fetchTasks.mockReset();
    useAuthStore.setState({
      accessToken: "access-1",
    } as never);
  });

  it("resolves document context from the page id and renders shared versions as read-only", async () => {
    await act(async () => {
      render(<DocsPageDetailClient pageId="page-1" />);
    });

    await waitFor(() => {
      expect(resolvePageContext).toHaveBeenCalledWith("page-1");
    });
    await waitFor(() => {
      expect(setProjectId).toHaveBeenCalledWith("project-2");
    });

    expect(fetchTree).toHaveBeenCalledWith("project-2");
    expect(fetchPageWorkspace).toHaveBeenCalledWith("project-2", "page-1");
    expect(
      screen.queryByText("Select a project from the dashboard before opening a document."),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("block-editor")).toHaveTextContent("Shared snapshot");
    expect(screen.getByTestId("block-editor")).toHaveTextContent('"editable":false');
    expect(screen.getByTestId("editor-toolbar")).toHaveTextContent('"readonly":true');
    expect(screen.getByTestId("version-history-panel")).toHaveTextContent('"readonly":true');
    expect(screen.getByTestId("version-viewer")).toHaveTextContent("version-1");
    expect(screen.getByTestId("related-tasks-panel")).toBeInTheDocument();
    expect(screen.getByTestId("backlinks-panel")).toBeInTheDocument();
    expect(screen.getByTestId("task-link-picker")).toBeInTheDocument();
    expect(screen.getByTestId("decompose-tasks-dialog")).toBeInTheDocument();
  });

  it("renders the not-found state when the page workspace never loads a document", async () => {
    docsStoreState.currentAsset = null;
    resolvePageContext.mockResolvedValue(null);

    await act(async () => {
      render(<DocsPageDetailClient pageId="missing-page" />);
    });

    await waitFor(() => {
      expect(resolvePageContext).toHaveBeenCalledWith("missing-page");
    });
    expect(screen.getByRole("heading")).toBeInTheDocument();
    expect(screen.getByRole("button")).toBeInTheDocument();
    expect(fetchTree).not.toHaveBeenCalled();
    expect(fetchPageWorkspace).not.toHaveBeenCalled();
  });

  it("wires page editor actions back to docs store handlers", async () => {
    const user = userEvent.setup();
    dashboardState.selectedProjectId = "project-2";
    window.history.pushState({}, "", "/docs/page-1");

    await act(async () => {
      render(<DocsPageDetailClient pageId="page-1" />);
    });

    await waitFor(() => {
      expect(setProjectId).toHaveBeenCalledWith("project-2");
    });

    await user.click(screen.getByRole("button", { name: "move-page" }));
    await user.click(screen.getByRole("button", { name: "toggle-favorite" }));
    await user.click(screen.getByRole("button", { name: "toggle-pinned" }));
    await user.click(screen.getByRole("button", { name: "save-version" }));
    await user.click(screen.getByRole("button", { name: "save-template" }));
    await user.click(screen.getByRole("button", { name: "share-version" }));
    await user.click(screen.getByRole("button", { name: "update-doc-content" }));
    await user.click(screen.getByRole("button", { name: "add-task-link" }));
    await user.click(screen.getByRole("button", { name: "pick-task" }));
    await user.click(screen.getByRole("button", { name: "remove-task-link" }));
    await user.click(screen.getByRole("button", { name: "create-comment" }));
    await user.click(screen.getByRole("button", { name: "resolve-comment" }));
    await user.click(screen.getByRole("button", { name: "reopen-comment" }));
    await user.click(screen.getByRole("button", { name: "copy-comment-link" }));
    await user.click(screen.getByRole("button", { name: "create-selection-tasks" }));
    await user.click(screen.getByRole("button", { name: "confirm-decompose" }));
    await user.click(screen.getByRole("button", { name: "pick-template" }));
    await user.click(screen.getByRole("button", { name: "restore-version" }));
    await user.click(screen.getByRole("button", { name: "share-version-link" }));

    expect(movePage).toHaveBeenCalledWith({
      projectId: "project-2",
      pageId: "page-1",
      parentId: "parent-1",
      sortOrder: 2,
    });
    expect(toggleFavorite).toHaveBeenCalledWith({
      projectId: "project-2",
      pageId: "page-1",
      favorite: true,
    });
    expect(togglePinned).toHaveBeenCalledWith({
      projectId: "project-2",
      pageId: "page-1",
      pinned: true,
    });
    expect(createVersion).toHaveBeenCalledWith({
      projectId: "project-2",
      assetId: "page-1",
      name: expect.stringContaining("Snapshot"),
    });
    expect(createTemplateFromPage).toHaveBeenCalledWith({
      projectId: "project-2",
      pageId: "page-1",
      name: "Runbook Template",
      category: "custom",
    });
    expect(updatePage).toHaveBeenCalledWith({
      projectId: "project-2",
      pageId: "page-1",
      title: "Runbook",
      content: '[{"id":"block-1","content":"Updated"}]',
      contentText: "Updated",
      expectedUpdatedAt: "2026-03-26T12:05:00.000Z",
      templateCategory: undefined,
    });
    expect(createLink).toHaveBeenCalledWith({
      projectId: "project-2",
      sourceType: "wiki_page",
      sourceId: "page-1",
      targetType: "task",
      targetId: "task-1",
      linkType: "design",
    });
    expect(deleteLink).toHaveBeenCalledWith("project-2", "wiki_page", "page-1", "link-1");
    expect(createComment).toHaveBeenCalledWith({
      projectId: "project-2",
      assetId: "page-1",
      body: "Investigate",
      mentions: "[]",
    });
    expect(setCommentResolved).toHaveBeenCalledWith({
      projectId: "project-2",
      assetId: "page-1",
      commentId: "comment-1",
      resolved: false,
    });
    expect(createPageFromTemplate).toHaveBeenCalledWith({
      projectId: "project-2",
      templateId: "template-1",
      title: "Template Draft",
      parentId: "page-2",
    });
    expect(restoreVersion).toHaveBeenCalledWith({
      projectId: "project-2",
      assetId: "page-1",
      versionId: "version-1",
    });
    expect(apiPost).toHaveBeenCalledWith(
      "/api/v1/projects/project-2/knowledge/assets/page-1/decompose-tasks",
      {
        blockIds: ["block-1"],
        parentTaskId: "task-1",
      },
      { token: "access-1" },
    );
  });
});
