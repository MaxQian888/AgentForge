import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import type { ReviewFinding } from "../review/types.js";

export type ReviewFindingInput = Omit<ReviewFinding, "sources" | "suggested_patch"> & {
  sources?: string[];
  suggestedPatch?: string;
};

export type ReviewResultInput = {
  pluginId: string;
  summary: string;
  findings: ReviewFindingInput[];
};

type FindingsPayload = {
  format: "findings/v1" | "findings/v2";
  summary: string;
  findings: ReviewFinding[];
};

export function createReviewFinding(
  input: ReviewFindingInput,
  options: { pluginId?: string } = {},
): ReviewFinding {
  const sources = new Set(input.sources ?? []);
  if (options.pluginId) {
    sources.add(options.pluginId);
  }

  const { suggestedPatch, ...rest } = input;
  return {
    ...rest,
    sources: Array.from(sources),
    suggested_patch: suggestedPatch ?? null,
  };
}

export function createReviewResult(input: ReviewResultInput): CallToolResult {
  const findings = input.findings.map((finding) =>
    createReviewFinding(finding, { pluginId: input.pluginId }),
  );

  const hasPatch = findings.some(
    (f) => f.suggested_patch != null && f.suggested_patch !== "",
  );
  const format = hasPatch ? "findings/v2" : "findings/v1";

  // On v2 envelope, normalize missing patches to null.
  if (format === "findings/v2") {
    for (const f of findings) {
      if (f.suggested_patch === undefined) {
        f.suggested_patch = null;
      }
    }
  }

  const structuredContent: FindingsPayload = {
    format,
    summary: input.summary,
    findings,
  };

  return {
    content: [
      {
        type: "text",
        text: input.summary,
      },
    ],
    structuredContent,
  };
}
