import { create } from "zustand";

export interface BreadcrumbItem {
  label: string;
  href?: string;
}

export interface RecentCommandItem {
  id: string;
  label: string;
  href: string;
  kind: "navigation" | "action";
}

interface LayoutState {
  breadcrumbs: BreadcrumbItem[];
  commandPaletteOpen: boolean;
  recentCommands: RecentCommandItem[];
  setBreadcrumbs: (breadcrumbs: BreadcrumbItem[]) => void;
  openCommandPalette: () => void;
  closeCommandPalette: () => void;
  toggleCommandPalette: () => void;
  recordCommand: (command: RecentCommandItem) => void;
  clearRecentCommands: () => void;
}

export const useLayoutStore = create<LayoutState>()((set) => ({
  breadcrumbs: [],
  commandPaletteOpen: false,
  recentCommands: [],
  setBreadcrumbs: (breadcrumbs) => set({ breadcrumbs }),
  openCommandPalette: () => set({ commandPaletteOpen: true }),
  closeCommandPalette: () => set({ commandPaletteOpen: false }),
  toggleCommandPalette: () =>
    set((state) => ({ commandPaletteOpen: !state.commandPaletteOpen })),
  recordCommand: (command) =>
    set((state) => ({
      recentCommands: [
        command,
        ...state.recentCommands.filter((item) => item.id !== command.id),
      ].slice(0, 5),
    })),
  clearRecentCommands: () => set({ recentCommands: [] }),
}));
