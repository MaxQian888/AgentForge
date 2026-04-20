import { render, screen, fireEvent, waitFor } from "@testing-library/react";

jest.mock("next/navigation", () => ({
  useParams: () => ({ id: "proj-1" }),
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({ useBreadcrumbs: jest.fn() }));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

jest.mock("@/lib/stores/secrets-store", () => {
  const actual = jest.requireActual("@/lib/stores/secrets-store");
  return {
    ...actual,
    useSecretsStore: Object.assign(jest.fn(), { setState: jest.fn() }),
  };
});

import { useSecretsStore } from "@/lib/stores/secrets-store";
import ProjectSecretsPage from "./page";

describe("ProjectSecretsPage", () => {
  beforeEach(() => {
    (useSecretsStore as unknown as jest.Mock).mockReturnValue({
      secretsByProject: {
        "proj-1": [
          {
            name: "GITHUB_TOKEN",
            description: "review token",
            createdBy: "user-1",
            createdAt: "2026-04-20T00:00:00Z",
            updatedAt: "2026-04-20T00:00:00Z",
          },
        ],
      },
      loadingByProject: { "proj-1": false },
      lastRevealedValue: {
        projectId: "proj-1",
        name: "NEW_KEY",
        value: "ghp_xyz",
      },
      fetchSecrets: jest.fn(),
      createSecret: jest.fn(),
      rotateSecret: jest.fn(),
      deleteSecret: jest.fn(),
      consumeRevealedValue: jest.fn(),
    });
  });

  it("renders the secrets table and the one-time-reveal dialog", () => {
    render(<ProjectSecretsPage />);

    expect(screen.getByText("GITHUB_TOKEN")).toBeInTheDocument();
    expect(screen.getByText(/请保存.*NEW_KEY.*的值/)).toBeInTheDocument();
    expect(screen.getByDisplayValue("ghp_xyz")).toBeInTheDocument();
  });

  it("clears the revealed value on dismiss", async () => {
    const consume = jest.fn();
    (useSecretsStore as unknown as jest.Mock).mockReturnValue({
      secretsByProject: { "proj-1": [] },
      loadingByProject: {},
      lastRevealedValue: { projectId: "proj-1", name: "X", value: "v" },
      fetchSecrets: jest.fn(),
      createSecret: jest.fn(),
      rotateSecret: jest.fn(),
      deleteSecret: jest.fn(),
      consumeRevealedValue: consume,
    });
    render(<ProjectSecretsPage />);
    fireEvent.click(screen.getByText("我已保存"));
    await waitFor(() => expect(consume).toHaveBeenCalled());
  });
});
