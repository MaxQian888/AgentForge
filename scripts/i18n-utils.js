/* eslint-disable @typescript-eslint/no-require-imports */
/**
 * i18n Shared Utilities
 *
 * Provides helpers for reading, flattening, and writing next-intl message files.
 * Used by i18n-audit.js and the /i18n-fill Claude Code skill.
 *
 * Usage:
 *   const { flattenMessages, loadNamespaceMessages, getAllNamespaces, writeNamespaceMessages } = require('./i18n-utils');
 */

const fs = require("fs");
const path = require("path");

const MESSAGES_DIR = path.resolve(__dirname, "..", "messages");
const LOCALES = ["en", "zh-CN"];

/**
 * Recursively flattens a nested object into dotted-key format.
 * e.g. { login: { title: "Hi" } } → { "login.title": "Hi" }
 *
 * @param {Record<string, unknown>} obj
 * @param {string} [prefix]
 * @returns {Record<string, string>}
 */
function flattenMessages(obj, prefix) {
  /** @type {Record<string, string>} */
  const result = {};

  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      Object.assign(result, flattenMessages(value, fullKey));
    } else {
      result[fullKey] = String(value);
    }
  }

  return result;
}

/**
 * Unflattens dotted keys back to a nested object.
 * e.g. { "login.title": "Hi" } → { login: { title: "Hi" } }
 *
 * @param {Record<string, string>} flatMap
 * @returns {Record<string, unknown>}
 */
function unflattenMessages(flatMap) {
  /** @type {Record<string, unknown>} */
  const result = {};

  for (const [dottedKey, value] of Object.entries(flatMap)) {
    const segments = dottedKey.split(".");
    let current = result;

    for (let i = 0; i < segments.length - 1; i++) {
      const seg = segments[i];
      if (!current[seg] || typeof current[seg] !== "object") {
        current[seg] = {};
      }
      current = current[seg];
    }

    current[segments[segments.length - 1]] = value;
  }

  return result;
}

/**
 * Reads a namespace's message JSON file for a given locale and returns flattened keys.
 *
 * @param {string} locale - e.g. "en" or "zh-CN"
 * @param {string} namespace - e.g. "auth"
 * @returns {Record<string, string>} Flattened key-value map
 */
function loadNamespaceMessages(locale, namespace) {
  const filePath = path.join(MESSAGES_DIR, locale, `${namespace}.json`);
  if (!fs.existsSync(filePath)) {
    return {};
  }
  const raw = JSON.parse(fs.readFileSync(filePath, "utf-8"));
  return flattenMessages(raw);
}

/**
 * Returns all namespace names by listing JSON files in messages/en/.
 *
 * @returns {string[]}
 */
function getAllNamespaces() {
  const enDir = path.join(MESSAGES_DIR, "en");
  return fs
    .readdirSync(enDir)
    .filter((f) => f.endsWith(".json"))
    .map((f) => f.replace(/\.json$/, ""));
}

/**
 * Writes a flat key-value map back to a namespace's JSON file,
 * unflattening to nested structure and preserving consistent formatting.
 *
 * @param {string} locale
 * @param {string} namespace
 * @param {Record<string, string>} flatMap
 */
function writeNamespaceMessages(locale, namespace, flatMap) {
  const filePath = path.join(MESSAGES_DIR, locale, `${namespace}.json`);
  const nested = unflattenMessages(flatMap);
  fs.writeFileSync(filePath, JSON.stringify(nested, null, 2) + "\n", "utf-8");
}

module.exports = {
  MESSAGES_DIR,
  LOCALES,
  flattenMessages,
  unflattenMessages,
  loadNamespaceMessages,
  getAllNamespaces,
  writeNamespaceMessages,
};
