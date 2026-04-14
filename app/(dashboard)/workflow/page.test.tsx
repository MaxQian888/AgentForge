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

jest.mock("@/components/workflow/workflow-execution-view", () => ({
  WorkflowExecutionView: () => <div data-testid="workflow-execution-view" />,
}));

jest.mock("@/components/workflow/workflow-reviews-tab", () => ({
  WorkflowReviewsTab: ({ projectId }: { projectId: string }) => (
    <div data-testid="workflow-reviews-tab">{projectId}</div>
  ),
}));

jest.mock("@/components/workflow/workflow-templates-tab", () => ({
  WorkflowTemplatesTab: ({ projectId }: { projectId: string; setActiveTab: (tab: string) => void }) => (
    <div data-testid="workflow-templates-tab">{projectId}</div>
  ),
}));

jest.mock("@/components/workflow-editor", () => ({
  WorkflowEditor: ({ definition }: { definition: { name: string } }) => (
    <div data-testid="workflow-editor">{definition.name}</div>
  ),
}));

jest.mock("@/lib/stores/workflow-store", () => ({
  useWorkflowStore: () => ({
    definitions: [],
    definitionsLoading: false,
    fetchDefinitions: jest.fn(),
    deleteDefinition: jest.fn(),
    selectDefinition: jest.fn(),
    selectedDefinition: null,
    startExecution: jest.fn(),
    updateDefinition: jest.fn(),
    executions: [],
    executionsLoading: false,
    fetchExecutions: jest.fn(),
    cancelExecution: jest.fn(),
    saving: false,
    createDefinition: jest.fn(),
  }),
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

  it("renders the workflow tabs for the active project", () => {
    dashboardState.selectedProjectId = "project-99";

    render(<WorkflowPage />);

    // The default tab is "workflows", WorkflowListTab renders with empty list
    expect(screen.getByRole("heading", { name: "workflow.title" })).toBeInTheDocument();
    // Config tab panel is not visible (not the default tab), no "select project" empty state
    expect(screen.queryByTestId("workflow-config-panel")).not.toBeInTheDocument();
  });
});
