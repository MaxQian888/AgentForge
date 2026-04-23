jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const messages: Record<string, string> = {
      noFindingsReported: "No findings reported.",
      findingSeverity: "Severity",
      findingCategory: "Category",
      findingSource: "Source",
      findingFileLine: "File:Line",
      findingMessage: "Message",
      findingSuggestion: "Suggestion",
      findingDecision: "Decision",
      findingActions: "Actions",
      actionApprove: "Approve",
      actionDismiss: "Dismiss",
      actionDefer: "Defer",
      actionShowPatch: "Show patch",
      decisionPending: "Pending",
      decisionApproved: "Approved",
      decisionDismissed: "Dismissed",
      decisionDeferred: "Deferred",
      decisionNeedsManualFix: "Needs Manual Fix",
    };
    return messages[key] ?? key;
  },
}));

jest.mock("react-diff-viewer-continued", () => ({
  __esModule: true,
  default: () => <div data-testid="diff-viewer" />,
  DiffMethod: { LINES: "lines" },
}));

const mockDecideFinding = jest.fn();
jest.mock("@/lib/stores/review-store", () => ({
  useReviewStore: <T,>(selector: (s: unknown) => T): T =>
    selector({ decideFinding: mockDecideFinding }),
}));

import { render, screen, fireEvent } from "@testing-library/react";
import { ReviewFindingsTable } from "./review-findings-table";

describe("ReviewFindingsTable", () => {
  beforeEach(() => {
    mockDecideFinding.mockClear();
  });

  it("shows an empty-state message when there are no findings", () => {
    render(<ReviewFindingsTable findings={[]} />);
    expect(screen.getByText("No findings reported.")).toBeInTheDocument();
  });

  it("renders findings with file locations and fallback suggestion text", () => {
    render(
      <ReviewFindingsTable
        findings={[
          {
            id: "f1",
            severity: "critical",
            category: "security",
            subcategory: "auth",
            file: "src/auth.ts",
            line: 22,
            message: "Token validation is missing.",
            suggestion: "Validate the session token before access.",
            sources: ["plugin.security"],
          },
          {
            severity: "low",
            category: "style",
            message: "Formatting is inconsistent.",
          },
        ]}
      />,
    );

    expect(screen.getByText("Severity")).toBeInTheDocument();
    expect(screen.getByText("Source")).toBeInTheDocument();
    expect(screen.getByText("security / auth")).toBeInTheDocument();
    expect(screen.getByText("plugin.security")).toBeInTheDocument();
    expect(screen.getByText("src/auth.ts:22")).toBeInTheDocument();
    expect(screen.getByText("Token validation is missing.")).toBeInTheDocument();
    expect(
      screen.getByText("Validate the session token before access."),
    ).toBeInTheDocument();
    expect(screen.getByText("Formatting is inconsistent.")).toBeInTheDocument();
  });

  it("approve button fires POST with approve action", () => {
    render(
      <ReviewFindingsTable
        findings={[
          {
            id: "f1",
            severity: "high",
            category: "logic",
            message: "Bug found.",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByTestId("approve-f1"));
    expect(mockDecideFinding).toHaveBeenCalledWith("f1", "approve");
  });

  it("dismiss button fires POST with dismiss action", () => {
    render(
      <ReviewFindingsTable
        findings={[
          {
            id: "f2",
            severity: "low",
            category: "style",
            message: "Indent.",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByTestId("dismiss-f2"));
    expect(mockDecideFinding).toHaveBeenCalledWith("f2", "dismiss");
  });

  it("defer button fires POST with defer action", () => {
    render(
      <ReviewFindingsTable
        findings={[
          {
            id: "f3",
            severity: "medium",
            category: "perf",
            message: "Slow loop.",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByTestId("defer-f3"));
    expect(mockDecideFinding).toHaveBeenCalledWith("f3", "defer");
  });

  it("show patch button is visible when suggestedPatch is present", () => {
    render(
      <ReviewFindingsTable
        findings={[
          {
            id: "f4",
            severity: "high",
            category: "logic",
            message: "Fix this.",
            suggestedPatch: "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b",
          },
        ]}
      />,
    );

    expect(screen.getByTestId("show-patch-f4")).toBeInTheDocument();
  });

  it("show patch button is hidden when suggestedPatch is absent", () => {
    render(
      <ReviewFindingsTable
        findings={[
          {
            id: "f5",
            severity: "low",
            category: "style",
            message: "Format.",
          },
        ]}
      />,
    );

    expect(screen.queryByTestId("show-patch-f5")).not.toBeInTheDocument();
  });
});
