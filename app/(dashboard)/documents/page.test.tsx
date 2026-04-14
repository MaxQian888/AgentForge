import { render, screen } from "@testing-library/react";
import DocumentsPage from "./page";

const dashboardState = {
  selectedProjectId: null as string | null,
};

jest.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => (key: string) =>
    namespace ? `${namespace}.${key}` : key,
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("@/components/shared/page-header", () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

jest.mock("@/components/shared/empty-state", () => ({
  EmptyState: ({ title }: { title: string }) => <div data-testid="empty-state">{title}</div>,
}));

jest.mock("@/components/documents/document-panel", () => ({
  DocumentPanel: ({ projectId }: { projectId: string }) => (
    <div data-testid="document-panel">{projectId}</div>
  ),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) => selector(dashboardState),
}));

describe("DocumentsPage", () => {
  beforeEach(() => {
    dashboardState.selectedProjectId = null;
  });

  it("shows a project selection empty state when no project is active", () => {
    render(<DocumentsPage />);

    expect(screen.getByRole("heading", { name: "documents.title" })).toBeInTheDocument();
    expect(screen.getByTestId("empty-state")).toHaveTextContent("documents.selectProject");
    expect(screen.queryByTestId("document-panel")).not.toBeInTheDocument();
  });

  it("renders the document panel for the selected project", () => {
    dashboardState.selectedProjectId = "project-42";

    render(<DocumentsPage />);

    expect(screen.getByTestId("document-panel")).toHaveTextContent("project-42");
    expect(screen.queryByTestId("empty-state")).not.toBeInTheDocument();
  });
});
