jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "wizard.noRolesSelected": "Select roles first.",
      "wizard.agentsHint": "Assign agents to each role",
      "wizard.selectAgent": "Select an agent",
      "wizard.autoAssign": "Auto assign",
    };
    return map[key] ?? key;
  },
}));

jest.mock("@/components/ui/select", () => ({
  Select: ({
    value,
    onValueChange,
    children,
  }: {
    value?: string;
    onValueChange?: (value: string) => void;
    children?: React.ReactNode;
  }) => (
    <div data-testid={`select-${value}`}>
      {children}
      <button type="button" onClick={() => onValueChange?.("agent-1")}>
        Choose agent-1
      </button>
    </div>
  ),
  SelectTrigger: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectValue: ({ placeholder }: { placeholder?: string }) => <span>{placeholder}</span>,
  SelectContent: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
}));

const fetchAgentsMock = jest.fn();
const roleStoreState = {
  roles: [] as Array<Record<string, unknown>>,
};
const agentStoreState = {
  agents: [] as Array<Record<string, unknown>>,
  fetchAgents: fetchAgentsMock,
};

jest.mock("@/lib/stores/role-store", () => ({
  useRoleStore: (selector: (state: typeof roleStoreState) => unknown) =>
    selector(roleStoreState),
}));

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: () => agentStoreState,
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StepAgents } from "./step-agents";

describe("StepAgents", () => {
  beforeEach(() => {
    fetchAgentsMock.mockReset();
    roleStoreState.roles = [];
    agentStoreState.agents = [];
  });

  it("shows an empty prompt when no roles are selected", () => {
    render(
      <StepAgents
        selectedRoleIds={[]}
        agentAssignments={{}}
        onChange={jest.fn()}
      />,
    );

    expect(screen.getByText("Select roles first.")).toBeInTheDocument();
  });

  it("loads agents and updates role assignments", async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();
    roleStoreState.roles = [
      { metadata: { id: "frontend", name: "Frontend Developer" } },
    ];
    agentStoreState.agents = [
      { id: "agent-1", roleName: "Reviewer", status: "running" },
      { id: "agent-2", roleName: "Planner", status: "failed" },
    ];

    render(
      <StepAgents
        selectedRoleIds={["frontend"]}
        agentAssignments={{}}
        onChange={onChange}
      />,
    );

    expect(fetchAgentsMock).not.toHaveBeenCalled();
    expect(screen.getByText("Assign agents to each role")).toBeInTheDocument();
    expect(screen.getByText("Frontend Developer")).toBeInTheDocument();
    expect(screen.getByTestId("select-__auto__")).toHaveTextContent("Auto assign");
    expect(screen.getByTestId("select-__auto__")).toHaveTextContent("Reviewer (agent-1)");
    expect(screen.queryByText("Planner (agent-2)")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Choose agent-1" }));
    expect(onChange).toHaveBeenCalledWith({ frontend: "agent-1" });
  });
});
