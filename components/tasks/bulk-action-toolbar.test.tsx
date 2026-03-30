import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BulkActionToolbar } from "./bulk-action-toolbar";
import type { TeamMember } from "@/lib/dashboard/summary";

const members: TeamMember[] = [
  {
    id: "member-1",
    projectId: "project-1",
    name: "Alice",
    type: "human",
    typeLabel: "Human",
    role: "frontend",
    email: "",
    avatarUrl: "",
    skills: [],
    isActive: true,
    status: "active",
    createdAt: "2026-03-24T09:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
  },
  {
    id: "member-2",
    projectId: "project-1",
    name: "Inactive Bot",
    type: "agent",
    typeLabel: "Agent",
    role: "automation",
    email: "",
    avatarUrl: "",
    skills: [],
    isActive: false,
    status: "inactive",
    createdAt: "2026-03-24T09:00:00.000Z",
    lastActivityAt: null,
    workload: {
      assignedTasks: 0,
      inProgressTasks: 0,
      inReviewTasks: 0,
      activeAgentRuns: 0,
    },
  },
];

describe("BulkActionToolbar", () => {
  it("shows bulk actions and forwards status, assign, delete, and clear callbacks", async () => {
    const user = userEvent.setup();
    const onBulkStatusChange = jest.fn();
    const onBulkAssign = jest.fn();
    const onBulkDelete = jest.fn();
    const onClearSelection = jest.fn();

    render(
      <BulkActionToolbar
        selectedCount={2}
        members={members}
        onBulkStatusChange={onBulkStatusChange}
        onBulkAssign={onBulkAssign}
        onBulkDelete={onBulkDelete}
        onClearSelection={onClearSelection}
      />
    );

    expect(screen.getByText("2 selected")).toBeInTheDocument();

    const comboboxes = screen.getAllByRole("combobox");

    await user.click(comboboxes[0]);
    await user.click(screen.getByRole("option", { name: "done" }));
    expect(onBulkStatusChange).toHaveBeenCalledWith("done");

    await user.click(comboboxes[1]);
    await user.click(screen.getByRole("option", { name: "Alice" }));
    expect(onBulkAssign).toHaveBeenCalledWith("member-1", "human");
    expect(screen.queryByRole("option", { name: "Inactive Bot" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(screen.getByRole("button", { name: "Delete All" }));
    expect(onBulkDelete).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Clear" }));
    expect(onClearSelection).toHaveBeenCalledTimes(1);
  });
});
