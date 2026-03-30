import { render, screen } from "@testing-library/react";
import { WorkspaceLayout } from "./workspace-layout";

describe("WorkspaceLayout", () => {
  it("renders sidebar, main content, and rail using the configured widths", () => {
    const { container } = render(
      <WorkspaceLayout
        sidebar={<div>Sidebar</div>}
        rail={<div>Rail</div>}
        sidebarWidth="280px"
        railWidth="360px"
        flush
        className="workspace"
      >
        <div>Main content</div>
      </WorkspaceLayout>,
    );

    const root = container.firstChild as HTMLDivElement;

    expect(root).toHaveClass("workspace");
    expect(root).toHaveClass("-m-6");
    expect(root.style.display).toBe("grid");
    expect(root.style.gridTemplateColumns).toBe("280px minmax(0,1fr) 360px");
    expect(screen.getByText("Sidebar").closest("aside")).toHaveClass("border-r");
    expect(screen.getByText("Rail").closest("aside")).toHaveClass("border-l");
    expect(screen.getByText("Main content").closest("main")).toHaveClass("overflow-y-auto");
  });

  it("renders a single-column main area when there are no side regions", () => {
    const { container } = render(
      <WorkspaceLayout>
        <div>Only main</div>
      </WorkspaceLayout>,
    );

    const root = container.firstChild as HTMLDivElement;

    expect(screen.getByText("Only main")).toBeInTheDocument();
    expect(root.style.display).toBe("");
    expect(root.style.gridTemplateColumns).toBe("");
    expect(container.querySelectorAll("aside")).toHaveLength(0);
  });
});
