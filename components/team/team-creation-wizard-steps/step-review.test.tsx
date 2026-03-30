jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string | number>,
  ) => {
    const map: Record<string, string> = {
      "wizard.reviewHint": "Review the setup before launch",
      "wizard.nameLabel": "Name",
      "wizard.descriptionLabel": "Description",
      "wizard.objectiveLabel": "Objective",
      "wizard.rolesLabel": "Roles",
      "wizard.agentsLabel": "Agents",
      "wizard.strategyLabel": "Strategy",
      "wizard.budgetLabel": "Budget",
      "wizard.noneSelected": "None selected",
      "wizard.strategy.parallel": "Parallel",
      "wizard.autoStopEnabled": "Auto stop enabled",
    };
    if (key === "wizard.manuallyAssigned") {
      return `${values?.count ?? 0} manually assigned`;
    }
    if (key === "wizard.autoAssigned") {
      return `${values?.count ?? 0} auto assigned`;
    }
    if (key === "wizard.budgetSummary") {
      return `$${values?.perAgent ?? "0"} per agent / $${values?.total ?? "0"} total`;
    }
    return map[key] ?? key;
  },
}));

const roleStoreState = {
  roles: [] as Array<Record<string, unknown>>,
};

jest.mock("@/lib/stores/role-store", () => ({
  useRoleStore: (selector: (state: typeof roleStoreState) => unknown) =>
    selector(roleStoreState),
}));

import { render, screen } from "@testing-library/react";
import { StepReview } from "./step-review";

describe("StepReview", () => {
  beforeEach(() => {
    roleStoreState.roles = [
      { metadata: { id: "frontend", name: "Frontend Developer" } },
      { metadata: { id: "reviewer", name: "Reviewer" } },
    ];
  });

  it("summarizes team setup choices and assignment counts", () => {
    render(
      <StepReview
        nameData={{
          name: "Release Squad",
          description: "Handles release workflow",
          objective: "Ship stable builds",
        }}
        selectedRoleIds={["frontend", "reviewer"]}
        agentAssignments={{ frontend: "agent-1", reviewer: "__auto__" }}
        strategy="parallel"
        budgetData={{
          maxBudgetPerAgent: 12.5,
          totalTeamBudget: 50,
          autoStopOnExceed: true,
        }}
      />,
    );

    expect(screen.getByText("Review the setup before launch")).toBeInTheDocument();
    expect(screen.getByText("Release Squad")).toBeInTheDocument();
    expect(screen.getByText("Handles release workflow")).toBeInTheDocument();
    expect(screen.getByText("Ship stable builds")).toBeInTheDocument();
    expect(screen.getByText("Frontend Developer, Reviewer")).toBeInTheDocument();
    expect(screen.getByText("1 manually assigned, 1 auto assigned")).toBeInTheDocument();
    expect(screen.getByText("Parallel")).toBeInTheDocument();
    expect(screen.getByText("$12.50 per agent / $50.00 total")).toBeInTheDocument();
    expect(screen.getByText("Auto stop enabled")).toBeInTheDocument();
  });
});
