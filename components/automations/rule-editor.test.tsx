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
});
