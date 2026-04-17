import { act, render, screen } from "@testing-library/react";
import MemoryPage from "./page";
import {
  clearFeatureFlagOverrides,
  setFeatureFlagOverride,
} from "@/lib/feature-flags";

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

jest.mock("@/components/memory/memory-panel", () => ({
  MemoryPanel: ({ projectId }: { projectId: string }) => (
    <div data-testid="memory-panel">{projectId}</div>
  ),
}));

jest.mock("@/lib/stores/dashboard-store", () => ({
  useDashboardStore: (selector: (state: typeof dashboardState) => unknown) => selector(dashboardState),
}));

describe("MemoryPage", () => {
  beforeEach(() => {
    dashboardState.selectedProjectId = null;
    act(() => {
      clearFeatureFlagOverrides();
    });
  });

  afterEach(() => {
    act(() => {
      clearFeatureFlagOverrides();
    });
  });

  it("shows a project selection empty state when no project is active", () => {
    render(<MemoryPage />);

    expect(screen.getByRole("heading", { name: "memory.title" })).toBeInTheDocument();
    expect(screen.getByTestId("empty-state")).toHaveTextContent("memory.selectProject");
    expect(screen.queryByTestId("memory-panel")).not.toBeInTheDocument();
  });

  it("renders the memory panel for the selected project", () => {
    dashboardState.selectedProjectId = "project-42";

    render(<MemoryPage />);

    expect(screen.getByTestId("memory-panel")).toHaveTextContent("project-42");
    expect(screen.queryByTestId("empty-state")).not.toBeInTheDocument();
  });

  it("renders a disabled notice when the MEMORY_EXPLORER feature flag is off", () => {
    dashboardState.selectedProjectId = "project-42";
    act(() => {
      setFeatureFlagOverride("MEMORY_EXPLORER", false);
    });

    render(<MemoryPage />);

    expect(screen.queryByTestId("memory-panel")).not.toBeInTheDocument();
    expect(screen.getByTestId("empty-state")).toBeInTheDocument();
  });
});
