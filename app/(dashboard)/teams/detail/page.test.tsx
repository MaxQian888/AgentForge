import { render, screen, waitFor } from "@testing-library/react";
import TeamDetailPage from "./page";

const suspendedPromise = new Promise<never>(() => {});
const replace = jest.fn();
const searchParamsState = {
  teamId: null as string | null,
  suspend: false,
};

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("next/navigation", () => ({
  useSearchParams: () => {
    if (searchParamsState.suspend) {
      throw suspendedPromise;
    }
    return {
      get: (key: string) => (key === "id" ? searchParamsState.teamId : null),
    };
  },
  useRouter: () => ({ replace }),
}));

jest.mock("@/components/team/team-detail-view", () => ({
  TeamDetailView: ({ teamId }: { teamId: string }) => <div data-testid="team-detail-view">{teamId}</div>,
}));

jest.mock("@/components/shared/page-header", () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

jest.mock("@/components/ui/skeleton", () => ({
  Skeleton: () => <div data-testid="skeleton" />,
}));

describe("TeamDetailPage", () => {
  beforeEach(() => {
    replace.mockReset();
    searchParamsState.teamId = null;
    searchParamsState.suspend = false;
  });

  it("redirects back to the teams list when no team id is present", async () => {
    render(<TeamDetailPage />);

    await waitFor(() => {
      expect(replace).toHaveBeenCalledWith("/teams");
    });
    expect(screen.queryByTestId("team-detail-view")).not.toBeInTheDocument();
  });

  it("renders the selected team detail view", () => {
    searchParamsState.teamId = "team-7";

    render(<TeamDetailPage />);

    expect(screen.getByTestId("team-detail-view")).toHaveTextContent("team-7");
  });

  it("shows the loading fallback while the route search params are suspended", () => {
    searchParamsState.suspend = true;

    render(<TeamDetailPage />);

    expect(screen.getByRole("heading", { name: "Loading..." })).toBeInTheDocument();
    expect(screen.getAllByTestId("skeleton")).toHaveLength(2);
  });
});
