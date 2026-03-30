import { render, screen } from "@testing-library/react";
import { ListLayout } from "./list-layout";

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
        {props.description ? <span>{props.description}</span> : null}
        {props.actions}
      </div>
    );
  },
}));

describe("ListLayout", () => {
  beforeEach(() => {
    pageHeaderMock.mockClear();
  });

  it("renders the header, optional toolbar, and content area", () => {
    const { container } = render(
      <ListLayout
        title="Projects"
        description="Track active delivery work."
        breadcrumbs={[{ label: "Home", href: "/" }, { label: "Projects" }]}
        actions={<button type="button">New project</button>}
        toolbar={<div>Toolbar</div>}
        className="custom-layout"
      >
        <div>Project list</div>
      </ListLayout>,
    );

    expect(pageHeaderMock).toHaveBeenCalledWith(
      expect.objectContaining({
        title: "Projects",
        description: "Track active delivery work.",
        breadcrumbs: [{ label: "Home", href: "/" }, { label: "Projects" }],
      }),
    );
    expect(screen.getByTestId("page-header")).toBeInTheDocument();
    expect(screen.getByText("Toolbar")).toBeInTheDocument();
    expect(screen.getByText("Project list")).toBeInTheDocument();
    expect(container.firstChild).toHaveClass("custom-layout");
  });
});
