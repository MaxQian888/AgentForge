import { render, screen } from "@testing-library/react";
import {
  getReviewRecommendationLabel,
  getReviewRiskLabel,
  getReviewStatusLabel,
  ReviewRecommendationBadge,
  ReviewRiskBadge,
  ReviewStatusBadge,
} from "./review-copy";

const t = (key: string) => {
  const map: Record<string, string> = {
    statusPendingHuman: "Pending Human",
    riskHigh: "High",
    recommendationRequestChanges: "Request Changes",
    statusUnknown: "Unknown Status",
    riskUnknown: "Unknown Risk",
    recommendationUnknown: "Unknown Recommendation",
  };
  return map[key] ?? key;
};

describe("review copy helpers", () => {
  it("maps known review labels through the translation keys", () => {
    expect(getReviewStatusLabel(t, "pending_human")).toBe("Pending Human");
    expect(getReviewRiskLabel(t, "high")).toBe("High");
    expect(getReviewRecommendationLabel(t, "request_changes")).toBe(
      "Request Changes",
    );
  });

  it("falls back to unknown labels for unmapped values", () => {
    expect(getReviewStatusLabel(t, "mystery")).toBe("Unknown Status");
    expect(getReviewRiskLabel(t, "mystery")).toBe("Unknown Risk");
    expect(getReviewRecommendationLabel(t, "mystery")).toBe(
      "Unknown Recommendation",
    );
  });

  it("renders badges with the expected styling and text", () => {
    render(
      <div>
        <ReviewStatusBadge status="pending_human" t={t} className="status" />
        <ReviewRiskBadge riskLevel="high" t={t} className="risk" />
        <ReviewRecommendationBadge
          recommendation="request_changes"
          t={t}
          className="recommendation"
        />
      </div>,
    );

    expect(screen.getByText("Pending Human")).toHaveClass("status");
    expect(screen.getByText("Pending Human")).toHaveClass("bg-amber-500/15");
    expect(screen.getByText("High")).toHaveClass("risk");
    expect(screen.getByText("High")).toHaveClass("bg-orange-500/15");
    expect(screen.getByText("Request Changes")).toHaveClass("recommendation");
    expect(screen.getByText("Request Changes")).toHaveClass("bg-amber-500/15");
  });
});
