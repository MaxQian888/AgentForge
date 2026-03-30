## 1. Dependencies and Theme Infrastructure

- [x] 1.1 Add `next-themes` to `package.json` via `pnpm add next-themes`
- [x] 1.2 Create `lib/stores/appearance-store.ts` with `useAppearanceStore` (Zustand + persist), exposing `theme: "light" | "dark" | "system"` and `setTheme`
- [x] 1.3 Wrap root layout (`app/layout.tsx`) with `ThemeProvider` from `next-themes` using `attribute="class"`, `enableSystem`, `disableTransitionOnChange`, and `defaultTheme` driven from the appearance store

## 2. Theme Toggle Component

- [x] 2.1 Create `components/ui/theme-toggle.tsx` — a segmented control (Light / Dark / System) that reads `useTheme` from `next-themes` and writes back on selection
- [x] 2.2 Add unit test or smoke test for `ThemeToggle` verifying it renders three options and calls `setTheme` on click

## 3. i18n Messages

- [x] 3.1 Add appearance message keys to `messages/en/settings.json`: `appearance`, `appearanceDesc`, `themeMode`, `themeLight`, `themeDark`, `themeSystem`, `language`
- [x] 3.2 Add the same appearance keys with Chinese translations to `messages/zh-CN/settings.json`

## 4. Settings Page — Appearance Section

- [x] 4.1 Add an Appearance `<Card>` to `app/(dashboard)/settings/page.tsx` that renders above the project-scoped content (and outside `SettingsContent` so it is visible even with no project selected)
- [x] 4.2 Wire the theme mode selector in the Appearance card to `ThemeToggle` (or an equivalent `Select`) backed by `useAppearanceStore`
- [x] 4.3 Wire the language selector in the Appearance card to `useLocaleStore.setLocale`, showing all `SUPPORTED_LOCALES` with human-readable labels
- [x] 4.4 Ensure the Appearance card is rendered when no project is selected (settings page no-project state should still show Appearance)

## 5. Verification

- [x] 5.1 Confirm no FOUC on hard reload in both light and dark OS modes (manual check or snapshot test)
- [x] 5.2 Update or add tests in `app/(dashboard)/settings/page.test.tsx` to cover the Appearance card rendering and locale selector interaction
- [x] 5.3 Run `pnpm test` and confirm all tests pass
- [x] 5.4 Run `pnpm exec tsc --noEmit` and confirm no type errors
