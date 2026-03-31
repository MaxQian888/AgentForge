import { filterPaletteCommands, type PaletteSearchCommand } from "./command-palette";

describe("filterPaletteCommands", () => {
  const commands: PaletteSearchCommand[] = [
    {
      id: "agents",
      label: "Agents",
      value: "navigation agents",
      href: "/agents",
      kind: "navigation",
      category: "navigation",
      keywords: ["runtime", "workers"],
    },
    {
      id: "settings",
      label: "Settings",
      value: "navigation settings",
      href: "/settings",
      kind: "navigation",
      category: "navigation",
    },
    {
      id: "project-settings",
      label: "Project Settings",
      value: "context project settings",
      href: "/settings",
      kind: "navigation",
      category: "context",
    },
  ];

  it("matches commands when the query contains a typo", () => {
    const results = filterPaletteCommands(commands, "agnets");

    expect(results.map((command) => command.id)).toContain("agents");
    expect(results.map((command) => command.id)).not.toContain("settings");
  });

  it("ranks stronger exact and prefix matches ahead of looser partial matches", () => {
    const results = filterPaletteCommands(commands, "set");

    expect(results[0]?.id).toBe("settings");
    expect(results[1]?.id).toBe("project-settings");
  });
});
