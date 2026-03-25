import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import type { ReviewFinding } from "../review/types.js";

export type ReviewFindingInput = Omit<ReviewFinding, "sources"> & {
  sources?: string[];
};

export type ReviewResultInput = {
  pluginId: string;
  summary: string;
  findings: ReviewFindingInput[];
};

type FindingsPayload = {
  format: "findings/v1";
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

  return {
    ...input,
    sources: Array.from(sources),
  };
}

export function createReviewResult(input: ReviewResultInput): CallToolResult {
  const findings = input.findings.map((finding) =>
    createReviewFinding(finding, { pluginId: input.pluginId }),
  );
  const structuredContent: FindingsPayload = {
    format: "findings/v1",
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
