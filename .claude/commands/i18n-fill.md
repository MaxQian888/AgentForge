Auto-translate missing i18n keys detected by the audit script.

**Steps:**

1. Run `node scripts/i18n-audit.js --json` and capture the JSON output from stdout
2. Parse the JSON array of `AuditResult` objects — each has `moduleName`, `missing[]` with `key`, `en`, `zh`, and `missingIn`
3. For each missing entry:
   - If `missingIn` includes `"en"`: the `zh` value is present — translate it to English
   - If `missingIn` includes `"zh-CN"`: the `en` value is present — translate it to Chinese (Simplified)
4. Read the corresponding `messages/{locale}/{namespace}.json` file
5. Add the translated key-value pair to the JSON, preserving the existing structure and nesting (keys use dotted notation like `"login.title"` which maps to `{ "login": { "title": "..." } }`)
6. Write the updated JSON back with `JSON.stringify(obj, null, 2)` + trailing newline
7. Print a summary of all keys that were translated and written

**Translation guidelines:**
- Match the tone and terminology of existing translations in the same namespace
- Preserve interpolation placeholders like `{count}`, `{name}`, `{time}` exactly as-is
- Keep translations concise and natural for UI text (buttons, labels, descriptions)
- For Chinese: use Simplified Chinese (zh-CN), not Traditional

**Important:**
- Do NOT modify keys that already have both en and zh-CN translations
- Only process entries from the `missing` array in the audit report
- If the audit reports 0 missing keys, just say "All translations are complete!"
