import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import DocsLandingPage from "./page";

const push = jest.fn();
const searchParamsState = {
  pageId: null as string | null,
  project: null as string | null,
  action: null as string | null,
};

const docsStoreState = {
  tree: [
    {
      id: "page-1",
      title: "Runbook",
      path: "/runbook",
      isPinned: true,
    },
    {
      id: "page-2",
      title: "Retrospective",
      path: "/retro",
      isPinned: false,
    },
  ],
  templates: [{ id: "template-1", title: "Postmortem" }],
  favorites: [{ pageId: "page-1" }],
  recentAccess: [{ pageId: "page-2", accessedAt: "2026-03-30T00:00:00.000Z" }],
  ingestedFiles: [],
  uploading: false,
  saving: false,
  loading: false,
  searchResults: null,
  uploadFile: jest.fn(),
  reuploadFile: jest.fn(),
  searchKnowledge: jest.fn(),
  clearSearch: jest.fn(),
  fetchIngestedFiles: jest.fn(),
  fetchTree: jest.fn(),
  fetchTemplates: jest.fn(),
  fetchFavorites: jest.fn(),
  fetchRecentAccess: jest.fn(),
  createPage: jest.fn().mockResolvedValue(undefined),
  createTemplate: jest.fn().mockResolvedValue(undefined),
  createPageFromTemplate: jest.fn().mockResolvedValue(undefined),
  duplicateTemplate: jest.fn().mockResolvedValue(undefined),
  deleteTemplate: jest.fn().mockResolvedValue(undefined),
  movePage: jest.fn().mockResolvedValue(undefined),
  toggleFavorite: jest.fn().mockResolvedValue(undefined),
  togglePinned: jest.fn().mockResolvedValue(undefined),
  setProjectId: jest.fn(),
};

const dashboardState = {
  selectedProjectId: null as string | null,
};

jest.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => (key: string) =>
    namespace ? `${namespace}.${key}` : key,
}));

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
  useSearchParams: () => ({
    get: (key: string) =>
      key === "pageId"
        ? searchParamsState.pageId
        : key === "project"
          ? searchParamsState.project
          : key === "action"
            ? searchParamsState.action
            : null,
  }),
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("@/components/shared/page-header", () => ({
  PageHeader: ({
    title,
    description,
    actions,
  }: {
    title: string;
    description?: string;
    actions?: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      {description ? <p>{description}</p> : null}
      {actions}
    </div>
  ),
}));

jest.mock("@/components/shared/empty-state", () => ({
  EmptyState: ({ title }: { title: string }) => <div data-testid="empty-state">{title}</div>,
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
      <button type="button" onClick={() => onMovePage("page-1", "folder-1", 3)}>
        move-doc
      </button>
      <button type="button" onClick={() => onToggleFavorite("page-1", true)}>
        favorite-doc
      </button>
      <button type="button" onClick={() => onTogglePinned("page-1", true)}>
        pin-doc
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/template-center", () => ({
  TemplateCenter: ({
    onCreateFromTemplate,
    onCreateTemplate,
    onEditTemplate,
    onDuplicateTemplate,
    onDeleteTemplate,
  }: {
    onCreateFromTemplate: (templateId: string) => void;
    onCreateTemplate: (input: { title: string; category: string }) => void;
    onEditTemplate: (templateId: string) => void;
    onDuplicateTemplate: (input: { templateId: string; name: string; category: string }) => void;
    onDeleteTemplate: (templateId: string) => void;
  }) => (
    <div data-testid="template-center">
      <button type="button" onClick={() => onCreateFromTemplate("template-1")}>
        create-from-center
      </button>
      <button type="button" onClick={() => onCreateTemplate({ title: "Blank Template", category: "custom" })}>
        create-template
      </button>
      <button type="button" onClick={() => onEditTemplate("template-1")}>
        edit-template
      </button>
      <button
        type="button"
        onClick={() => onDuplicateTemplate({ templateId: "template-1", name: "Template Copy", category: "custom" })}
      >
        duplicate-template
      </button>
      <button type="button" onClick={() => onDeleteTemplate("template-1")}>
        delete-template
      </button>
    </div>
  ),
}));

jest.mock("@/components/docs/template-picker", () => ({
  TemplatePicker: ({
    open,
    onPick,
  }: {
    open: boolean;
    onPick: (selection: { templateId: string; title: string; parentId?: string | null }) => void;
  }) =>
    open ? (
      <div data-testid="template-picker">
        <button type="button" onClick={() => onPick({ templateId: "template-1", title: "Template Draft", parentId: "page-1" })}>
          pick-template
        </button>
      </div>
    ) : null,
}));

jest.mock("./[pageId]/page-client", () => ({
  DocsPageDetailClient: ({ pageId }: { pageId: string }) => (
    <div data-testid="docs-page-detail-client">{pageId}</div>
  ),
}));

jest.mock("@/lib/route-hrefs", () => ({
  buildDocsHref: (pageId: string) => `/docs?pageId=${pageId}`,
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) => selector(dashboardState),
}));

jest.mock("@/lib/stores/knowledge-store", () => ({
  flattenKnowledgeTree: (tree: Array<{ id: string; title: string; path: string; isPinned: boolean }>) => tree,
  useKnowledgeStore: Object.assign(() => docsStoreState, {
    getState: () => docsStoreState,
  }),
}));

describe("DocsLandingPage", () => {
  beforeEach(() => {
    searchParamsState.pageId = null;
    searchParamsState.project = null;
    searchParamsState.action = null;
    dashboardState.selectedProjectId = null;
    docsStoreState.fetchTree.mockReset();
    docsStoreState.fetchTemplates.mockReset();
    docsStoreState.fetchFavorites.mockReset();
    docsStoreState.fetchRecentAccess.mockReset();
    docsStoreState.createPage.mockReset().mockResolvedValue(undefined);
    docsStoreState.createTemplate.mockReset().mockResolvedValue(undefined);
    docsStoreState.duplicateTemplate.mockReset().mockResolvedValue(undefined);
    docsStoreState.deleteTemplate.mockReset().mockResolvedValue(undefined);
    docsStoreState.setProjectId.mockReset();
    push.mockReset();
  });

  it("shows the empty state until a project is selected", () => {
    render(<DocsLandingPage />);

    expect(screen.getByRole("heading", { name: "docs.title" })).toBeInTheDocument();
    expect(screen.getByTestId("empty-state")).toHaveTextContent("docs.selectProject");
    expect(docsStoreState.fetchTree).not.toHaveBeenCalled();
  });

  it("uses explicit project scope and opens the template picker for bootstrap handoffs", async () => {
    const user = userEvent.setup();
    searchParamsState.project = "project-5";
    searchParamsState.action = "use-template";

    render(<DocsLandingPage />);

    await waitFor(() => {
      expect(docsStoreState.setProjectId).toHaveBeenCalledWith("project-5");
    });
    expect(screen.getByTestId("template-picker")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "pick-template" }));
    expect(docsStoreState.createPageFromTemplate).toHaveBeenCalledWith(
      expect.objectContaining({
        projectId: "project-5",
        templateId: "template-1",
      }),
    );
  });

  it("delegates to the page detail client when a page id query param is present", () => {
    searchParamsState.pageId = "page-77";

    render(<DocsLandingPage />);

    expect(screen.getByTestId("docs-page-detail-client")).toHaveTextContent("page-77");
  });

  it("loads the docs workspace and creates new pages within the selected project", async () => {
    const user = userEvent.setup();
    dashboardState.selectedProjectId = "project-5";

    render(<DocsLandingPage />);

    await waitFor(() => {
      expect(docsStoreState.setProjectId).toHaveBeenCalledWith("project-5");
    });
    expect(docsStoreState.fetchTree).toHaveBeenCalledWith("project-5");
    expect(docsStoreState.fetchTemplates).toHaveBeenCalledWith("project-5");
    expect(docsStoreState.fetchFavorites).toHaveBeenCalledWith("project-5");
    expect(docsStoreState.fetchRecentAccess).toHaveBeenCalledWith("project-5");

    expect(screen.getByTestId("docs-sidebar-panel")).toBeInTheDocument();
    expect(screen.getByTestId("template-center")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Runbook" })[0]).toHaveAttribute(
      "href",
      "/docs?pageId=page-1",
    );

    await user.click(screen.getByRole("button", { name: "move-doc" }));
    await user.click(screen.getByRole("button", { name: "favorite-doc" }));
    await user.click(screen.getByRole("button", { name: "pin-doc" }));
    await user.click(screen.getByRole("button", { name: "create-from-center" }));
    await user.click(screen.getByRole("button", { name: "create-template" }));
    await user.click(screen.getByRole("button", { name: "edit-template" }));
    await user.click(screen.getByRole("button", { name: "duplicate-template" }));
    await user.click(screen.getByRole("button", { name: "delete-template" }));
    await user.click(screen.getByRole("button", { name: "docs.useTemplate" }));
    await user.click(screen.getByRole("button", { name: "pick-template" }));
    await user.click(screen.getByRole("button", { name: "docs.newPage" }));

    expect(docsStoreState.movePage).toHaveBeenCalledWith({
      projectId: "project-5",
      pageId: "page-1",
      parentId: "folder-1",
      sortOrder: 3,
    });
    expect(docsStoreState.toggleFavorite).toHaveBeenCalledWith({
      projectId: "project-5",
      pageId: "page-1",
      favorite: true,
    });
    expect(docsStoreState.togglePinned).toHaveBeenCalledWith({
      projectId: "project-5",
      pageId: "page-1",
      pinned: true,
    });
    expect(docsStoreState.createPageFromTemplate).toHaveBeenCalledWith({
      projectId: "project-5",
      templateId: "template-1",
      title: "Template Draft",
      parentId: "page-1",
    });
    expect(docsStoreState.createPage).toHaveBeenCalledWith({
      projectId: "project-5",
      title: "docs.untitledDoc",
    });
    expect(docsStoreState.createTemplate).toHaveBeenCalledWith({
      projectId: "project-5",
      title: "Blank Template",
      category: "custom",
    });
    expect(docsStoreState.duplicateTemplate).toHaveBeenCalledWith({
      projectId: "project-5",
      templateId: "template-1",
      name: "Template Copy",
      category: "custom",
    });
    expect(docsStoreState.deleteTemplate).toHaveBeenCalledWith({
      projectId: "project-5",
      templateId: "template-1",
    });
    expect(push).toHaveBeenNthCalledWith(1, "/docs?pageId=template-1");
  });
});
