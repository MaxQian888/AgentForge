#!/usr/bin/env bun
/**
 * Role Manifest Validator
 *
 * Validates a role manifest JSON/YAML file against the agentforge/v1 schema.
 * Usage: bun run skills/role-creator/scripts/verify-manifest.ts <path-to-manifest>
 */

import * as fs from "node:fs";
import * as path from "node:path";
import * as YAML from "yaml";

interface ValidationIssue {
  field: string;
  message: string;
  severity: "error" | "warning";
}

/**
 * Get a value from manifest, checking both camelCase and snake_case keys.
 * The Go backend YAML parser uses snake_case tags, while JSON API uses camelCase.
 */
function getManifestValue<T>(
  obj: Record<string, unknown> | undefined,
  camelKey: string,
  snakeKey: string,
): T | undefined {
  if (!obj || typeof obj !== "object") return undefined;
  return (obj[camelKey] ?? obj[snakeKey]) as T | undefined;
}

function validateRoleManifest(manifest: Record<string, unknown>): ValidationIssue[] {
  const issues: ValidationIssue[] = [];

  // Top-level required fields
  if (manifest.apiVersion !== "agentforge/v1") {
    issues.push({
      field: "apiVersion",
      message: `Expected "agentforge/v1", got "${manifest.apiVersion}"`,
      severity: "error",
    });
  }
  if (manifest.kind !== "Role") {
    issues.push({
      field: "kind",
      message: `Expected "Role", got "${manifest.kind}"`,
      severity: "error",
    });
  }

  // Metadata
  const metadata = manifest.metadata as Record<string, unknown> | undefined;
  if (!metadata || typeof metadata !== "object") {
    issues.push({ field: "metadata", message: "metadata is required", severity: "error" });
  } else {
    const id = getManifestValue<string>(metadata, "id", "id");
    const name = getManifestValue<string>(metadata, "name", "name");
    const version = getManifestValue<string>(metadata, "version", "version");
    const description = getManifestValue<string>(metadata, "description", "description");

    if (!id || typeof id !== "string") {
      issues.push({ field: "metadata.id", message: "metadata.id is required and must be a string", severity: "error" });
    } else if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(id)) {
      issues.push({
        field: "metadata.id",
        message: "metadata.id must be kebab-case (e.g. 'backend-engineer')",
        severity: "error",
      });
    }
    if (!name || typeof name !== "string") {
      issues.push({ field: "metadata.name", message: "metadata.name is required", severity: "error" });
    }
    if (!version || typeof version !== "string") {
      issues.push({ field: "metadata.version", message: "metadata.version is required", severity: "error" });
    }
    if (!description || typeof description !== "string") {
      issues.push({ field: "metadata.description", message: "metadata.description is recommended", severity: "warning" });
    }
  }

  // Identity
  const identity = manifest.identity as Record<string, unknown> | undefined;
  if (!identity || typeof identity !== "object") {
    issues.push({ field: "identity", message: "identity is required", severity: "error" });
  } else {
    const systemPrompt = getManifestValue<string>(identity, "systemPrompt", "system_prompt");
    const topLevelSystemPrompt = getManifestValue<string>(manifest, "systemPrompt", "system_prompt");
    const role = getManifestValue<string>(identity, "role", "role");

    // Go backend allows system_prompt at top level (YAML tag matching)
    const effectiveSystemPrompt = systemPrompt ?? topLevelSystemPrompt;
    if (!effectiveSystemPrompt || typeof effectiveSystemPrompt !== "string") {
      issues.push({ field: "identity.systemPrompt", message: "identity.systemPrompt (or top-level system_prompt) is required", severity: "error" });
    }
    if (!role || typeof role !== "string") {
      issues.push({ field: "identity.role", message: "identity.role is recommended", severity: "warning" });
    }
  }

  // Capabilities
  const capabilities = manifest.capabilities as Record<string, unknown> | undefined;
  if (!capabilities || typeof capabilities !== "object") {
    issues.push({ field: "capabilities", message: "capabilities is required", severity: "error" });
  } else {
    const toolConfig = getManifestValue<Record<string, unknown>>(capabilities, "toolConfig", "tools");
    const allowedTools = getManifestValue<unknown[]>(capabilities, "allowedTools", "allowed_tools");
    if (!toolConfig && !allowedTools) {
      issues.push({ field: "capabilities.toolConfig", message: "capabilities.toolConfig or capabilities.allowedTools is required", severity: "error" });
    }
  }

  // Knowledge
  const knowledge = manifest.knowledge as Record<string, unknown> | undefined;
  if (!knowledge || typeof knowledge !== "object") {
    issues.push({ field: "knowledge", message: "knowledge is recommended for context", severity: "warning" });
  } else {
    const repositories = getManifestValue<unknown[]>(knowledge, "repositories", "repositories");
    const documents = getManifestValue<unknown[]>(knowledge, "documents", "documents");
    const patterns = getManifestValue<unknown[]>(knowledge, "patterns", "patterns");

    if (!Array.isArray(repositories)) {
      issues.push({ field: "knowledge.repositories", message: "knowledge.repositories must be an array", severity: "error" });
    }
    if (!Array.isArray(documents)) {
      issues.push({ field: "knowledge.documents", message: "knowledge.documents must be an array", severity: "error" });
    }
    if (!Array.isArray(patterns)) {
      issues.push({ field: "knowledge.patterns", message: "knowledge.patterns must be an array", severity: "error" });
    }
  }

  // Security
  const security = manifest.security as Record<string, unknown> | undefined;
  if (!security || typeof security !== "object") {
    issues.push({ field: "security", message: "security is required", severity: "error" });
  } else {
    const permissionMode = getManifestValue<string>(security, "permissionMode", "permission_mode");
    const allowedPaths = getManifestValue<unknown[]>(security, "allowedPaths", "allowed_paths");

    if (!permissionMode || typeof permissionMode !== "string") {
      issues.push({ field: "security.permissionMode", message: "security.permissionMode is required", severity: "error" });
    }
    if (!Array.isArray(allowedPaths)) {
      issues.push({ field: "security.allowedPaths", message: "security.allowedPaths is recommended for scoping", severity: "warning" });
    }
  }

  // Extends validation
  const extendsValue = manifest.extends as string | undefined;
  if (extendsValue && typeof extendsValue === "string") {
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(extendsValue)) {
      issues.push({
        field: "extends",
        message: "extends must be a valid kebab-case role ID",
        severity: "error",
      });
    }
  }

  // Overrides validation
  const overrides = manifest.overrides as Record<string, unknown> | undefined;
  if (overrides && typeof overrides !== "object") {
    issues.push({ field: "overrides", message: "overrides must be an object", severity: "error" });
  }

  return issues;
}

function formatIssues(issues: ValidationIssue[]): string {
  const errors = issues.filter((i) => i.severity === "error");
  const warnings = issues.filter((i) => i.severity === "warning");

  let output = "";
  if (errors.length > 0) {
    output += `Errors (${errors.length}):\n`;
    for (const issue of errors) {
      output += `  [${issue.field}] ${issue.message}\n`;
    }
  }
  if (warnings.length > 0) {
    output += `Warnings (${warnings.length}):\n`;
    for (const issue of warnings) {
      output += `  [${issue.field}] ${issue.message}\n`;
    }
  }
  return output || "No issues found.\n";
}

function main(): void {
  const args = process.argv.slice(2);
  if (args.length === 0) {
    console.error("Usage: bun run skills/role-creator/scripts/verify-manifest.ts <path-to-manifest>");
    process.exit(1);
  }

  const filePath = path.resolve(args[0]);
  if (!fs.existsSync(filePath)) {
    console.error(`File not found: ${filePath}`);
    process.exit(1);
  }

  const content = fs.readFileSync(filePath, "utf8");
  let manifest: Record<string, unknown>;

  try {
    if (filePath.endsWith(".yaml") || filePath.endsWith(".yml")) {
      manifest = YAML.parse(content) ?? {};
    } else {
      manifest = JSON.parse(content);
    }
  } catch (error) {
    console.error(`Failed to parse manifest: ${error instanceof Error ? error.message : String(error)}`);
    process.exit(1);
  }

  const issues = validateRoleManifest(manifest);
  console.log(formatIssues(issues));

  if (issues.some((i) => i.severity === "error")) {
    process.exit(1);
  }
}

main();
