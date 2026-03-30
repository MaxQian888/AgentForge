import { render } from "@testing-library/react";
import { WorkspaceSkeleton } from "./workspace-skeleton";

describe("WorkspaceSkeleton", () => {
  it("renders sidebar, main content, and rail placeholders when enabled", () => {
    const { container } = render(
      <WorkspaceSkeleton showSidebar showRail className="workspace-skeleton" />,
    );

    expect(container.firstChild).toHaveClass("workspace-skeleton");
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(17);
    expect(container.querySelectorAll(".border-r")).toHaveLength(1);
    expect(container.querySelectorAll(".border-l")).toHaveLength(1);
  });

  it("can render only the main workspace placeholder", () => {
    const { container } = render(
      <WorkspaceSkeleton showSidebar={false} showRail={false} />,
    );

    expect(container.querySelectorAll(".border-r")).toHaveLength(0);
    expect(container.querySelectorAll(".border-l")).toHaveLength(0);
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(6);
  });
});
