import { render, screen } from "@testing-library/react";
import { EmployeeRunRow } from "./employee-run-row";

describe("EmployeeRunRow", () => {
  it("renders kind badge, status badge, name as a link, and formatted duration", () => {
    render(
      <EmployeeRunRow
        row={{
          kind: "workflow",
          id: "exec-1",
          name: "echo-flow",
          status: "completed",
          startedAt: "2026-04-20T10:00:00Z",
          completedAt: "2026-04-20T10:00:45Z",
          durationMs: 45000,
          refUrl: "/workflow/runs/exec-1",
        }}
      />,
    );
    expect(screen.getByText("workflow")).toBeInTheDocument();
    expect(screen.getByText("completed")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: /echo-flow/ });
    expect(link).toHaveAttribute("href", "/workflow/runs/exec-1");
    expect(screen.getByText(/45\.0s|45000ms|0:45/)).toBeInTheDocument();
  });

  it("renders an em-dash for missing started_at and missing duration", () => {
    render(
      <EmployeeRunRow
        row={{
          kind: "agent",
          id: "run-1",
          name: "code-reviewer",
          status: "running",
          refUrl: "/agents?run=run-1",
        }}
      />,
    );
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
    expect(screen.getByText("agent")).toBeInTheDocument();
    expect(screen.getByText("running")).toBeInTheDocument();
  });
});
