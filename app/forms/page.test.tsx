import { render, screen } from "@testing-library/react";
import PublicFormsPage from "./page";

const suspendedPromise = new Promise<never>(() => {});
let shouldSuspend = false;

jest.mock("./page-content", () => ({
  PublicFormsPageContent: () => {
    if (shouldSuspend) {
      throw suspendedPromise;
    }
    return <div data-testid="public-forms-page-content" />;
  },
}));

describe("PublicFormsPage", () => {
  beforeEach(() => {
    shouldSuspend = false;
  });

  it("renders the route content when the search params are ready", () => {
    render(<PublicFormsPage />);

    expect(screen.getByTestId("public-forms-page-content")).toBeInTheDocument();
  });

  it("shows the fallback card while the route content is suspended", () => {
    shouldSuspend = true;

    render(<PublicFormsPage />);

    expect(screen.getByText("Loading form")).toBeInTheDocument();
    expect(screen.getByText("Checking the requested form link...")).toBeInTheDocument();
  });
});
