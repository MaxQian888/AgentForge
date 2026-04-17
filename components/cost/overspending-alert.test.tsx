import { render, screen, fireEvent } from "@testing-library/react";
import {
  OverspendingAlertBanner,
  deriveOverspendingAlerts,
} from "./overspending-alert";

describe("deriveOverspendingAlerts", () => {
  it("returns critical alerts when spend >= budget", () => {
    const alerts = deriveOverspendingAlerts([
      { id: "s-1", scope: "Sprint Alpha", spentUsd: 150, budgetUsd: 100 },
    ]);
    expect(alerts).toHaveLength(1);
    expect(alerts[0].severity).toBe("critical");
  });

  it("returns warning alerts when spend >= 80% of budget", () => {
    const alerts = deriveOverspendingAlerts([
      { id: "s-1", scope: "Sprint Alpha", spentUsd: 85, budgetUsd: 100 },
    ]);
    expect(alerts).toHaveLength(1);
    expect(alerts[0].severity).toBe("warning");
  });

  it("skips items with no budget configured", () => {
    const alerts = deriveOverspendingAlerts([
      { id: "s-1", scope: "Sprint Alpha", spentUsd: 100, budgetUsd: 0 },
    ]);
    expect(alerts).toHaveLength(0);
  });

  it("ignores items under 80% threshold", () => {
    const alerts = deriveOverspendingAlerts([
      { id: "s-1", scope: "Sprint Alpha", spentUsd: 50, budgetUsd: 100 },
    ]);
    expect(alerts).toHaveLength(0);
  });
});

describe("OverspendingAlertBanner", () => {
  it("renders nothing when there are no alerts", () => {
    const { container } = render(<OverspendingAlertBanner alerts={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders a critical banner with exceeded-budget copy", () => {
    render(
      <OverspendingAlertBanner
        alerts={[
          {
            id: "s-1",
            scope: "Sprint Alpha",
            severity: "critical",
            spentUsd: 150,
            budgetUsd: 100,
          },
        ]}
      />,
    );
    expect(
      screen.getByText("Sprint Alpha has exceeded budget"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("$150.00 spent of $100.00 (150%)"),
    ).toBeInTheDocument();
  });

  it("triggers onAction when Adjust budget is clicked", () => {
    const onAction = jest.fn();
    render(
      <OverspendingAlertBanner
        alerts={[
          {
            id: "s-1",
            scope: "Sprint Alpha",
            severity: "warning",
            spentUsd: 80,
            budgetUsd: 100,
          },
        ]}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByText("Adjust budget"));
    expect(onAction).toHaveBeenCalledWith(
      expect.objectContaining({ id: "s-1" }),
    );
  });
});
