import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditTeamDialog } from "./edit-team-dialog";

const team = {
  id: "team-1",
  name: "Review Squad",
  totalBudget: 10,
  totalSpent: 6,
} as never;

describe("EditTeamDialog", () => {
  it("blocks underspending budgets and saves trimmed updates", async () => {
    const user = userEvent.setup();
    const onSave = jest.fn().mockResolvedValue(undefined);
    const onClose = jest.fn();

    render(
      <EditTeamDialog
        open
        team={team}
        onSave={onSave}
        onClose={onClose}
      />,
    );

    const budgetInput = screen.getByLabelText("Budget (USD)");
    await user.clear(budgetInput);
    await user.type(budgetInput, "5");
    expect(screen.getByText("Budget cannot be less than already spent ($6.00).")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeDisabled();

    await user.clear(screen.getByLabelText("Team Name"));
    await user.type(screen.getByLabelText("Team Name"), "  Release Squad  ");
    await user.clear(budgetInput);
    await user.type(budgetInput, "12");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith({
        name: "Release Squad",
        totalBudgetUsd: 12,
      });
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("closes without saving when cancelled", async () => {
    const user = userEvent.setup();
    const onClose = jest.fn();

    render(
      <EditTeamDialog
        open
        team={team}
        onSave={jest.fn().mockResolvedValue(undefined)}
        onClose={onClose}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalled();
  });
});
