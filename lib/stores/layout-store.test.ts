import { useLayoutStore } from "./layout-store";

describe("useLayoutStore", () => {
  beforeEach(() => {
    useLayoutStore.setState({
      breadcrumbs: [],
      commandPaletteOpen: false,
      recentCommands: [],
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

  it("records recent commands in recency order without duplicates", () => {
    const store = useLayoutStore.getState();

    store.recordCommand({
      id: "projects",
      label: "Projects",
      href: "/projects",
      kind: "navigation",
    });
    store.recordCommand({
      id: "spawn-agent",
      label: "Spawn agent",
      href: "/agents?action=spawn",
      kind: "action",
    });
    store.recordCommand({
      id: "projects",
      label: "Projects",
      href: "/projects",
      kind: "navigation",
    });

    expect(useLayoutStore.getState().recentCommands).toEqual([
      {
        id: "projects",
        label: "Projects",
        href: "/projects",
        kind: "navigation",
      },
      {
        id: "spawn-agent",
        label: "Spawn agent",
        href: "/agents?action=spawn",
        kind: "action",
      },
    ]);
  });

  it("keeps only the five most recent commands", () => {
    const store = useLayoutStore.getState();

    for (let index = 0; index < 6; index += 1) {
      store.recordCommand({
        id: `command-${index}`,
        label: `Command ${index}`,
        href: `/command-${index}`,
        kind: "navigation",
      });
    }

    expect(useLayoutStore.getState().recentCommands).toHaveLength(5);
    expect(useLayoutStore.getState().recentCommands[0]?.id).toBe("command-5");
    expect(useLayoutStore.getState().recentCommands[4]?.id).toBe("command-1");
  });
});
