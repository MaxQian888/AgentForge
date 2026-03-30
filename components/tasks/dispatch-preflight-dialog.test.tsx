jest.mock("next-intl", () => ({
  useTranslations: () => (
    key: string,
    values?: Record<string, string | number>,
  ) => {
    const map: Record<string, string> = {
      "detail.dispatchHint.started": "Started",
      "detail.dispatchLikely": "Likely to dispatch",
      "detail.dispatchUncertain": "Dispatch uncertain",
      "detail.dispatchScope.project": "project",
      "detail.dispatchPreflightConfirm": "Assign and dispatch",
    };
    if (key === "detail.dispatchPreflightTitle") {
      return `Dispatch ${values?.name ?? ""}`;
    }
    if (key === "detail.dispatchPreflightDescription") {
      return `Check ${values?.task ?? ""} before dispatch`;
    }
    if (key === "detail.dispatchBudgetBlocked") {
      return `Blocked on ${values?.scope ?? ""}: ${values?.message ?? ""}`;
    }
    if (key === "detail.dispatchBudgetWarning") {
      return `Warning on ${values?.scope ?? ""}: ${values?.message ?? ""}`;
    }
    if (key === "detail.dispatchPoolSnapshot") {
      return `Pool ${values?.active ?? 0}/${values?.available ?? 0}/${values?.queued ?? 0}`;
    }
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DispatchPreflightDialog } from "./dispatch-preflight-dialog";

describe("DispatchPreflightDialog", () => {
  it("renders preflight details and forwards confirm/cancel actions", async () => {
    const user = userEvent.setup();
    const onConfirm = jest.fn();
    const onCancel = jest.fn();

    render(
      <DispatchPreflightDialog
        open
        taskTitle="Ship dashboard"
        memberName="Calendar Bot"
        summary={{
          dispatchOutcomeHint: "started",
          admissionLikely: true,
          budgetBlocked: { scope: "project", message: "No budget left" },
          budgetWarning: { scope: "project", message: "Near the threshold" },
          poolActive: 2,
          poolAvailable: 1,
          poolQueued: 3,
        }}
        onConfirm={onConfirm}
        onCancel={onCancel}
      />,
    );

    expect(screen.getByText("Dispatch Calendar Bot")).toBeInTheDocument();
    expect(screen.getByText("Check Ship dashboard before dispatch")).toBeInTheDocument();
    expect(screen.getByText("Started")).toBeInTheDocument();
    expect(screen.getByText("Likely to dispatch")).toBeInTheDocument();
    expect(screen.getByText("Blocked on project: No budget left")).toBeInTheDocument();
    expect(screen.getByText("Warning on project: Near the threshold")).toBeInTheDocument();
    expect(screen.getByText("Pool 2/1/3")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Assign and dispatch" }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onConfirm).toHaveBeenCalled();
    expect(onCancel).toHaveBeenCalled();
  });
});
