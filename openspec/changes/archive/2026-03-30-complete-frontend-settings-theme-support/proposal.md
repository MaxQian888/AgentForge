## Why

The application ships CSS variables for both light and dark themes and a persisted locale store, but no theme-switching infrastructure or user-preference UI exposes them — operators have no way to toggle appearance or language without editing storage directly. These partially-built capabilities need to be completed so the feature set matches what the codebase already documents.

## What Changes

- Wire `next-themes` `ThemeProvider` into the root layout so the `.dark` class is applied reactively based on user preference and `prefers-color-scheme`.
- Add a `useAppearanceStore` (Zustand + persist) that holds `theme: "light" | "dark" | "system"`.
- Add a `ThemeToggle` component (Light / Dark / System) that reads and writes the store.
- Add a user-level **Appearance** settings section to the existing settings page covering: theme mode selector, language/locale selector.
- Expose the existing `useLocaleStore` locale selector in the settings UI (the store and i18n infrastructure already exist but are unreachable from the UI).
- Add i18n message keys for the new appearance section in both `en` and `zh-CN` bundles.
- Wire `<html lang>` updates through `I18nProvider` (already partially done) and keep it consistent with the new locale selector.

## Capabilities

### New Capabilities

- `app-appearance-preferences`: User-scoped appearance preferences (theme mode, locale) stored client-side, applied globally, and configurable from the settings page.

### Modified Capabilities

- `project-settings-control-plane`: The settings page gains a new non-project-scoped "Appearance" section above the project sections. No existing requirement changes — additive only.

## Impact

- `app/layout.tsx` — wrap with `ThemeProvider`
- `lib/stores/appearance-store.ts` — new Zustand + persist store
- `components/ui/theme-toggle.tsx` — new component
- `app/(dashboard)/settings/page.tsx` — add Appearance card
- `messages/en/settings.json` and `messages/zh-CN/settings.json` — new appearance keys
- No backend changes required
- No breaking changes to existing project settings contract
