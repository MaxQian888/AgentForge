import { render, screen } from "@testing-library/react";
import TeamPage from "./page";

const suspendedPromise = new Promise<never>(() => {});
let shouldSuspend = false;

jest.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => (key: string) =>
    namespace ? `${namespace}.${key}` : key,
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("@/components/team/team-page-client", () => ({
  TeamPageClient: () => {
    if (shouldSuspend) {
      throw suspendedPromise;
    }
    return <div data-testid="team-page-client" />;
  },
}));

describe("TeamPage", () => {
  beforeEach(() => {
    shouldSuspend = false;
  });

  it("renders the team workspace client when the route is ready", () => {
    render(<TeamPage />);

    expect(screen.getByTestId("team-page-client")).toBeInTheDocument();
  });

  it("shows the suspense fallback while the team workspace is loading", () => {
    shouldSuspend = true;

    render(<TeamPage />);

    expect(screen.getByText("teams.teamPage.loading")).toBeInTheDocument();
  });
});
