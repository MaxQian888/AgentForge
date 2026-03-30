jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string | number>,
  ) => {
    const map: Record<string, string> = {
      "wizard.title": "Create Team",
      "wizard.step.nameGoal": "Name & Goal",
      "wizard.step.selectRoles": "Select Roles",
      "wizard.step.assignAgents": "Assign Agents",
      "wizard.step.strategy": "Strategy",
      "wizard.step.budget": "Budget",
      "wizard.step.review": "Review",
      "wizard.previous": "Previous",
      "wizard.next": "Next",
      "wizard.launch": "Launch",
      "wizard.launching": "Launching",
    };
    if (key === "wizard.stepOf") {
      return `Step ${values?.current ?? 0} of ${values?.total ?? 0}`;
    }
    return map[key] ?? key;
  },
}));

const startTeamMock = jest.fn();

jest.mock("@/lib/stores/team-store", () => ({
  useTeamStore: (selector: (state: { startTeam: typeof startTeamMock }) => unknown) =>
    selector({ startTeam: startTeamMock }),
}));

jest.mock("@/components/ui/sheet", () => ({
  Sheet: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  SheetContent: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  SheetHeader: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  SheetTitle: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

jest.mock("./team-creation-wizard-steps/step-name", () => ({
  StepName: ({
    onChange,
  }: {
    onChange: (data: { name: string; description: string; objective: string }) => void;
  }) => (
    <button
      type="button"
      onClick={() =>
        onChange({
          name: "Release Squad",
          description: "Handles releases",
          objective: "Ship builds",
        })
      }
    >
      Fill Name Step
    </button>
  ),
}));

jest.mock("./team-creation-wizard-steps/step-roles", () => ({
  StepRoles: ({ onChange }: { onChange: (roleIds: string[]) => void }) => (
    <button type="button" onClick={() => onChange(["frontend", "reviewer"])}>
      Choose Roles
    </button>
  ),
}));

jest.mock("./team-creation-wizard-steps/step-agents", () => ({
  StepAgents: ({ onChange }: { onChange: (assignments: Record<string, string>) => void }) => (
    <button type="button" onClick={() => onChange({ frontend: "agent-1" })}>
      Assign Agents
    </button>
  ),
}));

jest.mock("./team-creation-wizard-steps/step-strategy", () => ({
  StepStrategy: ({ onChange }: { onChange: (strategy: "hybrid") => void }) => (
    <button type="button" onClick={() => onChange("hybrid")}>
      Choose Strategy
    </button>
  ),
}));

jest.mock("./team-creation-wizard-steps/step-budget", () => ({
  StepBudget: ({
    onChange,
  }: {
    onChange: (data: { maxBudgetPerAgent: number; totalTeamBudget: number; autoStopOnExceed: boolean }) => void;
  }) => (
    <button
      type="button"
      onClick={() =>
        onChange({
          maxBudgetPerAgent: 10,
          totalTeamBudget: 50,
          autoStopOnExceed: true,
        })
      }
    >
      Set Budget
    </button>
  ),
}));

jest.mock("./team-creation-wizard-steps/step-review", () => ({
  StepReview: () => <div>Review Step</div>,
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeamCreationWizard } from "./team-creation-wizard";

describe("TeamCreationWizard", () => {
  beforeEach(() => {
    startTeamMock.mockReset();
    startTeamMock.mockResolvedValue(undefined);
  });

  it("advances through steps and launches teams with mapped strategy values", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(<TeamCreationWizard open onOpenChange={onOpenChange} />);

    expect(screen.getByText("Create Team")).toBeInTheDocument();
    expect(screen.getByText("Step 1 of 6")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Fill Name Step" }));
    expect(screen.getByRole("button", { name: "Next" })).not.toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next" }));

    await user.click(screen.getByRole("button", { name: "Choose Roles" }));
    await user.click(screen.getByRole("button", { name: "Next" }));

    await user.click(screen.getByText("Assign Agents", { selector: "button" }));
    await user.click(screen.getByRole("button", { name: "Next" }));

    await user.click(screen.getByRole("button", { name: "Choose Strategy" }));
    await user.click(screen.getByRole("button", { name: "Next" }));

    await user.click(screen.getByRole("button", { name: "Set Budget" }));
    await user.click(screen.getByRole("button", { name: "Next" }));

    expect(screen.getByText("Review Step")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Launch" }));

    await waitFor(() => {
      expect(startTeamMock).toHaveBeenCalledWith("", "", {
        strategy: "plan-code-review",
        totalBudgetUsd: 50,
      });
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
