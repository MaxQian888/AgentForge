## Context

The project uses next-intl v4.8.3 with 21 namespaces split across `messages/en/` and `messages/zh-CN/` directories. Each namespace is a separate JSON file imported through a barrel `index.ts` and normalized via `messages/normalize.ts` (converts dotted keys to nested objects). Code uses `useTranslations("namespace")` then `t("key")` or `t("dotted.key", { vars })`.

next-intl has **no built-in extraction CLI** — only a runtime SWC plugin. The project already has Jest tests for message normalization but no automated gap detection.

## Goals / Non-Goals

**Goals:**
- Detect all `useTranslations` + `t()` call sites statically
- Compare extracted keys against both locale message files
- Output machine-readable report of missing translations
- Provide an agent-powered translation fill via `/i18n-fill` skill
- Integrate into CI as a lint-style check

**Non-Goals:**
- Handle dynamic key expressions (`t(someVar)`)
- Handle `useFormatter` or `useNow` — only `useTranslations`
- Replace human review for critical translations
- Support non-TS/TSX source files

## Decisions

### 1. AST Parsing Strategy

**Choice: Regex-based extraction with scope tracking**

Rationale: A full TypeScript AST parser (ts-morph, babel) would be more robust but heavyweight for this use case. The patterns are highly regular:
- `useTranslations("literal")` — always a string literal argument
- `t("literal.key")` or `t("literal.key", { ... })` — always a string literal first arg

A regex approach with basic scope awareness (tracking which `useTranslations` call `t` refers to) covers 95%+ of actual usage. For the remaining edge cases (re-aliased `t`, destructured patterns), the script logs warnings rather than silently missing them.

### 2. Message File Reading

**Choice: Direct JSON parse of flat files, then flatten nested keys**

Read each `messages/{locale}/{namespace}.json` directly (before normalization) and recursively flatten nested objects to dotted-key format. This matches how keys are written in `t()` calls.

### 3. Output Format

```typescript
interface AuditResult {
  moduleName: string; // namespace, e.g. "auth"
  keys: Record<string, { en?: string; zh?: string }>; // all keys with their messages
  missing: {
    key: string;
    en?: string;   // present value, or undefined if missing
    zh?: string;   // present value, or undefined if missing
    missingIn: ("en" | "zh-CN")[];
  }[];
}
```

JSON array output to stdout, plus a human-readable summary table to stderr.

### 4. Translation Fill Strategy

**Choice: Claude Code skill (`/i18n-fill`) — no API key needed**

Instead of calling an LLM API directly, the fill step is designed as a **Claude Code slash command / skill** that the developer invokes within their coding agent session. The audit script outputs a structured report; the developer runs `/i18n-fill` (or the agent runs it automatically), and Claude Code translates the missing keys in-context — reading the existing locale files for tone/style consistency and writing the translations directly into the JSON files.

Workflow:
1. `pnpm i18n:audit` → produces `i18n-audit-report.json` (or stdout)
2. Developer invokes the fill skill (or the audit script prints actionable instructions for the agent)
3. The coding agent reads the report, translates missing keys using its built-in language capability, and writes them back to the correct `messages/{locale}/{namespace}.json` files

Benefits:
- Zero external dependencies (no API key, no SDK, no billing)
- Runs inside the developer's existing agent session
- Agent has full project context (style, tone, existing translations)
- Preserves key ordering via `JSON.stringify(obj, null, 2)`

### 5. File Structure

```
scripts/
  i18n-audit.js     # Core extraction + comparison logic
  i18n-utils.js     # Shared: file reading, key flattening, JSON writing
```

No separate `i18n-fill.ts` — translation fill is handled by the coding agent via skill invocation.

## Component Interaction

```
Source files (app/, components/, hooks/, lib/)
  │
  ▼  [regex extraction]
Per-file: { namespace: string, keys: string[] }
  │
  ▼  [aggregate by namespace]
Per-namespace: { moduleName, usedKeys: Set<string> }
  │
  ├─► messages/en/{ns}.json ──► flatten ──► enKeys: Map<key, value>
  ├─► messages/zh-CN/{ns}.json ──► flatten ──► zhKeys: Map<key, value>
  │
  ▼  [cross-reference]
AuditResult[] ──► stdout (JSON)
               ──► stderr (summary table)
               ──► exit code (0 = clean, 1 = missing found)
               │
               ▼  [optional: /i18n-fill skill]
            Coding agent translates ──► write back to JSON files
```
