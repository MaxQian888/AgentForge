## Context

AgentForge's CSS (`globals.css`) already defines full light and dark token sets using oklch CSS variables and a `.dark` class selector. The locale store (`lib/stores/locale-store.ts`) and i18n provider (`lib/i18n/provider.tsx`) are wired up and persisted. However, the root layout does not include any theme-management infrastructure, so the `.dark` class is never applied programmatically. The settings page only exposes project-scoped settings — no user-preference surface exists. Users cannot change appearance or language from the UI.

## Goals / Non-Goals

**Goals:**
- Apply theme class (`light` / `dark` / `system`) at the root `<html>` element reactively with no FOUC.
- Add a client-side `useAppearanceStore` that persists theme preference.
- Provide a `ThemeToggle` component reusable across the app.
- Expose an **Appearance** card in the settings page with theme and locale selectors.
- Add i18n message keys for the new UI in both `en` and `zh-CN`.

**Non-Goals:**
- Per-project theme overrides.
- Server-side theme detection (cookie-based SSR theming).
- Additional locales beyond `zh-CN` and `en`.
- Custom color palettes or design-token editors.

## Decisions

### Use `next-themes` for theme management

**Decision**: Add `next-themes` as a dependency and use its `ThemeProvider` in the root layout.

**Rationale**: `next-themes` handles flash-of-unstyled-content (FOUC) suppression via an inline script injected before hydration, integrates cleanly with class-based dark mode (`class` attribute strategy), and exposes a `useTheme` hook that reacts to `prefers-color-scheme` for the `"system"` option. Rolling a custom solution would need the same inline script and adds maintenance surface.

**Alternative considered**: Store-only approach (Zustand + `useEffect` to toggle class). Rejected because `useEffect` runs after hydration, causing a visible flash on initial load.

### `useAppearanceStore` wraps `next-themes` `useTheme`

**Decision**: Create a thin `useAppearanceStore` (Zustand + persist) that stores the raw `"light" | "dark" | "system"` preference. The `ThemeProvider` reads the system preference; the store drives the initial value for `ThemeProvider`'s `defaultTheme`.

**Rationale**: Keeps theme state alongside other persisted app state and avoids `localStorage` key fragmentation. `next-themes` own persistence (also `localStorage`) is disabled to keep a single source of truth.

**Alternative considered**: Let `next-themes` own persistence entirely. Rejected because it introduces a second `localStorage` key and makes it harder to read the stored value outside React (e.g., for SSR cookie hints in a future iteration).

### Appearance card added to the existing settings page

**Decision**: Add an "Appearance" `<Card>` at the top of `SettingsContent`, before project-level sections.

**Rationale**: The settings page is the canonical configuration surface operators already navigate to. A dedicated `/preferences` route would fragment the UX; adding a clearly-labeled card at the top is consistent with the existing section pattern and requires no new routing.

## Risks / Trade-offs

- **FOUC risk on first load** → Mitigated by `next-themes` attribute script. On first visit (no stored preference) the `"system"` default is applied inline before paint.
- **`next-themes` dependency footprint** → Small (~3 kB). Acceptable for the FOUC guarantee.
- **Locale selector updates `document.documentElement.lang` via `I18nProvider` `useEffect`** — there is no SSR re-render on locale change. This is acceptable for a client-rendered app with static export.
- **Appearance store vs. project store divergence** → Appearance preferences are user/client-scoped, not project-scoped. Keeping them in a separate store avoids polluting `ProjectSettings`.

## Migration Plan

1. `pnpm add next-themes` — add dependency.
2. Add `ThemeProvider` wrapper in `app/layout.tsx` with `attribute="class"`, `defaultTheme="system"`, `enableSystem`, `disableTransitionOnChange`.
3. Create `lib/stores/appearance-store.ts`.
4. Create `components/ui/theme-toggle.tsx`.
5. Add Appearance card to `app/(dashboard)/settings/page.tsx`.
6. Add message keys to both locale bundles.

No migration is needed for existing users — the store hydrates to `"system"` on first load.

## Open Questions

- Should the `ThemeToggle` also be surfaced in the sidebar/header for quick access? (Deferred — implement in settings only for this change.)
