import { render, screen, waitFor } from "@testing-library/react";
import DocsPageDetail from "./page";

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

const currentPage = {
  id: "page-1",
  spaceId: "space-1",
  parentId: null,
  title: "Runbook",
  content: '[{"type":"paragraph","content":"Latest draft"}]',
  contentText: "Latest draft",
  path: "/runbook",
  sortOrder: 0,
  isTemplate: false,
  templateCategory: "",
  isSystem: false,
  isPinned: false,
  createdBy: "user-1",
  updatedBy: "user-1",
  createdAt: "2026-03-26T12:00:00.000Z",
  updatedAt: "2026-03-26T12:05:00.000Z",
  deletedAt: null,
};

const sharedVersion = {
  id: "version-1",
  pageId: "page-1",
  versionNumber: 2,
  name: "Shared snapshot",
  content: '[{"type":"paragraph","content":"Shared snapshot"}]',
  createdBy: "user-1",
  createdAt: "2026-03-26T12:03:00.000Z",
};

const docsStoreState = {
  projectId: null as string | null,
  tree: [],
  currentPage,
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

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: { selectedProjectId: string | null }) => unknown) =>
    selector({ selectedProjectId: null }),
}));

jest.mock("@/lib/stores/docs-store", () => ({
  useDocsStore: () => docsStoreState,
}));

jest.mock("@/components/docs/docs-sidebar-panel", () => ({
  DocsSidebarPanel: () => <div data-testid="docs-sidebar-panel" />,
}));

jest.mock("@/components/docs/block-editor", () => ({
  BlockEditor: ({
    value,
    editable,
  }: {
    value: string;
    editable?: boolean;
  }) => (
    <div data-testid="block-editor">
      {JSON.stringify({ value, editable })}
    </div>
  ),
}));

jest.mock("@/components/docs/comments-panel", () => ({
  CommentsPanel: () => <div data-testid="comments-panel" />,
}));

jest.mock("@/components/docs/editor-toolbar", () => ({
  EditorToolbar: ({ readonly }: { readonly?: boolean }) => (
    <div data-testid="editor-toolbar">{JSON.stringify({ readonly })}</div>
  ),
}));

jest.mock("@/components/docs/template-picker", () => ({
  TemplatePicker: () => null,
}));

jest.mock("@/components/docs/version-history-panel", () => ({
  VersionHistoryPanel: ({ readonly }: { readonly?: boolean }) => (
    <div data-testid="version-history-panel">{JSON.stringify({ readonly })}</div>
  ),
}));

jest.mock("@/components/docs/version-viewer", () => ({
  VersionViewer: ({ version }: { version: { id: string } | null }) => (
    <div data-testid="version-viewer">{version?.id ?? "none"}</div>
  ),
}));

describe("DocsPageDetail", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    docsStoreState.projectId = null;
    resolvePageContext.mockResolvedValue({
      projectId: "project-2",
      page: currentPage,
    });
    window.history.pushState({}, "", "/docs/page-1?version=version-1&readonly=1");
  });

  it("resolves document context from the page id and renders shared versions as read-only", async () => {
    render(<DocsPageDetail params={Promise.resolve({ pageId: "page-1" })} />);

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
  });
});
