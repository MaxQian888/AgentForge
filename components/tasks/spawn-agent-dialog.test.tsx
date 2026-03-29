import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SpawnAgentDialog } from "./spawn-agent-dialog";

const fetchRuntimeCatalog = jest.fn();
const fetchBridgeHealth = jest.fn();
const spawnAgent = jest.fn();

const storeState = {
  runtimeCatalog: {
    defaultRuntime: "codex",
    defaultSelection: {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
    },
    runtimes: [
      {
        runtime: "codex",
        label: "Codex",
        defaultProvider: "openai",
        compatibleProviders: ["openai"],
        defaultModel: "gpt-5-codex",
        available: true,
        diagnostics: [],
      },
    ],
  },
  bridgeHealth: {
    status: "ready",
    lastCheck: "2026-03-28T12:00:00.000Z",
    pool: {
      active: 1,
      available: 1,
      warm: 0,
    },
  },
  fetchRuntimeCatalog,
  fetchBridgeHealth,
  spawnAgent,
};

jest.mock("@/lib/stores/agent-store", () => ({
  useAgentStore: (selector: (state: typeof storeState) => unknown) => selector(storeState),
}));

jest.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
  DialogContent: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
  DialogFooter: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
}));

jest.mock("@/components/ui/select", () => {
  function flattenOptions(children: ReactNode): Array<{ value: string; label: string; disabled: boolean }> {
    const options: Array<{ value: string; label: string; disabled: boolean }> = [];

    function visit(node: ReactNode) {
      Children.forEach(node, (child) => {
        if (!isValidElement(child)) return;
        const element = child as ReactElement<{ children?: ReactNode; value?: string; disabled?: boolean }>;
        if (element.props.value !== undefined) {
          options.push({
            value: element.props.value,
            label: typeof element.props.children === "string" ? element.props.children : String(element.props.value),
            disabled: Boolean(element.props.disabled),
          });
          return;
        }
        visit(element.props.children);
      });
    }

    visit(children);
    return options;
  }

  return {
    Select: ({
      value,
      onValueChange,
      disabled,
      children,
    }: {
      value?: string;
      onValueChange?: (value: string) => void;
      disabled?: boolean;
      children?: ReactNode;
    }) => {
      const options = flattenOptions(children);
      return (
        <select
          aria-label="spawn-agent-select"
          value={value}
          disabled={disabled}
          onChange={(event) => onValueChange?.(event.target.value)}
        >
          {options.map((option) => (
            <option key={option.value} value={option.value} disabled={option.disabled}>
              {option.label}
            </option>
          ))}
        </select>
      );
    },
    SelectTrigger: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectValue: () => null,
    SelectContent: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectItem: ({ children }: { children?: ReactNode }) => <>{children}</>,
  };
});

describe("SpawnAgentDialog", () => {
  beforeEach(() => {
    fetchRuntimeCatalog.mockReset().mockResolvedValue(storeState.runtimeCatalog);
    fetchBridgeHealth.mockReset().mockResolvedValue(storeState.bridgeHealth);
    spawnAgent.mockReset().mockResolvedValue(undefined);
    storeState.bridgeHealth.status = "ready";
  });

  it("loads bridge catalog/health when opened and submits defaults", async () => {
    const user = userEvent.setup();
    render(
      <SpawnAgentDialog
        taskId="task-1"
        taskTitle="Implement bridge validation"
        memberId="member-1"
        open
        onOpenChange={jest.fn()}
      />,
    );

    await waitFor(() => expect(fetchRuntimeCatalog).toHaveBeenCalled());
    expect(fetchBridgeHealth).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Start Agent" }));

    expect(spawnAgent).toHaveBeenCalledWith("task-1", "member-1", {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      maxBudgetUsd: 5,
    });
  });

  it("disables spawn while bridge health is degraded", () => {
    storeState.bridgeHealth.status = "degraded";

    render(
      <SpawnAgentDialog
        taskId="task-2"
        taskTitle="Handle degraded state"
        memberId="member-2"
        open
        onOpenChange={jest.fn()}
      />,
    );

    expect(screen.getByText(/Bridge is degraded/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Start Agent" })).toBeDisabled();
  });
});
