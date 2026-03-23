import { aggregateReviewResults } from "./aggregator.js";
import type {
  DeepReviewRequest,
  DeepReviewResponse,
  DimensionReviewResult,
  ReviewDimension,
  ReviewFinding,
} from "./types.js";

const DEFAULT_DIMENSIONS: ReviewDimension[] = [
  "logic",
  "security",
  "performance",
  "compliance",
];

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

function reviewLogic(request: DeepReviewRequest): DimensionReviewResult {
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
    status: "completed",
    findings,
    summary: findings.length === 0 ? "No logic issues detected." : `Found ${findings.length} logic issue(s).`,
  };
}

function reviewSecurity(request: DeepReviewRequest): DimensionReviewResult {
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
    status: "completed",
    findings,
    summary: findings.length === 0 ? "No security issues detected." : `Found ${findings.length} security issue(s).`,
  };
}

function reviewPerformance(request: DeepReviewRequest): DimensionReviewResult {
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
    status: "completed",
    findings,
    summary: findings.length === 0 ? "No performance issues detected." : `Found ${findings.length} performance issue(s).`,
  };
}

function reviewCompliance(request: DeepReviewRequest): DimensionReviewResult {
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
    status: "completed",
    findings,
    summary: findings.length === 0 ? "No compliance issues detected." : `Found ${findings.length} compliance issue(s).`,
  };
}

const reviewers: Record<ReviewDimension, (request: DeepReviewRequest) => DimensionReviewResult> = {
  logic: reviewLogic,
  security: reviewSecurity,
  performance: reviewPerformance,
  compliance: reviewCompliance,
};

export async function orchestrateDeepReview(request: DeepReviewRequest): Promise<DeepReviewResponse> {
  const dimensions = request.dimensions?.length ? request.dimensions : DEFAULT_DIMENSIONS;

  const settled = await Promise.allSettled(
    dimensions.map(async (dimension) => reviewers[dimension](request)),
  );

  const results: DimensionReviewResult[] = settled.map((result, index) => {
    const dimension = dimensions[index]!;
    if (result.status === "fulfilled") {
      return result.value;
    }

    return {
      dimension,
      status: "failed",
      findings: [],
      summary: `${dimension} review failed`,
      error: result.reason instanceof Error ? result.reason.message : String(result.reason),
    };
  });

  return aggregateReviewResults(results);
}
