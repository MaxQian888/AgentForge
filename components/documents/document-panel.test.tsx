import { render, screen } from "@testing-library/react";
import { DocumentPanel } from "./document-panel";

const loadDocuments = jest.fn().mockResolvedValue(undefined);

const storeState = {
  documents: [],
  loading: false,
  uploading: false,
  error: null,
  currentProjectId: "project-1",
  loadDocuments,
  uploadDocument: jest.fn().mockResolvedValue(undefined),
  deleteDocument: jest.fn().mockResolvedValue(undefined),
  clearError: jest.fn(),
};

jest.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => (key: string, params?: Record<string, unknown>) => {
    if (params) {
      return Object.entries(params).reduce(
        (acc, [k, v]) => acc.replace(`{${k}}`, String(v)),
        `${namespace}.${key}`,
      );
    }
    return namespace ? `${namespace}.${key}` : key;
  },
}));

jest.mock("@/lib/stores/knowledge-store", () => ({
  useKnowledgeStore: () => storeState,
}));

jest.mock("@/components/shared/error-banner", () => ({
  ErrorBanner: ({ message }: { message: string }) => (
    <div data-testid="error-banner">{message}</div>
  ),
}));

jest.mock("@/components/shared/empty-state", () => ({
  EmptyState: ({ title }: { title: string }) => (
    <div data-testid="empty-state">{title}</div>
  ),
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

describe("DocumentPanel", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    storeState.documents = [];
    storeState.loading = false;
    storeState.error = null;
  });

  it("renders the upload zone", () => {
    render(<DocumentPanel projectId="project-1" />);

    expect(screen.getByText("documents.uploadButton")).toBeInTheDocument();
  });

  it("renders empty state when no documents exist", () => {
    render(<DocumentPanel projectId="project-1" />);

    expect(screen.getByTestId("empty-state")).toHaveTextContent("documents.noDocuments");
  });

  it("calls loadDocuments on mount", () => {
    render(<DocumentPanel projectId="project-1" />);

    expect(loadDocuments).toHaveBeenCalledWith("project-1");
  });
});
