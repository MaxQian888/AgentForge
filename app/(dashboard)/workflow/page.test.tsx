import { render, screen } from "@testing-library/react";
import WorkflowPage from "./page";

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

jest.mock("@/components/workflow/workflow-config-panel", () => ({
  WorkflowConfigPanel: ({ projectId }: { projectId: string }) => (
    <div data-testid="workflow-config-panel">{projectId}</div>
  ),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) => selector(dashboardState),
}));

describe("WorkflowPage", () => {
  beforeEach(() => {
    dashboardState.selectedProjectId = null;
  });

  it("asks the user to select a project before loading workflow settings", () => {
    render(<WorkflowPage />);

    expect(screen.getByRole("heading", { name: "workflow.title" })).toBeInTheDocument();
    expect(screen.getByTestId("empty-state")).toHaveTextContent("workflow.selectProject");
  });

  it("renders the workflow configuration panel for the active project", () => {
    dashboardState.selectedProjectId = "project-99";

    render(<WorkflowPage />);

    expect(screen.getByTestId("workflow-config-panel")).toHaveTextContent("project-99");
    expect(screen.queryByTestId("empty-state")).not.toBeInTheDocument();
  });
});
