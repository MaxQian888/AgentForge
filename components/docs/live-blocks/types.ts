"use client";

/**
 * Shared TypeScript types for live-artifact BlockNote blocks.
 *
 * Mirrors the Go-side `liveartifact.ProjectionResult` shape plus the
 * BlockNote-block reference props. BlockNote props are primitives, so
 * `target_ref` and `view_opts` are persisted as JSON strings inside the
 * block and parsed at render time.
 */

export type LiveArtifactKind =
  | "agent_run"
  | "cost_summary"
  | "review"
  | "task_group";

export type ProjectionStatus = "ok" | "not_found" | "forbidden" | "degraded";

// ---------------------------------------------------------------------------
// target_ref tagged union — what the block points at
// ---------------------------------------------------------------------------

export interface AgentRunTargetRef {
  kind: "agent_run";
  id: string;
}

export interface CostSummaryFilter {
  range_start: string;
  range_end: string;
  runtime?: string;
  provider?: string;
  member_id?: string;
  group_by?: "runtime" | "provider" | "member";
}

export interface CostSummaryTargetRef {
  kind: "cost_summary";
  filter: CostSummaryFilter;
}

export interface ReviewTargetRef {
  kind: "review";
  id: string;
}

export interface TaskGroupInlineFilter {
  status?: string[];
  assignee_id?: string;
  labels?: string[];
  sprint_id?: string;
  milestone_id?: string;
}

export interface TaskGroupTargetRef {
  kind: "task_group";
  filter: {
    saved_view_id?: string;
    inline?: TaskGroupInlineFilter;
  };
}

export type TargetRef =
  | AgentRunTargetRef
  | CostSummaryTargetRef
  | ReviewTargetRef
  | TaskGroupTargetRef;

// ---------------------------------------------------------------------------
// view_opts per kind
// ---------------------------------------------------------------------------

export interface AgentRunViewOpts {
  show_log_lines?: number;
}

export interface CostSummaryViewOpts {
  group_by?: "runtime" | "provider" | "member";
  top_n?: number;
}

export interface ReviewViewOpts {
  show_findings?: boolean;
}

export interface TaskGroupViewOpts {
  page_size?: number;
  columns?: string[];
}

export type ViewOpts =
  | AgentRunViewOpts
  | CostSummaryViewOpts
  | ReviewViewOpts
  | TaskGroupViewOpts;

// ---------------------------------------------------------------------------
// ProjectionResult — mirrors Go liveartifact.ProjectionResult
// ---------------------------------------------------------------------------

export interface BlockNoteBlock {
  id?: string;
  type: string;
  props?: Record<string, unknown>;
  content?: unknown;
  children?: BlockNoteBlock[];
}

export interface ProjectionResult {
  status: ProjectionStatus;
  projection?: BlockNoteBlock[];
  projected_at: string;
  ttl_hint_ms?: number;
  diagnostics?: string;
}

// ---------------------------------------------------------------------------
// Live-block reference (the BlockNote custom-block props, parsed)
// ---------------------------------------------------------------------------

export interface LiveArtifactBlockRef {
  id: string;
  live_kind: LiveArtifactKind;
  target_ref: TargetRef;
  view_opts: ViewOpts;
  last_rendered_at: string;
}

/** Parse a JSON string from a BlockNote prop, defaulting safely. */
export function safeParseJson<T>(raw: string, fallback: T): T {
  if (!raw || typeof raw !== "string") return fallback;
  try {
    const parsed = JSON.parse(raw);
    return parsed === null || parsed === undefined ? fallback : (parsed as T);
  } catch {
    return fallback;
  }
}

/** Stringify a JS value for storage in a BlockNote prop. */
export function serializeJson(value: unknown): string {
  try {
    return JSON.stringify(value ?? null);
  } catch {
    return "{}";
  }
}

/** Human-readable title for each live kind (used in chrome header). */
export function kindTitle(kind: string): string {
  switch (kind) {
    case "agent_run":
      return "Agent run";
    case "cost_summary":
      return "Cost summary";
    case "review":
      return "Review";
    case "task_group":
      return "Task group";
    default:
      return "Live artifact";
  }
}
