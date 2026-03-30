jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "wizard.budgetPerAgent": "Budget Per Agent",
      "wizard.budgetTotal": "Total Team Budget",
      "wizard.autoStop": "Auto Stop",
      "wizard.autoStopDesc": "Stop when the team exceeds budget",
    };
    return map[key] ?? key;
  },
}));

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StepBudget } from "./step-budget";

describe("StepBudget", () => {
  it("updates numeric budgets and the auto-stop toggle", async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();

    render(
      <StepBudget
        data={{ maxBudgetPerAgent: 0, totalTeamBudget: 0, autoStopOnExceed: true }}
        onChange={onChange}
      />,
    );

    fireEvent.change(screen.getByLabelText("Budget Per Agent"), {
      target: { value: "12.5" },
    });
    fireEvent.change(screen.getByLabelText("Total Team Budget"), {
      target: { value: "50" },
    });
    await user.click(screen.getByRole("switch"));

    expect(onChange).toHaveBeenCalledWith({
      maxBudgetPerAgent: 12.5,
      totalTeamBudget: 0,
      autoStopOnExceed: true,
    });
    expect(onChange).toHaveBeenCalledWith({
      maxBudgetPerAgent: 0,
      totalTeamBudget: 50,
      autoStopOnExceed: true,
    });
    expect(onChange).toHaveBeenLastCalledWith({
      maxBudgetPerAgent: 0,
      totalTeamBudget: 0,
      autoStopOnExceed: false,
    });
  });
});
