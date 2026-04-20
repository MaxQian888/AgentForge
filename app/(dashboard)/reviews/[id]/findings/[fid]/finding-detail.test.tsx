jest.mock("next/navigation", () => ({
  useParams: () => ({ id: "review-1", fid: "finding-1" }),
}));

jest.mock("react-diff-viewer-continued", () => ({
  __esModule: true,
  default: () => <div data-testid="diff-viewer-inner" />,
  DiffMethod: { LINES: "lines" },
}));

const mockDecideFinding = jest.fn();
jest.mock("@/lib/stores/review-store", () => ({
  useReviewStore: <T,>(selector: (s: unknown) => T): T =>
    selector({
      decideFinding: mockDecideFinding,
      allReviews: [
        {
          id: "review-1",
          findings: [
            {
              id: "finding-1",
              category: "logic",
              severity: "high",
              message: "Null pointer dereference.",
              file: "src/handler.go",
              line: 42,
              suggestedPatch: "--- a/src/handler.go\n+++ b/src/handler.go\n@@ -42 +42 @@\n-x\n+y\n",
              decision: "pending",
              sources: ["plugin.lint"],
            },
          ],
        },
      ],
    }),
}));

import { render, screen } from "@testing-library/react";
import FindingDetailPage from "./page";

describe("FindingDetailPage", () => {
  it("renders metadata and diff panel when finding has suggestedPatch", () => {
    render(<FindingDetailPage />);

    expect(screen.getByText("Null pointer dereference.")).toBeInTheDocument();
    expect(screen.getByText("src/handler.go:42")).toBeInTheDocument();
    expect(screen.getByText("high")).toBeInTheDocument();
    expect(screen.getByTestId("diff-panel")).toBeInTheDocument();
  });

  it("renders empty fix-runs state", () => {
    render(<FindingDetailPage />);

    expect(screen.getByText("No fix runs yet")).toBeInTheDocument();
  });

  it("shows Approve/Dismiss/Defer buttons", () => {
    render(<FindingDetailPage />);

    expect(screen.getByText("Approve")).toBeInTheDocument();
    expect(screen.getByText("Dismiss")).toBeInTheDocument();
    expect(screen.getByText("Defer")).toBeInTheDocument();
  });
});
