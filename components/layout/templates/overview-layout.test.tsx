import { render, screen } from "@testing-library/react";
import { OverviewLayout } from "./overview-layout";

const pageHeaderMock = jest.fn();

jest.mock("@/components/shared/page-header", () => ({
  PageHeader: (props: {
    title: string;
    description?: string;
    actions?: React.ReactNode;
    breadcrumbs?: Array<{ label: string; href?: string }>;
  }) => {
    pageHeaderMock(props);
    return (
      <div data-testid="page-header">
        <span>{props.title}</span>
        {props.actions}
      </div>
    );
  },
}));

describe("OverviewLayout", () => {
  beforeEach(() => {
    pageHeaderMock.mockClear();
  });

  it("renders the header, metrics row, and overview grid", () => {
    render(
      <OverviewLayout
        title="Overview"
        description="Top-level delivery metrics."
        breadcrumbs={[{ label: "Dashboard" }]}
        actions={<button type="button">Refresh</button>}
        metrics={<div>Metric A</div>}
      >
        <section>Primary content</section>
        <section>Secondary content</section>
      </OverviewLayout>,
    );

    expect(pageHeaderMock).toHaveBeenCalledWith(
      expect.objectContaining({
        title: "Overview",
        description: "Top-level delivery metrics.",
      }),
    );
    expect(screen.getByText("Metric A").parentElement).toHaveClass("grid-cols-2");
    expect(screen.getByText("Primary content").parentElement).toHaveClass("lg:grid-cols-2");
    expect(screen.getByText("Secondary content")).toBeInTheDocument();
  });
});
