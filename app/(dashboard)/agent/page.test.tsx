import { render, waitFor } from "@testing-library/react";
import AgentPage from "./page";

const replace = jest.fn();
const searchParamsState = {
  agentId: null as string | null,
};

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("next/navigation", () => ({
  useSearchParams: () => ({
    get: (key: string) => (key === "id" ? searchParamsState.agentId : null),
  }),
  useRouter: () => ({ replace }),
}));

describe("AgentPage", () => {
  beforeEach(() => {
    replace.mockReset();
    searchParamsState.agentId = null;
  });

  it("redirects to the agent detail tab when an id is provided", async () => {
    searchParamsState.agentId = "agent-3";

    render(<AgentPage />);

    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith("/agents?agent=agent-3");
    });
  });

  it("redirects to the agents list when the query string is missing an id", async () => {
    render(<AgentPage />);

    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith("/agents");
    });
  });
});
