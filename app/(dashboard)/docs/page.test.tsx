import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import DocsLandingPage from "./page";

const searchParamsState = {
  pageId: null as string | null,
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
  fetchTree: jest.fn(),
  fetchTemplates: jest.fn(),
  fetchFavorites: jest.fn(),
  fetchRecentAccess: jest.fn(),
  createPage: jest.fn().mockResolvedValue(undefined),
  createPageFromTemplate: jest.fn().mockResolvedValue(undefined),
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
  useSearchParams: () => ({
    get: (key: string) => (key === "pageId" ? searchParamsState.pageId : null),
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
  }: {
    onCreateFromTemplate: (templateId: string) => void;
  }) => (
    <div data-testid="template-center">
      <button type="button" onClick={() => onCreateFromTemplate("template-1")}>
        create-from-center
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
    onPick: (templateId: string) => void;
  }) =>
    open ? (
      <div data-testid="template-picker">
        <button type="button" onClick={() => onPick("template-1")}>
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

jest.mock("@/lib/stores/docs-store", () => ({
  flattenDocsTree: (tree: Array<{ id: string; title: string; path: string; isPinned: boolean }>) => tree,
  useDocsStore: Object.assign(() => docsStoreState, {
    getState: () => docsStoreState,
  }),
}));

describe("DocsLandingPage", () => {
  beforeEach(() => {
    searchParamsState.pageId = null;
    dashboardState.selectedProjectId = null;
    docsStoreState.fetchTree.mockReset();
    docsStoreState.fetchTemplates.mockReset();
    docsStoreState.fetchFavorites.mockReset();
    docsStoreState.fetchRecentAccess.mockReset();
    docsStoreState.createPage.mockReset().mockResolvedValue(undefined);
    docsStoreState.setProjectId.mockReset();
  });

  it("shows the empty state until a project is selected", () => {
    render(<DocsLandingPage />);

    expect(screen.getByRole("heading", { name: "docs.title" })).toBeInTheDocument();
    expect(screen.getByTestId("empty-state")).toHaveTextContent("docs.selectProject");
    expect(docsStoreState.fetchTree).not.toHaveBeenCalled();
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
    expect(docsStoreState.createPageFromTemplate).toHaveBeenNthCalledWith(1, {
      projectId: "project-5",
      templateId: "template-1",
      title: "docs.newFromTemplate",
    });
    expect(docsStoreState.createPageFromTemplate).toHaveBeenNthCalledWith(2, {
      projectId: "project-5",
      templateId: "template-1",
      title: "docs.newFromTemplate",
    });
    expect(docsStoreState.createPage).toHaveBeenCalledWith({
      projectId: "project-5",
      title: "docs.untitledDoc",
    });
  });
});
