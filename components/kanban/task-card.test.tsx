import type { ReactNode } from "react";

type MockDraggableChildrenProps = {
  children: (
    provided: {
      innerRef: () => void;
      draggableProps: Record<string, unknown>;
      dragHandleProps: Record<string, unknown>;
    },
    snapshot: { isDragging: boolean },
  ) => ReactNode;
};

jest.mock("@hello-pangea/dnd", () => ({
  Draggable: ({ children }: MockDraggableChildrenProps) =>
    children(
      {
        innerRef: jest.fn(),
        draggableProps: {},
        dragHandleProps: {},
      },
      { isDragging: false }
    ),
}));

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TaskCard } from "./task-card";

describe("TaskCard", () => {
  it("renders progress risk badges for at-risk tasks", () => {
    const onClick = jest.fn();

    render(
      <TaskCard
        index={0}
        isSelected={false}
        density="comfortable"
        showDescription={true}
        onClick={onClick}
        task={{
          id: "task-1",
          projectId: "project-1",
          title: "Implement detector",
          description: "",
          status: "in_progress",
          priority: "high",
          assigneeId: "member-1",
          assigneeType: "human",
          assigneeName: "Alice",
          cost: 3.5,
          budgetUsd: 5,
          spentUsd: 3.5,
          agentBranch: "",
          agentWorktree: "",
          agentSessionId: "",
          labels: [],
          blockedBy: [],
          plannedStartAt: null,
          plannedEndAt: null,
          progress: {
            lastActivityAt: "2026-03-24T11:00:00.000Z",
            lastActivitySource: "agent_heartbeat",
            lastTransitionAt: "2026-03-24T10:00:00.000Z",
            healthStatus: "stalled",
            riskReason: "no_recent_update",
            riskSinceAt: "2026-03-24T11:30:00.000Z",
            lastAlertState: "stalled:no_recent_update",
            lastAlertAt: "2026-03-24T11:35:00.000Z",
            lastRecoveredAt: null,
          },
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T12:00:00.000Z",
        }}
      />
    );

    expect(screen.getByText("Stalled")).toBeInTheDocument();
    expect(screen.getByText("No recent update")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Implement detector"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("shows a linked-doc preview when the docs indicator is hovered", async () => {
    const user = userEvent.setup();

    render(
      <TaskCard
        index={0}
        isSelected={false}
        density="comfortable"
        showDescription={true}
        onClick={jest.fn()}
        linkedDocs={[
          {
            id: "link-1",
            pageId: "page-1",
            title: "Architecture brief",
            linkType: "design",
            updatedAt: "2026-03-24T12:00:00.000Z",
            preview: "Line 1\nLine 2\nLine 3\nLine 4",
          },
        ]}
        task={{
          id: "task-2",
          projectId: "project-1",
          title: "Review docs hover",
          description: "",
          status: "in_progress",
          priority: "medium",
          assigneeId: null,
          assigneeType: null,
          assigneeName: null,
          cost: null,
          budgetUsd: 0,
          spentUsd: 0,
          agentBranch: "",
          agentWorktree: "",
          agentSessionId: "",
          labels: [],
          blockedBy: [],
          plannedStartAt: null,
          plannedEndAt: null,
          progress: null,
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T12:00:00.000Z",
        }}
      />
    );

    await user.hover(
      screen.getByRole("button", { name: "Show linked docs for Review docs hover" })
    );

    expect(await screen.findByText("Architecture brief")).toBeInTheDocument();
    expect(screen.getByText(/Line 1\s+Line 2\s+Line 3/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View" })).toHaveAttribute(
      "href",
      "/docs?pageId=page-1"
    );
  });

  it("uses ctrl/cmd click to toggle multi-select without opening task details", () => {
    const onClick = jest.fn();
    const onToggleSelect = jest.fn();

    render(
      <TaskCard
        index={0}
        isSelected={false}
        isMultiSelected={false}
        density="comfortable"
        showDescription={true}
        onClick={onClick}
        onToggleSelect={onToggleSelect}
        task={{
          id: "task-3",
          projectId: "project-1",
          title: "Bulk select candidate",
          description: "",
          status: "triaged",
          priority: "medium",
          assigneeId: null,
          assigneeType: null,
          assigneeName: null,
          cost: null,
          budgetUsd: 0,
          spentUsd: 0,
          agentBranch: "",
          agentWorktree: "",
          agentSessionId: "",
          labels: [],
          blockedBy: [],
          plannedStartAt: null,
          plannedEndAt: null,
          progress: null,
          createdAt: "2026-03-24T09:00:00.000Z",
          updatedAt: "2026-03-24T12:00:00.000Z",
        }}
      />
    );

    fireEvent.click(screen.getByText("Bulk select candidate"), { ctrlKey: true });

    expect(onToggleSelect).toHaveBeenCalledWith("task-3");
    expect(onClick).not.toHaveBeenCalled();
  });
});
