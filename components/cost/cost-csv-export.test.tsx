import { render, screen, fireEvent } from "@testing-library/react";
import { CostCsvExport, buildCostCsv } from "./cost-csv-export";

describe("buildCostCsv", () => {
  it("serializes entries with a header row and escapes special characters", () => {
    const csv = buildCostCsv([
      {
        id: "r1",
        date: "2026-04-10",
        category: "runtime",
        agent: "planner",
        amountUsd: 1.25,
      },
      {
        id: "r2",
        date: "2026-04-11",
        category: "compute, storage",
        agent: 'agent "primary"',
        amountUsd: 4.5,
      },
    ]);

    const lines = csv.split("\n");
    expect(lines[0]).toBe("Date,Category,Agent,Amount (USD)");
    expect(lines[1]).toBe("2026-04-10,runtime,planner,1.25");
    expect(lines[2]).toBe(
      '2026-04-11,"compute, storage","agent ""primary""",4.50',
    );
  });

  it("accepts a translated header row", () => {
    const csv = buildCostCsv(
      [
        {
          id: "r1",
          date: "2026-04-10",
          category: "runtime",
          agent: "planner",
          amountUsd: 1.25,
        },
      ],
      ["日期", "类别", "Agent", "金额 (USD)"],
    );

    const lines = csv.split("\n");
    expect(lines[0]).toBe("日期,类别,Agent,金额 (USD)");
    expect(lines[1]).toBe("2026-04-10,runtime,planner,1.25");
  });
});

describe("CostCsvExport", () => {
  it("is disabled when there is no data", () => {
    render(<CostCsvExport data={[]} />);
    expect(screen.getByRole("button", { name: /Export CSV/i })).toBeDisabled();
  });

  it("triggers a download when clicked", () => {
    const originalCreate = URL.createObjectURL;
    const originalRevoke = URL.revokeObjectURL;
    const createSpy = jest.fn(() => "blob:mock-url");
    const revokeSpy = jest.fn();
    URL.createObjectURL = createSpy as unknown as typeof URL.createObjectURL;
    URL.revokeObjectURL = revokeSpy as unknown as typeof URL.revokeObjectURL;

    const clickSpy = jest.fn();
    const anchorProto = Object.getPrototypeOf(
      document.createElement("a"),
    ) as HTMLAnchorElement;
    const origClick = anchorProto.click;
    anchorProto.click = clickSpy;

    try {
      render(
        <CostCsvExport
          data={[
            {
              id: "r1",
              date: "2026-04-10",
              category: "runtime",
              agent: "planner",
              amountUsd: 1.25,
            },
          ]}
          fileName="test.csv"
        />,
      );

      fireEvent.click(screen.getByRole("button", { name: /Export CSV/i }));
      expect(createSpy).toHaveBeenCalledTimes(1);
      expect(clickSpy).toHaveBeenCalledTimes(1);
      expect(revokeSpy).toHaveBeenCalledWith("blob:mock-url");
    } finally {
      anchorProto.click = origClick;
      URL.createObjectURL = originalCreate;
      URL.revokeObjectURL = originalRevoke;
    }
  });
});
