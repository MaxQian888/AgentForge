jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "automations.disable": "Disable",
      "automations.enable": "Enable",
      "automations.delete": "Delete",
    };
    return map[key] ?? key;
  },
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RuleList } from "./rule-list";
import { useAutomationStore } from "@/lib/stores/automation-store";

const fetchRulesMock = jest.fn();
const updateRuleMock = jest.fn();
const deleteRuleMock = jest.fn();

describe("RuleList", () => {
  beforeEach(() => {
    fetchRulesMock.mockReset();
    fetchRulesMock.mockResolvedValue(undefined);
    updateRuleMock.mockReset();
    updateRuleMock.mockResolvedValue(undefined);
    deleteRuleMock.mockReset();
    deleteRuleMock.mockResolvedValue(undefined);

    useAutomationStore.setState({
      rulesByProject: {
        "project-1": [
          {
            id: "rule-1",
            projectId: "project-1",
            name: "Notify reviewers",
            enabled: true,
            eventType: "review.completed",
            conditions: [],
            actions: [],
            createdBy: "user-1",
            createdAt: "",
            updatedAt: "",
          },
        ],
      },
      fetchRules: fetchRulesMock,
      updateRule: updateRuleMock,
      deleteRule: deleteRuleMock,
    });
  });

  it("loads rules and routes enable and delete actions", async () => {
    const user = userEvent.setup();

    render(<RuleList projectId="project-1" />);

    await waitFor(() => {
      expect(fetchRulesMock).toHaveBeenCalledWith("project-1");
    });

    expect(screen.getByText("Notify reviewers")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Disable" }));
    expect(updateRuleMock).toHaveBeenCalledWith("project-1", "rule-1", {
      enabled: false,
    });

    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(deleteRuleMock).toHaveBeenCalledWith("project-1", "rule-1");
  });
});
