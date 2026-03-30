jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "budget.title": "Budget",
      "budget.total": "Total",
      "budget.spent": "Spent",
      "budget.remaining": "Remaining",
      "budget.utilization": "Utilization",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { BudgetWidget } from "./budget-widget";

describe("BudgetWidget", () => {
  it("renders budget values and utilization progress", () => {
    const { container } = render(
      <BudgetWidget totalBudget={1000} spent={850} remaining={150} />,
    );

    expect(screen.getByText("$1,000.00")).toBeInTheDocument();
    expect(screen.getByText("$850.00")).toBeInTheDocument();
    expect(screen.getByText("$150.00")).toBeInTheDocument();
    expect(screen.getByText("85%")).toBeInTheDocument();
    expect(container.querySelector('[style="width: 85%;"]')).toHaveClass("bg-amber-500");
  });
});
