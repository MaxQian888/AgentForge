import type {
  DeepReviewResponse,
  DimensionReviewResult,
  ReviewFinding,
  ReviewRecommendation,
  ReviewSeverity,
} from "./types.js";

const severityOrder: Record<ReviewSeverity, number> = {
  low: 1,
  medium: 2,
  high: 3,
  critical: 4,
};

function dedupeFindings(findings: Array<ReviewFinding & { source: string }>): ReviewFinding[] {
  const deduped = new Map<string, ReviewFinding>();

  for (const finding of findings) {
    const key = [
      finding.category,
      finding.file ?? "",
      finding.line ?? 0,
      finding.message,
    ].join("|");

    const existing = deduped.get(key);
    if (existing) {
      const mergedSources = new Set([...(existing.sources ?? []), finding.source]);
      existing.sources = Array.from(mergedSources);
      if (severityOrder[finding.severity] > severityOrder[existing.severity]) {
        existing.severity = finding.severity;
      }
      if (!existing.suggestion && finding.suggestion) {
        existing.suggestion = finding.suggestion;
      }
      continue;
    }

    deduped.set(key, {
      ...finding,
      sources: [finding.source],
    });
  }

  return Array.from(deduped.values());
}

function highestRisk(findings: ReviewFinding[]): ReviewSeverity {
  if (findings.length === 0) {
    return "low";
  }

  return findings.reduce<ReviewSeverity>((highest, finding) => {
    if (severityOrder[finding.severity] > severityOrder[highest]) {
      return finding.severity;
    }
    return highest;
  }, "low");
}

function recommendationFor(findings: ReviewFinding[], hadFailures: boolean): ReviewRecommendation {
  const risk = highestRisk(findings);
  if (risk === "critical") {
    return "reject";
  }
  if (hadFailures || findings.length > 0) {
    return "request_changes";
  }
  return "approve";
}

export function aggregateReviewResults(results: DimensionReviewResult[]): DeepReviewResponse {
  const allFindings = results.flatMap((result) =>
    result.findings.map((finding) => ({
      ...finding,
      source: result.dimension,
    })),
  );
  const dedupedFindings = dedupeFindings(allFindings);
  const failedDimensions = results.filter((result) => result.status === "failed");
  const risk = highestRisk(dedupedFindings);
  const recommendation = recommendationFor(dedupedFindings, failedDimensions.length > 0);

  const summaryParts = results.map((result) => {
    if (result.status === "failed") {
      return `${result.dimension} failed: ${result.error ?? result.summary}`;
    }
    return `${result.dimension}: ${result.summary}`;
  });

  return {
    risk_level: risk,
    findings: dedupedFindings,
    summary: summaryParts.join("; "),
    recommendation,
    cost_usd: Number((results.length * 0.05).toFixed(2)),
    dimension_results: results,
  };
}
