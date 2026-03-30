import { renderHook } from "@testing-library/react";
import { type BreadcrumbItem, useLayoutStore } from "@/lib/stores/layout-store";
import { useBreadcrumbs } from "./use-breadcrumbs";

describe("useBreadcrumbs", () => {
  beforeEach(() => {
    useLayoutStore.setState({
      breadcrumbs: [],
      commandPaletteOpen: false,
    });
  });

  it("writes breadcrumbs to the layout store on mount and clears them on unmount", () => {
    const breadcrumbs: BreadcrumbItem[] = [
      { label: "Projects", href: "/projects" },
      { label: "Roadmap" },
    ];

    const { unmount } = renderHook(({ items }) => useBreadcrumbs(items), {
      initialProps: { items: breadcrumbs },
    });

    expect(useLayoutStore.getState().breadcrumbs).toEqual(breadcrumbs);

    unmount();

    expect(useLayoutStore.getState().breadcrumbs).toEqual([]);
  });

  it("updates stored breadcrumbs when the breadcrumb content changes", () => {
    const initialBreadcrumbs: BreadcrumbItem[] = [
      { label: "Teams", href: "/teams" },
      { label: "Design" },
    ];
    const nextBreadcrumbs: BreadcrumbItem[] = [
      { label: "Teams", href: "/teams" },
      { label: "Operations" },
    ];

    const { rerender } = renderHook(({ items }) => useBreadcrumbs(items), {
      initialProps: { items: initialBreadcrumbs },
    });

    expect(useLayoutStore.getState().breadcrumbs).toEqual(initialBreadcrumbs);

    rerender({ items: nextBreadcrumbs });

    expect(useLayoutStore.getState().breadcrumbs).toEqual(nextBreadcrumbs);
  });
});
