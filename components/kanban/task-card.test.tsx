jest.mock("@hello-pangea/dnd", () => ({
  Draggable: ({ children }: { children: (provided: any, snapshot: any) => React.ReactNode }) =>
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
import { TaskCard } from "./task-card";

describe("TaskCard", () => {
  it("renders progress risk badges for at-risk tasks", () => {
    const onClick = jest.fn();

    render(
      <TaskCard
        index={0}
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
          spentUsd: 3.5,
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
});
