import { render, screen, waitFor, fireEvent } from "@testing-library/react";

jest.mock("next/navigation", () => ({
  useParams: () => ({ id: "p1" }),
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({ useBreadcrumbs: jest.fn() }));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn(), message: jest.fn() },
}));

jest.mock("@/lib/stores/vcs-integrations-store", () => {
  const actual = jest.requireActual("@/lib/stores/vcs-integrations-store");
  return {
    ...actual,
    useVCSIntegrationsStore: Object.assign(jest.fn(), { setState: jest.fn() }),
  };
});

jest.mock("@/lib/stores/secrets-store", () => {
  const actual = jest.requireActual("@/lib/stores/secrets-store");
  return {
    ...actual,
    useSecretsStore: Object.assign(jest.fn(), { setState: jest.fn() }),
  };
});

import { useSecretsStore } from "@/lib/stores/secrets-store";
import { useVCSIntegrationsStore } from "@/lib/stores/vcs-integrations-store";
import VCSIntegrationsPage from "./page";

describe("VCSIntegrationsPage", () => {
  beforeEach(() => {
    (window as unknown as { confirm: () => boolean }).confirm = () => true;
    (useVCSIntegrationsStore as unknown as jest.Mock).mockReturnValue({
      integrationsByProject: {
        p1: [
          {
            id: "i1",
            projectId: "p1",
            provider: "github",
            host: "github.com",
            owner: "octocat",
            repo: "hello",
            defaultBranch: "main",
            webhookId: "hook-99",
            tokenSecretRef: "vcs.github.demo.pat",
            webhookSecretRef: "vcs.github.demo.webhook",
            status: "active",
            createdAt: "2026-04-20T00:00:00Z",
            updatedAt: "2026-04-20T00:00:00Z",
          },
        ],
      },
      loadingByProject: { p1: false },
      fetchIntegrations: jest.fn(),
      createIntegration: jest.fn(),
      patchIntegration: jest.fn(),
      deleteIntegration: jest.fn(),
      syncIntegration: jest.fn(),
    });
    (useSecretsStore as unknown as jest.Mock).mockReturnValue({
      secretsByProject: {
        p1: [
          {
            name: "vcs.github.demo.pat",
            createdBy: "u",
            createdAt: "",
            updatedAt: "",
          },
          {
            name: "vcs.github.demo.webhook",
            createdBy: "u",
            createdAt: "",
            updatedAt: "",
          },
        ],
      },
      loadingByProject: {},
      lastRevealedValue: null,
      fetchSecrets: jest.fn(),
    });
  });

  it("renders existing integration with status + webhook id", async () => {
    render(<VCSIntegrationsPage />);
    await waitFor(() =>
      expect(screen.getByText("octocat/hello")).toBeInTheDocument(),
    );
    expect(screen.getByText(/active/i)).toBeInTheDocument();
    expect(screen.getByText("hook-99")).toBeInTheDocument();
  });

  it("triggers fetchIntegrations + fetchSecrets on mount", async () => {
    const fetchInteg = jest.fn();
    const fetchSec = jest.fn();
    (useVCSIntegrationsStore as unknown as jest.Mock).mockReturnValue({
      integrationsByProject: { p1: [] },
      loadingByProject: { p1: false },
      fetchIntegrations: fetchInteg,
      createIntegration: jest.fn(),
      patchIntegration: jest.fn(),
      deleteIntegration: jest.fn(),
      syncIntegration: jest.fn(),
    });
    (useSecretsStore as unknown as jest.Mock).mockReturnValue({
      secretsByProject: { p1: [] },
      loadingByProject: {},
      lastRevealedValue: null,
      fetchSecrets: fetchSec,
    });
    render(<VCSIntegrationsPage />);
    await waitFor(() => expect(fetchInteg).toHaveBeenCalledWith("p1"));
    expect(fetchSec).toHaveBeenCalledWith("p1");
  });

  it("calls deleteIntegration when the delete button is clicked", async () => {
    const del = jest.fn();
    (useVCSIntegrationsStore as unknown as jest.Mock).mockReturnValue({
      integrationsByProject: {
        p1: [
          {
            id: "i1",
            projectId: "p1",
            provider: "github",
            host: "github.com",
            owner: "octocat",
            repo: "hello",
            defaultBranch: "main",
            webhookId: "hook-99",
            tokenSecretRef: "t",
            webhookSecretRef: "w",
            status: "active",
            createdAt: "",
            updatedAt: "",
          },
        ],
      },
      loadingByProject: { p1: false },
      fetchIntegrations: jest.fn(),
      createIntegration: jest.fn(),
      patchIntegration: jest.fn(),
      deleteIntegration: del,
      syncIntegration: jest.fn(),
    });
    render(<VCSIntegrationsPage />);
    await waitFor(() => screen.getByText("octocat/hello"));
    fireEvent.click(screen.getByRole("button", { name: /删除/ }));
    await waitFor(() => expect(del).toHaveBeenCalledWith("p1", "i1"));
  });

  it("calls syncIntegration when the re-sync button is clicked", async () => {
    const sync = jest.fn();
    (useVCSIntegrationsStore as unknown as jest.Mock).mockReturnValue({
      integrationsByProject: {
        p1: [
          {
            id: "i1",
            projectId: "p1",
            provider: "github",
            host: "github.com",
            owner: "octocat",
            repo: "hello",
            defaultBranch: "main",
            webhookId: "hook-99",
            tokenSecretRef: "t",
            webhookSecretRef: "w",
            status: "active",
            createdAt: "",
            updatedAt: "",
          },
        ],
      },
      loadingByProject: { p1: false },
      fetchIntegrations: jest.fn(),
      createIntegration: jest.fn(),
      patchIntegration: jest.fn(),
      deleteIntegration: jest.fn(),
      syncIntegration: sync,
    });
    render(<VCSIntegrationsPage />);
    await waitFor(() => screen.getByText("octocat/hello"));
    fireEvent.click(screen.getByRole("button", { name: /重新同步/ }));
    await waitFor(() => expect(sync).toHaveBeenCalledWith("i1"));
  });
});
