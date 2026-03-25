import { MCPClientHub } from "../mcp/client-hub.js";
import type { MCPToolCallResult } from "../mcp/types.js";
import { aggregateReviewResults } from "./aggregator.js";
import type {
  DeepReviewRequest,
  DeepReviewResponse,
  ReviewDimension,
  ReviewExecutionResult,
  ReviewFinding,
  ReviewPluginExecution,
} from "./types.js";

const DEFAULT_DIMENSIONS: ReviewDimension[] = [
  "logic",
  "security",
  "performance",
  "compliance",
];

export interface DeepReviewOrchestratorOptions {
  executeReviewPlugin?: (
    plugin: ReviewPluginExecution,
    request: DeepReviewRequest,
  ) => Promise<ReviewExecutionResult>;
}

function buildFinding(
  dimension: ReviewDimension,
  severity: ReviewFinding["severity"],
  message: string,
  suggestion: string,
): ReviewFinding {
  return {
    category: dimension,
    severity,
    message,
    suggestion,
  };
}

function reviewLogic(request: DeepReviewRequest): ReviewExecutionResult {
  const haystack = `${request.title ?? ""}\n${request.description ?? ""}\n${request.diff ?? ""}`;
  const findings: ReviewFinding[] = [];

  if (/TODO|FIXME/i.test(haystack)) {
    findings.push(
      buildFinding(
        "logic",
        "medium",
        "Change includes TODO/FIXME markers that suggest unfinished logic.",
        "Replace TODO/FIXME placeholders with implemented logic or remove them before merge.",
      ),
    );
  }

  if (/eval\(/i.test(haystack)) {
    findings.push(
      buildFinding(
        "logic",
        "medium",
        "Dynamic evaluation introduces hard-to-reason execution paths.",
        "Replace eval-style execution with explicit control flow.",
      ),
    );
  }

  return {
    dimension: "logic",
    source_type: "builtin",
    status: "completed",
    findings,
    summary: findings.length === 0 ? "No logic issues detected." : `Found ${findings.length} logic issue(s).`,
  };
}

function reviewSecurity(request: DeepReviewRequest): ReviewExecutionResult {
  const haystack = `${request.title ?? ""}\n${request.description ?? ""}\n${request.diff ?? ""}`;
  const findings: ReviewFinding[] = [];

  if (/eval\(/i.test(haystack)) {
    findings.push(
      buildFinding(
        "security",
        "high",
        "Use of eval() creates a code-injection risk.",
        "Remove eval() and use safe parsing or explicit dispatch.",
      ),
    );
  }

  if (/(API_TOKEN|SECRET|PASSWORD|PRIVATE_KEY)/i.test(haystack)) {
    findings.push(
      buildFinding(
        "security",
        "high",
        "Potential secret or credential exposure detected in review context.",
        "Move secrets to secure configuration and remove them from code or diff content.",
      ),
    );
  }

  return {
    dimension: "security",
    source_type: "builtin",
    status: "completed",
    findings,
    summary: findings.length === 0 ? "No security issues detected." : `Found ${findings.length} security issue(s).`,
  };
}

function reviewPerformance(request: DeepReviewRequest): ReviewExecutionResult {
  const haystack = request.diff ?? "";
  const findings: ReviewFinding[] = [];

  if (/for\s*\(.*await|await\s+.*forEach/i.test(haystack)) {
    findings.push(
      buildFinding(
        "performance",
        "medium",
        "Potential serial async loop detected.",
        "Consider batching work or using Promise.all where concurrency is safe.",
      ),
    );
  }

  if (/SELECT \*/i.test(haystack)) {
    findings.push(
      buildFinding(
        "performance",
        "medium",
        "Broad database query detected.",
        "Limit selected columns and verify indexes for the reviewed query path.",
      ),
    );
  }

  return {
    dimension: "performance",
    source_type: "builtin",
    status: "completed",
    findings,
    summary: findings.length === 0 ? "No performance issues detected." : `Found ${findings.length} performance issue(s).`,
  };
}

function reviewCompliance(request: DeepReviewRequest): ReviewExecutionResult {
  const haystack = `${request.title ?? ""}\n${request.description ?? ""}\n${request.diff ?? ""}`;
  const findings: ReviewFinding[] = [];

  if (/console\.log/i.test(haystack)) {
    findings.push(
      buildFinding(
        "compliance",
        "low",
        "Debug logging appears in the reviewed change.",
        "Remove ad-hoc console logging or replace it with the project logging pattern.",
      ),
    );
  }

  return {
    dimension: "compliance",
    source_type: "builtin",
    status: "completed",
    findings,
    summary: findings.length === 0 ? "No compliance issues detected." : `Found ${findings.length} compliance issue(s).`,
  };
}

const reviewers: Record<ReviewDimension, (request: DeepReviewRequest) => ReviewExecutionResult> = {
  logic: reviewLogic,
  security: reviewSecurity,
  performance: reviewPerformance,
  compliance: reviewCompliance,
};

export function createDeepReviewOrchestrator(options: DeepReviewOrchestratorOptions = {}) {
  const executeReviewPlugin =
    options.executeReviewPlugin ?? (async (plugin: ReviewPluginExecution, request: DeepReviewRequest) =>
      executeMcpReviewPlugin(plugin, request));

  return async function runDeepReview(request: DeepReviewRequest): Promise<DeepReviewResponse> {
    const dimensions = request.dimensions?.length ? request.dimensions : DEFAULT_DIMENSIONS;
    const builtInSettled = await Promise.allSettled(
      dimensions.map(async (dimension) => reviewers[dimension](request)),
    );

    const builtInResults: ReviewExecutionResult[] = builtInSettled.map((result, index) => {
      const dimension = dimensions[index]!;
      if (result.status === "fulfilled") {
        return result.value;
      }

      return {
        dimension,
        source_type: "builtin",
        status: "failed",
        findings: [],
        summary: `${dimension} review failed`,
        error: result.reason instanceof Error ? result.reason.message : String(result.reason),
      };
    });

    const plugins = request.review_plugins ?? [];
    const pluginSettled = await Promise.allSettled(
      plugins.map(async (plugin) => executeReviewPlugin(plugin, request)),
    );

    const pluginResults: ReviewExecutionResult[] = pluginSettled.map((result, index) => {
      const plugin = plugins[index]!;
      if (result.status === "fulfilled") {
        return {
          ...result.value,
          dimension: result.value.dimension || plugin.plugin_id,
          source_type: "plugin",
          plugin_id: result.value.plugin_id ?? plugin.plugin_id,
          display_name: result.value.display_name ?? plugin.name,
        };
      }

      return {
        dimension: plugin.plugin_id,
        source_type: "plugin",
        plugin_id: plugin.plugin_id,
        display_name: plugin.name,
        status: "failed",
        findings: [],
        summary: `${plugin.plugin_id} review failed`,
        error: result.reason instanceof Error ? result.reason.message : String(result.reason),
      };
    });

    return aggregateReviewResults([...builtInResults, ...pluginResults]);
  };
}

async function executeMcpReviewPlugin(
  plugin: ReviewPluginExecution,
  request: DeepReviewRequest,
): Promise<ReviewExecutionResult> {
  if (!plugin.entrypoint) {
    throw new Error(`review plugin ${plugin.plugin_id} is missing an entrypoint`);
  }
  if (!plugin.transport) {
    throw new Error(`review plugin ${plugin.plugin_id} is missing a transport`);
  }

  const hub = new MCPClientHub();
  try {
    await hub.connectServer(plugin.plugin_id, {
      pluginId: plugin.plugin_id,
      transport: plugin.transport,
      command: plugin.command,
      args: plugin.args,
      url: plugin.url,
    });
    const result = await hub.callTool(plugin.plugin_id, plugin.entrypoint, buildPluginInput(plugin, request));
    return parseReviewPluginResult(plugin, result);
  } finally {
    await hub.dispose();
  }
}

function buildPluginInput(plugin: ReviewPluginExecution, request: DeepReviewRequest): Record<string, unknown> {
  return {
    review_id: request.review_id,
    task_id: request.task_id,
    pr_url: request.pr_url,
    pr_number: request.pr_number,
    title: request.title,
    description: request.description,
    diff: request.diff,
    trigger_event: request.trigger_event,
    changed_files: request.changed_files,
    dimensions: request.dimensions,
    plugin_id: plugin.plugin_id,
  };
}

function parseReviewPluginResult(
  plugin: ReviewPluginExecution,
  result: MCPToolCallResult,
): ReviewExecutionResult {
  if (result.isError) {
    throw new Error(`review plugin ${plugin.plugin_id} returned an MCP error`);
  }

  const payload = normalizePluginPayload(result);
  const findings = normalizePluginFindings(payload.findings, plugin.plugin_id);

  return {
    dimension: plugin.plugin_id,
    source_type: "plugin",
    plugin_id: plugin.plugin_id,
    display_name: plugin.name,
    status: "completed",
    findings,
    summary:
      typeof payload.summary === "string" && payload.summary.trim().length > 0
        ? payload.summary
        : findings.length === 0
          ? `${plugin.name} reported no findings.`
          : `${plugin.name} reported ${findings.length} finding(s).`,
  };
}

function normalizePluginPayload(result: MCPToolCallResult): Record<string, unknown> {
  if (result.structuredContent && typeof result.structuredContent === "object") {
    return result.structuredContent as Record<string, unknown>;
  }

  const firstText = result.content.find((item) => typeof item.text === "string" && item.text.trim().length > 0)?.text;
  if (!firstText) {
    return {};
  }
  try {
    return JSON.parse(firstText) as Record<string, unknown>;
  } catch {
    return { summary: firstText, findings: [] };
  }
}

function normalizePluginFindings(raw: unknown, pluginId: string): ReviewFinding[] {
  if (!Array.isArray(raw)) {
    return [];
  }

  return raw.flatMap((item) => {
    if (!item || typeof item !== "object") {
      return [];
    }
    const finding = item as Record<string, unknown>;
    const severity = normalizeSeverity(finding.severity);
    const category = typeof finding.category === "string" && finding.category.trim() ? finding.category : pluginId;
    const message = typeof finding.message === "string" ? finding.message.trim() : "";
    if (!message) {
      return [];
    }
    return [{
      category,
      subcategory: typeof finding.subcategory === "string" ? finding.subcategory : undefined,
      severity,
      file: typeof finding.file === "string" ? finding.file : undefined,
      line: typeof finding.line === "number" ? finding.line : undefined,
      message,
      suggestion: typeof finding.suggestion === "string" ? finding.suggestion : undefined,
      cwe: typeof finding.cwe === "string" ? finding.cwe : undefined,
    }];
  });
}

function normalizeSeverity(value: unknown): ReviewFinding["severity"] {
  switch (value) {
    case "critical":
    case "high":
    case "medium":
    case "low":
      return value;
    default:
      return "medium";
  }
}

export const orchestrateDeepReview = createDeepReviewOrchestrator();
