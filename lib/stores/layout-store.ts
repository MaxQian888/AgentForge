import { create } from "zustand";

export interface BreadcrumbItem {
  label: string;
  href?: string;
}

interface LayoutState {
  breadcrumbs: BreadcrumbItem[];
  commandPaletteOpen: boolean;
  setBreadcrumbs: (breadcrumbs: BreadcrumbItem[]) => void;
  openCommandPalette: () => void;
  closeCommandPalette: () => void;
  toggleCommandPalette: () => void;
}

export const useLayoutStore = create<LayoutState>()((set) => ({
  breadcrumbs: [],
  commandPaletteOpen: false,
  setBreadcrumbs: (breadcrumbs) => set({ breadcrumbs }),
  openCommandPalette: () => set({ commandPaletteOpen: true }),
  closeCommandPalette: () => set({ commandPaletteOpen: false }),
  toggleCommandPalette: () =>
    set((state) => ({ commandPaletteOpen: !state.commandPaletteOpen })),
}));
