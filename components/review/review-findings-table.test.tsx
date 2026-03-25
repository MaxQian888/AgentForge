import { render, screen } from "@testing-library/react";
import { ReviewFindingsTable } from "./review-findings-table";

describe("ReviewFindingsTable", () => {
  it("shows an empty-state message when there are no findings", () => {
    render(<ReviewFindingsTable findings={[]} />);

    expect(screen.getByText("No findings reported.")).toBeInTheDocument();
  });

  it("renders findings with file locations and fallback suggestion text", () => {
    render(
      <ReviewFindingsTable
        findings={[
          {
            severity: "critical",
            category: "security",
            subcategory: "auth",
            file: "src/auth.ts",
            line: 22,
            message: "Token validation is missing.",
            suggestion: "Validate the session token before access.",
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
    expect(screen.getByText("security / auth")).toBeInTheDocument();
    expect(screen.getByText("src/auth.ts:22")).toBeInTheDocument();
    expect(screen.getByText("Token validation is missing.")).toBeInTheDocument();
    expect(
      screen.getByText("Validate the session token before access."),
    ).toBeInTheDocument();
    expect(screen.getByText("Formatting is inconsistent.")).toBeInTheDocument();
    expect(screen.getAllByText("-")).toHaveLength(2);
  });
});
