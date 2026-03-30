# app-appearance-preferences Specification

## Purpose
Define the user-facing appearance preferences contract so users can control theme mode and application language, with preferences persisted and applied without flash on load.

## Requirements

### Requirement: Theme preference is persisted and applied without flash
The system SHALL persist the user's theme preference (`"light"`, `"dark"`, or `"system"`) in client-side storage and apply the corresponding `.dark` or light class to the root `<html>` element before the first paint. When the preference is `"system"`, the applied theme SHALL follow the OS `prefers-color-scheme` media query at runtime.

#### Scenario: First-time visitor — no stored preference
- **WHEN** a user opens the application for the first time with no stored theme preference
- **THEN** the application renders with a theme matching the OS `prefers-color-scheme` (light or dark)
- **THEN** no visible flash of unstyled or wrong-theme content occurs before the page is interactive

#### Scenario: Returning visitor with stored dark preference
- **WHEN** a user opens the application and their stored theme preference is `"dark"`
- **THEN** the `.dark` class is applied to `<html>` before the first paint
- **THEN** all color tokens from the `.dark` CSS block are active immediately

#### Scenario: OS color scheme changes while app is open with system preference
- **WHEN** the user's stored preference is `"system"` and they toggle their OS dark/light mode
- **THEN** the application theme switches without requiring a page reload

### Requirement: User can change theme mode from the settings page
The system SHALL expose a theme mode selector in the user-facing Appearance settings section with options for Light, Dark, and System. Selecting a mode SHALL immediately update the active theme and persist the choice.

#### Scenario: User switches to dark mode
- **WHEN** a user selects "Dark" in the Appearance settings section
- **THEN** the application theme switches to dark mode immediately
- **THEN** the preference is persisted so subsequent sessions start in dark mode

#### Scenario: User switches to system-follow mode
- **WHEN** a user selects "System" in the Appearance settings section
- **THEN** the application theme follows the OS `prefers-color-scheme` value
- **THEN** the "System" option is reflected as selected in the theme selector

### Requirement: User can change the application language from the settings page
The system SHALL expose a language selector in the Appearance settings section with options for all supported locales (`en`, `zh-CN`). Selecting a locale SHALL immediately switch the application language and persist the choice.

#### Scenario: User switches to Chinese
- **WHEN** a user selects "中文 (简体)" in the language selector
- **THEN** all i18n strings in the application switch to `zh-CN`
- **THEN** `document.documentElement.lang` is updated to `"zh-CN"`
- **THEN** the preference is persisted so subsequent sessions start in the selected language

#### Scenario: Language selector shows current locale
- **WHEN** a user opens the Appearance settings section
- **THEN** the language selector shows the currently active locale as its selected value
