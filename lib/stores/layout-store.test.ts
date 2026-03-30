import { useLayoutStore } from "./layout-store";

describe("useLayoutStore", () => {
  beforeEach(() => {
    useLayoutStore.setState({
      breadcrumbs: [],
      commandPaletteOpen: false,
    });
  });

  it("stores breadcrumb updates", () => {
    useLayoutStore.getState().setBreadcrumbs([
      { label: "Project", href: "/project" },
      { label: "Settings" },
    ]);

    expect(useLayoutStore.getState().breadcrumbs).toEqual([
      { label: "Project", href: "/project" },
      { label: "Settings" },
    ]);
  });

  it("opens, closes, and toggles the command palette", () => {
    const store = useLayoutStore.getState();

    store.openCommandPalette();
    expect(useLayoutStore.getState().commandPaletteOpen).toBe(true);

    store.toggleCommandPalette();
    expect(useLayoutStore.getState().commandPaletteOpen).toBe(false);

    store.toggleCommandPalette();
    expect(useLayoutStore.getState().commandPaletteOpen).toBe(true);

    store.closeCommandPalette();
    expect(useLayoutStore.getState().commandPaletteOpen).toBe(false);
  });
});
