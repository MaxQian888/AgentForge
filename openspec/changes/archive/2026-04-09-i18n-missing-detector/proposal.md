## Why

The project has 21 i18n namespaces across `en` and `zh-CN` locales, with 132+ files using `useTranslations()`. Translation keys are added during feature development but corresponding messages in one or both locales are often forgotten, causing runtime missing-key fallbacks. There is no automated way to detect these gaps — developers discover them only through manual testing or user reports.

next-intl does not ship any built-in CLI or extraction tooling. The project needs a custom workflow script that statically analyzes code usage against message files to surface missing translations before they reach production.

## What Changes

### i18n Audit Script (`scripts/i18n-audit.js`)

A Node.js script that:

1. **Extracts usage from source code** — Scans `app/`, `components/`, `hooks/`, `lib/` for `useTranslations("namespace")` calls, then collects all `t("key")` / `t("key.nested")` invocations within the same scope
2. **Loads message bundles** — Reads `messages/en/*.json` and `messages/zh-CN/*.json` and flattens them to dotted-key format for comparison
3. **Cross-references and reports** — For each extracted `(namespace, key)` pair, checks whether `en` and `zh-CN` both have the message. Outputs a structured report of missing entries
4. **Outputs actionable JSON** — Result format: `{ moduleName: string, missing: { key: string, en?: string, zh?: string }[] }[]` — entries where at least one locale is missing

### Translation Fill via Coding Agent Skill (`/i18n-fill`)

A Claude Code slash command that:
- Runs the audit script to get the missing keys report
- Reads existing translations from both locales for tone/style context
- Translates missing keys directly using the agent's built-in multilingual capability
- Writes translations back into the corresponding JSON files, preserving key order and formatting
- No API key, no external SDK — runs entirely within the developer's coding agent session

### CI Integration

- `pnpm i18n:audit` — runs the audit, exits non-zero if missing keys found

## Out of Scope

- Runtime missing-key detection or error boundaries
- Adding new locales beyond en / zh-CN
- Migrating away from next-intl
- Extracting keys from dynamic expressions like `t(variable)` (only static string literals)
