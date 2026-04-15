import { Children, isValidElement, type ReactNode, type ReactElement } from "react";

jest.mock("@/components/ui/select", () => {
  function flattenOptions(children: ReactNode): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [];
    function visit(node: ReactNode) {
      Children.forEach(node, (child) => {
        if (!isValidElement(child)) return;
        const element = child as ReactElement<{ children?: ReactNode; value?: string }>;
        if (element.props.value !== undefined) {
          options.push({
            value: element.props.value,
            label: typeof element.props.children === "string" ? element.props.children : String(element.props.value),
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
    Select: ({ value, onValueChange, children }: { value?: string; onValueChange?: (v: string) => void; children?: ReactNode }) => {
      const options = flattenOptions(children);
      return (
        <select value={value} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onValueChange?.(e.target.value)}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
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

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "automations.ruleName": "Rule Name",
      "automations.ruleNamePlaceholder": "Rule name",
      "automations.event": "Event",
      "automations.conditionField": "Condition Field",
      "automations.conditionValue": "Condition Value",
      "automations.action": "Action",
      "automations.createRule": "Create Rule",
    };
    return map[key] ?? key;
  },
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RuleEditor } from "./rule-editor";
import { useAutomationStore } from "@/lib/stores/automation-store";

const createRuleMock = jest.fn();

describe("RuleEditor", () => {
  beforeEach(() => {
    createRuleMock.mockReset();
    createRuleMock.mockResolvedValue(undefined);

    useAutomationStore.setState({
      createRule: createRuleMock,
    });
  });

  it("requires a name and creates rules with the configured condition and action", async () => {
    const user = userEvent.setup();

    render(<RuleEditor projectId="project-1" />);

    const createButton = screen.getByRole("button", { name: "Create Rule" });
    expect(createButton).toBeDisabled();

    const textboxes = screen.getAllByRole("textbox");
    await user.type(textboxes[0], "Notify blocked tasks");
    const selects = screen.getAllByRole("combobox");
    await user.selectOptions(selects[0], "review.completed");
    await user.clear(textboxes[1]);
    await user.type(textboxes[1], "priority");
    await user.clear(textboxes[2]);
    await user.type(textboxes[2], "high");
    await user.selectOptions(selects[1], "send_im_message");
    await user.click(createButton);

    await waitFor(() => {
      expect(createRuleMock).toHaveBeenCalledWith("project-1", {
        name: "Notify blocked tasks",
        enabled: true,
        eventType: "review.completed",
        conditions: [{ field: "priority", op: "eq", value: "high" }],
        actions: [{ type: "send_im_message", config: {} }],
      });
    });
  });

  it("requires a workflow plugin id for start_workflow actions", async () => {
    const user = userEvent.setup();

    render(<RuleEditor projectId="project-1" />);

    const textboxes = screen.getAllByRole("textbox");
    await user.type(textboxes[0], "Escalate due work");
    const selects = screen.getAllByRole("combobox");
    await user.selectOptions(selects[1], "start_workflow");

    const createButton = screen.getByRole("button", { name: "Create Rule" });
    expect(createButton).toBeDisabled();

    await user.type(screen.getByPlaceholderText("task-delivery-flow"), "task-delivery-flow");
    expect(createButton).toBeEnabled();

    await user.click(createButton);

    await waitFor(() => {
      expect(createRuleMock).toHaveBeenCalledWith("project-1", {
        name: "Escalate due work",
        enabled: true,
        eventType: "task.status_changed",
        conditions: [{ field: "status", op: "eq", value: "done" }],
        actions: [{ type: "start_workflow", config: { pluginId: "task-delivery-flow" } }],
      });
    });
  });
});
