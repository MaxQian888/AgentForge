export type ReviewDimension =
  | "logic"
  | "security"
  | "performance"
  | "compliance";

export type ReviewSeverity = "critical" | "high" | "medium" | "low";
export type ReviewRecommendation = "approve" | "request_changes" | "reject";
export type DimensionStatus = "completed" | "failed";
export type ReviewExecutionSourceType = "builtin" | "plugin";

export interface ReviewFinding {
  category: string;
  subcategory?: string;
  severity: ReviewSeverity;
  file?: string;
  line?: number;
  message: string;
  suggestion?: string;
  cwe?: string;
  sources?: string[];
}

export interface DeepReviewRequest {
  review_id: string;
  task_id: string;
  pr_url: string;
  pr_number?: number;
  title?: string;
  description?: string;
  diff?: string;
  trigger_event?: string;
  changed_files?: string[];
  dimensions?: ReviewDimension[];
  review_plugins?: ReviewPluginExecution[];
}

export interface ReviewPluginExecution {
  plugin_id: string;
  name: string;
  entrypoint?: string;
  source_type?: string;
  transport?: "stdio" | "http";
  command?: string;
  args?: string[];
  url?: string;
  events?: string[];
  file_patterns?: string[];
  output_format?: string;
}

export interface ReviewExecutionResult {
  dimension: string;
  source_type?: ReviewExecutionSourceType;
  plugin_id?: string;
  display_name?: string;
  status: DimensionStatus;
  findings: ReviewFinding[];
  summary: string;
  error?: string;
}

export interface DeepReviewResponse {
  risk_level: ReviewSeverity;
  findings: ReviewFinding[];
  summary: string;
  recommendation: ReviewRecommendation;
  cost_usd: number;
  dimension_results: ReviewExecutionResult[];
}
