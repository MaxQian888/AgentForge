jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string>,
  ) => {
    const map: Record<string, string> = {
      "detail.dispatchHistoryTitle": "Dispatch History",
      "detail.dispatchHistoryDescription": "Recent dispatch attempts",
      "detail.dispatchHistoryEmpty": "No dispatch attempts yet.",
      "detail.dispatchHint.started": "Started",
      "detail.dispatchHint.queued": "Queued",
      "detail.dispatchGuardrail.budget": "Budget Guardrail",
    };
    if (key === "detail.dispatchTrigger") {
      return `Triggered by ${values?.trigger ?? ""}`;
    }
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { DispatchHistoryPanel } from "./dispatch-history-panel";

describe("DispatchHistoryPanel", () => {
  it("renders the empty state when there are no attempts", () => {
    render(<DispatchHistoryPanel attempts={[]} />);

    expect(screen.getByText("Dispatch History")).toBeInTheDocument();
    expect(screen.getByText("No dispatch attempts yet.")).toBeInTheDocument();
  });

  it("renders attempts with outcome, trigger, guardrail, and reason", () => {
    render(
      <DispatchHistoryPanel
        attempts={[
          {
            id: "attempt-1",
            outcome: "queued",
            triggerSource: "manual",
            guardrailType: "budget",
            reason: "Pool is saturated",
            createdAt: "2026-03-30T10:00:00.000Z",
          },
        ] as never}
      />,
    );

    expect(screen.getByText("Queued")).toBeInTheDocument();
    expect(screen.getByText("Triggered by manual")).toBeInTheDocument();
    expect(screen.getByText("Budget Guardrail")).toBeInTheDocument();
    expect(screen.getByText("Pool is saturated")).toBeInTheDocument();
  });
});
