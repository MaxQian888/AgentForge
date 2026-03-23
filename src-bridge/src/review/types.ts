export type ReviewDimension =
  | "logic"
  | "security"
  | "performance"
  | "compliance";

export type ReviewSeverity = "critical" | "high" | "medium" | "low";
export type ReviewRecommendation = "approve" | "request_changes" | "reject";
export type DimensionStatus = "completed" | "failed";

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
  dimensions?: ReviewDimension[];
}

export interface DimensionReviewResult {
  dimension: ReviewDimension;
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
  dimension_results: DimensionReviewResult[];
}
