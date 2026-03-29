import { render, screen } from "@testing-library/react";
import { PublicFormsPageContent } from "./page-content";

const publicFormPageClientMock = jest.fn(({ slug }: { slug: string }) => (
  <div data-testid="public-form-page-client">{slug}</div>
));
const getSearchParamMock = jest.fn<string | null, [string]>();

jest.mock("next/navigation", () => ({
  useSearchParams: () => ({
    get: getSearchParamMock,
  }),
}));

jest.mock("./[slug]/page-client", () => ({
  PublicFormPageClient: (props: { slug: string }) => publicFormPageClientMock(props),
}));

describe("PublicFormsPageContent", () => {
  beforeEach(() => {
    getSearchParamMock.mockReset();
    publicFormPageClientMock.mockClear();
  });

  it("shows the empty-state card when no slug is provided", () => {
    getSearchParamMock.mockReturnValue(null);

    render(<PublicFormsPageContent />);

    expect(screen.getByText("Form not found")).toBeInTheDocument();
    expect(
      screen.getByText("Provide a form slug in the query string to load a public form."),
    ).toBeInTheDocument();
    expect(publicFormPageClientMock).not.toHaveBeenCalled();
  });

  it("renders the public form route when a slug is present", () => {
    getSearchParamMock.mockReturnValue("bug-report");

    render(<PublicFormsPageContent />);

    expect(screen.getByTestId("public-form-page-client")).toHaveTextContent("bug-report");
    expect(publicFormPageClientMock).toHaveBeenCalledWith({ slug: "bug-report" });
  });
});
