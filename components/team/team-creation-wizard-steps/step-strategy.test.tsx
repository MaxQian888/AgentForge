jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "wizard.strategyHint": "Choose a strategy",
      "wizard.strategy.sequential": "Sequential",
      "wizard.strategy.parallel": "Parallel",
      "wizard.strategy.hybrid": "Hybrid",
      "wizard.strategy.sequentialDesc": "One role after another",
      "wizard.strategy.parallelDesc": "Roles work at once",
      "wizard.strategy.hybridDesc": "Mix both",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StepStrategy } from "./step-strategy";

describe("StepStrategy", () => {
  it("renders available strategies and updates the selection", async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();

    render(<StepStrategy strategy="sequential" onChange={onChange} />);

    expect(screen.getByText("Choose a strategy")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Hybrid/i }));

    expect(onChange).toHaveBeenCalledWith("hybrid");
  });
});
