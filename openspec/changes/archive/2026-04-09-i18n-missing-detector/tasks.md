## 1. Shared Utilities (`scripts/i18n-utils.js`)

- [x] 1.1 Create `flattenMessages(obj, prefix?)` — recursively flattens nested JSON to `Record<string, string>` with dotted keys
- [x] 1.2 Create `loadNamespaceMessages(locale, namespace)` — reads `messages/{locale}/{namespace}.json`, returns flattened key-value map
- [x] 1.3 Create `getAllNamespaces()` — reads `messages/en/` directory listing, returns namespace names (minus `.json` extension)
- [x] 1.4 Create `writeNamespaceMessages(locale, namespace, flatMap)` — unflattens dotted keys back to nested structure and writes JSON with consistent formatting
- [x] 1.5 Add type definitions for `AuditResult`, `MissingEntry` interfaces (via JSDoc)

## 2. Code Extraction (`scripts/i18n-audit.js`)

- [x] 2.1 Implement `findSourceFiles(dirs)` — recursive fs.readdirSync under `app/`, `components/`, `hooks/`, `lib/` excluding `node_modules`, `*.test.*`, `*.spec.*`
- [x] 2.2 Implement `extractTranslationUsage(fileContent, filePath)` — regex-based extraction:
  - Find `useTranslations("namespace")` → capture namespace and variable name (usually `t`)
  - Find all `{varName}("key"` calls → capture keys
  - Handle multiple `useTranslations` in one file (different variable names)
  - Warn on non-literal arguments (log to stderr)
- [x] 2.3 Implement `aggregateByNamespace(extractions)` — deduplicate and group all used keys per namespace
- [x] 2.4 Implement `crossReference(usedKeys, enMessages, zhMessages)` — for each namespace, compare used keys against both locale maps, produce `AuditResult[]`
- [x] 2.5 Implement CLI entry point:
  - Accept optional `--json` flag for JSON-only output
  - Accept optional `--dir` to override scan directories
  - Print summary table to stderr (namespace, total keys, missing en, missing zh)
  - Print JSON report to stdout
  - Exit code 1 if any missing keys found
- [x] 2.6 Also detect "orphan" keys — keys present in message files but never referenced in code (report separately, lower priority)

## 3. Translation Fill Skill

- [x] 3.1 Create a Claude Code custom slash command / skill (`.claude/commands/i18n-fill.md`) that:
  - Runs `node scripts/i18n-audit.js --json` to get the missing keys report
  - Reads existing translations from both locales for context (tone, style, terminology)
  - Translates missing keys directly (agent has built-in multilingual capability)
  - Writes translations back to the correct `messages/{locale}/{namespace}.json`
  - Prints summary of changes made
- [x] 3.2 The audit script's stderr output includes agent-friendly instructions ("Run /i18n-fill to auto-translate N missing keys")

## 4. Package Scripts & CI

- [x] 4.1 Add to `package.json`:
  - `"i18n:audit": "node scripts/i18n-audit.js"`
- [x] 4.2 No tsx needed — scripts written as plain .js (consistent with existing scripts/)
- [x] 4.3 Document usage in a comment block at the top of each script

## 5. Testing & Validation

- [x] 5.1 Run `pnpm i18n:audit` against current codebase — found 8 missing keys across 4 namespaces, 425 orphans
- [x] 5.2 Verify output format matches the `AuditResult` interface — confirmed: `moduleName`, `keys`, `missing`, `orphans`
