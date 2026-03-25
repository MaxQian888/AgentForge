const createSprint = jest.fn().mockResolvedValue(undefined);
const updateSprint = jest.fn().mockResolvedValue(undefined);

jest.mock("@/lib/stores/sprint-store", () => ({
  useSprintStore: (selector: (state: { createSprint: typeof createSprint; updateSprint: typeof updateSprint }) => unknown) =>
    selector({ createSprint, updateSprint }),
}));

import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import { SprintManagement } from "./sprint-management";
import type { Sprint } from "@/lib/stores/sprint-store";

const planningSprint: Sprint = {
  id: "sprint-1",
  projectId: "project-1",
  name: "Sprint 1",
  startDate: "2026-03-25T00:00:00.000Z",
  endDate: "2026-03-31T00:00:00.000Z",
  status: "planning",
  totalBudgetUsd: 25,
  spentUsd: 4,
  createdAt: "2026-03-24T00:00:00.000Z",
};

describe("SprintManagement", () => {
  beforeEach(() => {
    createSprint.mockClear();
    updateSprint.mockClear();
  });

  it("shows the empty state when no sprints exist", () => {
    render(<SprintManagement projectId="project-1" sprints={[]} />);

    expect(
      screen.getByText("No sprints yet. Create the first sprint to begin tracking cycles."),
    ).toBeInTheDocument();
  });

  it("creates new sprints and updates sprint status", async () => {
    const user = userEvent.setup();
    render(<SprintManagement projectId="project-1" sprints={[planningSprint]} />);

    expect(screen.getByText("2026-03-25 - 2026-03-31")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Activate" }));
    expect(updateSprint).toHaveBeenCalledWith("project-1", "sprint-1", { status: "active" });

    await user.click(screen.getByRole("button", { name: "Create Sprint" }));
    await user.type(screen.getByLabelText("Name"), "Sprint 2");
    await user.type(screen.getByLabelText("Start date"), "2026-04-01");
    await user.type(screen.getByLabelText("End date"), "2026-04-07");
    await user.type(screen.getByLabelText("Budget (USD)"), "12.5");
    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() =>
      expect(createSprint).toHaveBeenCalledWith("project-1", {
        name: "Sprint 2",
        startDate: "2026-04-01T00:00:00.000Z",
        endDate: "2026-04-07T00:00:00.000Z",
        totalBudgetUsd: 12.5,
      }),
    );
  });
});
