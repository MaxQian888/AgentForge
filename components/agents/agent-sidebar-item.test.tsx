import { fireEvent, render, screen } from "@testing-library/react";
import type { Agent } from "@/lib/stores/agent-store";
import { AgentSidebarItem } from "./agent-sidebar-item";
import { TooltipProvider } from "@/components/ui/tooltip";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
}));

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "agent-1",
    taskId: "task-1",
    taskTitle: "Paused verification",
    memberId: "member-1",
    roleId: "role-1",
    roleName: "Reviewer",
    status: "paused",
    runtime: "claude_code",
    provider: "anthropic",
    model: "claude-sonnet-4-5",
    turns: 7,
    cost: 1.5,
    budget: 5,
    worktreePath: "",
    branchName: "",
    sessionId: "",
    lastActivity: "2026-03-26T10:10:00.000Z",
    startedAt: "2026-03-26T10:00:00.000Z",
    createdAt: "2026-03-26T10:00:00.000Z",
    canResume: true,
    memoryStatus: "available",
    dispatchStatus: "blocked",
    guardrailType: "budget",
    ...overrides,
  };
}

describe("AgentSidebarItem", () => {
  it("does not nest quick action buttons inside the selectable item", () => {
    const { container } = render(
      <TooltipProvider>
        <AgentSidebarItem
          agent={makeAgent()}
          selected={false}
          onSelect={jest.fn()}
          onPause={jest.fn()}
          onResume={jest.fn()}
          onKill={jest.fn()}
          bridgeDegraded={true}
        />
      </TooltipProvider>,
    );

    expect(container.querySelector("button button")).toBeNull();
  });

  it("supports keyboard selection on the outer item", () => {
    const onSelect = jest.fn();

    render(
      <TooltipProvider>
        <AgentSidebarItem
          agent={makeAgent()}
          selected={false}
          onSelect={onSelect}
          onPause={jest.fn()}
          onResume={jest.fn()}
          onKill={jest.fn()}
          bridgeDegraded={false}
        />
      </TooltipProvider>,
    );

    const item = screen.getByText("Paused verification").closest('[role="button"],button');
    expect(item).not.toBeNull();

    fireEvent.keyDown(item!, { key: "Enter" });

    expect(onSelect).toHaveBeenCalledWith("agent-1");
  });

  it("shows an accessible budget progress indicator", () => {
    render(
      <TooltipProvider>
        <AgentSidebarItem
          agent={makeAgent()}
          selected={false}
          onSelect={jest.fn()}
          onPause={jest.fn()}
          onResume={jest.fn()}
          onKill={jest.fn()}
          bridgeDegraded={false}
        />
      </TooltipProvider>,
    );

    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "30",
    );
  });
});
