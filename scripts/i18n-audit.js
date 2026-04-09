/* eslint-disable @typescript-eslint/no-require-imports */
/**
 * i18n Audit Script
 *
 * Scans source code for useTranslations() / t() calls and cross-references
 * them against messages/en/ and messages/zh-CN/ to detect missing translations.
 *
 * Usage:
 *   node scripts/i18n-audit.js              # Human-readable table + JSON
 *   node scripts/i18n-audit.js --json       # JSON-only to stdout
 *   node scripts/i18n-audit.js --dir app    # Override scan directories
 *
 * Exit codes:
 *   0 — no missing keys
 *   1 — missing keys found
 */

const fs = require("fs");
const path = require("path");
const {
  loadNamespaceMessages,
  getAllNamespaces,
} = require("./i18n-utils");

const PROJECT_ROOT = path.resolve(__dirname, "..");
const DEFAULT_DIRS = ["app", "components", "hooks", "lib"];

// ── 2.1 Find source files ──────────────────────────────────────────────────

/**
 * @param {string[]} dirs
 * @returns {string[]}
 */
function findSourceFiles(dirs) {
  /** @type {string[]} */
  const files = [];

  for (const dir of dirs) {
    const absDir = path.join(PROJECT_ROOT, dir);
    if (!fs.existsSync(absDir)) continue;
    collectFiles(absDir, files);
  }

  return files;
}

/**
 * Recursively collects .ts/.tsx files, skipping node_modules and test files.
 * @param {string} dir
 * @param {string[]} out
 */
function collectFiles(dir, out) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === "node_modules") continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      collectFiles(full, out);
    } else if (
      /\.(ts|tsx)$/.test(entry.name) &&
      !/\.(test|spec)\./.test(entry.name)
    ) {
      out.push(full);
    }
  }
}

// ── 2.2 Extract translation usage ──────────────────────────────────────────

/**
 * @typedef {{ namespace: string, keys: string[], varName: string }} TranslationUsage
 */

/**
 * Extracts useTranslations("ns") calls and their corresponding t("key") usage.
 *
 * @param {string} fileContent
 * @param {string} filePath
 * @returns {TranslationUsage[]}
 */
function extractTranslationUsage(fileContent, filePath) {
  /** @type {TranslationUsage[]} */
  const results = [];

  // Match: const t = useTranslations("namespace")
  // Also handles: const tAuth = useTranslations("auth")
  const hookRegex =
    /\bconst\s+(\w+)\s*=\s*useTranslations\(\s*["']([^"']+)["']\s*\)/g;

  let match;
  while ((match = hookRegex.exec(fileContent)) !== null) {
    const varName = match[1];
    const namespace = match[2];

    // Find all t("key") or t("key", ...) calls for this variable
    // Escape varName for regex safety
    const escaped = varName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const keyRegex = new RegExp(
      `\\b${escaped}\\(\\s*["']([^"']+)["']`,
      "g"
    );

    /** @type {string[]} */
    const keys = [];
    let keyMatch;
    while ((keyMatch = keyRegex.exec(fileContent)) !== null) {
      keys.push(keyMatch[1]);
    }

    results.push({ namespace, keys, varName });
  }

  // Warn on dynamic useTranslations calls
  const dynamicRegex =
    /\buseTranslations\(\s*(?!["'])(\w+)/g;
  while ((match = dynamicRegex.exec(fileContent)) !== null) {
    const rel = path.relative(PROJECT_ROOT, filePath);
    process.stderr.write(
      `⚠ Dynamic useTranslations(${match[1]}) in ${rel} — skipped\n`
    );
  }

  return results;
}

// ── 2.3 Aggregate by namespace ─────────────────────────────────────────────

/**
 * @param {TranslationUsage[]} extractions
 * @returns {Map<string, Set<string>>}
 */
function aggregateByNamespace(extractions) {
  /** @type {Map<string, Set<string>>} */
  const map = new Map();

  for (const { namespace, keys } of extractions) {
    if (!map.has(namespace)) {
      map.set(namespace, new Set());
    }
    const set = map.get(namespace);
    for (const key of keys) {
      set.add(key);
    }
  }

  return map;
}

// ── 2.4 Cross-reference ────────────────────────────────────────────────────

/**
 * @typedef {{
 *   moduleName: string,
 *   keys: Record<string, { en?: string, zh?: string }>,
 *   missing: Array<{ key: string, en?: string, zh?: string, missingIn: string[] }>,
 *   orphans: Array<{ key: string, locale: string }>
 * }} AuditResult
 */

/**
 * @param {Map<string, Set<string>>} usedKeys
 * @returns {AuditResult[]}
 */
function crossReference(usedKeys) {
  const allNamespaces = getAllNamespaces();
  /** @type {AuditResult[]} */
  const results = [];

  for (const ns of allNamespaces) {
    const en = loadNamespaceMessages("en", ns);
    const zh = loadNamespaceMessages("zh-CN", ns);
    const used = usedKeys.get(ns) || new Set();

    /** @type {Record<string, { en?: string, zh?: string }>} */
    const keys = {};
    /** @type {AuditResult["missing"]} */
    const missing = [];

    for (const key of used) {
      const enVal = en[key];
      const zhVal = zh[key];
      keys[key] = { en: enVal, zh: zhVal };

      /** @type {string[]} */
      const missingIn = [];
      if (enVal === undefined) missingIn.push("en");
      if (zhVal === undefined) missingIn.push("zh-CN");

      if (missingIn.length > 0) {
        missing.push({ key, en: enVal, zh: zhVal, missingIn });
      }
    }

    // 2.6 Orphan detection — keys in message files but never referenced in code
    /** @type {AuditResult["orphans"]} */
    const orphans = [];
    for (const key of Object.keys(en)) {
      if (!used.has(key)) orphans.push({ key, locale: "en" });
    }
    for (const key of Object.keys(zh)) {
      if (!used.has(key) && !en[key]) {
        // Only flag zh-CN orphans that aren't also in en (avoid duplicates)
        orphans.push({ key, locale: "zh-CN" });
      }
    }

    results.push({ moduleName: ns, keys, missing, orphans });
  }

  return results;
}

// ── 2.5 CLI entry point ────────────────────────────────────────────────────

function main() {
  const args = process.argv.slice(2);
  const jsonOnly = args.includes("--json");

  // Parse --dir flags
  const dirFlags = [];
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--dir" && args[i + 1]) {
      dirFlags.push(args[++i]);
    }
  }
  const dirs = dirFlags.length > 0 ? dirFlags : DEFAULT_DIRS;

  // Step 1: Find files and extract
  const files = findSourceFiles(dirs);
  if (!jsonOnly) {
    process.stderr.write(`Scanning ${files.length} files...\n`);
  }

  /** @type {TranslationUsage[]} */
  const allExtractions = [];
  for (const file of files) {
    const content = fs.readFileSync(file, "utf-8");
    allExtractions.push(...extractTranslationUsage(content, file));
  }

  // Step 2: Aggregate and cross-reference
  const byNamespace = aggregateByNamespace(allExtractions);
  const results = crossReference(byNamespace);

  // Step 3: Output
  const totalMissing = results.reduce((sum, r) => sum + r.missing.length, 0);
  const totalOrphans = results.reduce((sum, r) => sum + r.orphans.length, 0);

  if (!jsonOnly) {
    // Summary table to stderr
    process.stderr.write("\n");
    process.stderr.write(
      padEnd("Namespace", 16) +
        padEnd("Used Keys", 12) +
        padEnd("Missing EN", 12) +
        padEnd("Missing ZH", 12) +
        padEnd("Orphans", 10) +
        "\n"
    );
    process.stderr.write("─".repeat(62) + "\n");

    for (const r of results) {
      const usedCount = Object.keys(r.keys).length;
      if (usedCount === 0 && r.orphans.length === 0) continue;

      const missingEn = r.missing.filter((m) =>
        m.missingIn.includes("en")
      ).length;
      const missingZh = r.missing.filter((m) =>
        m.missingIn.includes("zh-CN")
      ).length;

      process.stderr.write(
        padEnd(r.moduleName, 16) +
          padEnd(String(usedCount), 12) +
          padEnd(missingEn > 0 ? `${missingEn} ✗` : "0 ✓", 12) +
          padEnd(missingZh > 0 ? `${missingZh} ✗` : "0 ✓", 12) +
          padEnd(String(r.orphans.length), 10) +
          "\n"
      );
    }

    process.stderr.write("─".repeat(62) + "\n");
    process.stderr.write(
      `Total: ${totalMissing} missing, ${totalOrphans} orphans\n\n`
    );

    if (totalMissing > 0) {
      // Show details of missing keys
      for (const r of results) {
        if (r.missing.length === 0) continue;
        process.stderr.write(`\n[${r.moduleName}] Missing keys:\n`);
        for (const m of r.missing) {
          process.stderr.write(
            `  ${m.key}  (missing in: ${m.missingIn.join(", ")})\n`
          );
        }
      }

      process.stderr.write(
        `\nRun /i18n-fill to auto-translate ${totalMissing} missing keys.\n`
      );
    }
  }

  // JSON report to stdout (only entries with missing or for --json full report)
  const report = jsonOnly
    ? results
    : results.filter((r) => r.missing.length > 0);

  const exitCode = totalMissing > 0 ? 1 : 0;
  process.stdout.write(JSON.stringify(report, null, 2) + "\n", () => {
    process.exit(exitCode);
  });
}

/** @param {string} str @param {number} len */
function padEnd(str, len) {
  return str.length >= len ? str + " " : str + " ".repeat(len - str.length);
}

main();
