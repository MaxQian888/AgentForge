import { render, screen, fireEvent } from "@testing-library/react";
import {
  CostBreakdownTable,
  type CostBreakdownEntry,
} from "./cost-breakdown-table";

function buildEntries(count: number): CostBreakdownEntry[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `row-${i}`,
    date: `2026-04-${String(i + 1).padStart(2, "0")}`,
    category: "runtime",
    agent: `agent-${i}`,
    amountUsd: i + 1,
  }));
}

describe("CostBreakdownTable", () => {
  it("sorts entries by date descending and paginates", () => {
    render(<CostBreakdownTable data={buildEntries(12)} pageSize={5} />);

    // First page should show 5 rows, with the latest date at top.
    expect(screen.getByText("2026-04-12")).toBeInTheDocument();
    expect(screen.getByText("2026-04-08")).toBeInTheDocument();
    expect(screen.queryByText("2026-04-07")).not.toBeInTheDocument();
    expect(screen.getByText(/Page 1 of 3/)).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Next page"));
    expect(screen.getByText("2026-04-07")).toBeInTheDocument();
    expect(screen.queryByText("2026-04-12")).not.toBeInTheDocument();
    expect(screen.getByText(/Page 2 of 3/)).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Previous page"));
    expect(screen.getByText(/Page 1 of 3/)).toBeInTheDocument();
  });

  it("shows empty message when there are no entries", () => {
    render(<CostBreakdownTable data={[]} />);
    expect(screen.getByText("No cost entries available.")).toBeInTheDocument();
  });
});
