jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "createDialog.title": "Create Scheduler Job",
      "createDialog.description": "Register a new background job.",
      "createDialog.jobKey": "Job key",
      "createDialog.jobKeyPlaceholder": "my.job",
      "createDialog.invalidJobKey": "Job key must start with a letter or digit.",
      "createDialog.name": "Name",
      "createDialog.namePlaceholder": "Human readable name",
      "createDialog.nameRequired": "Name is required",
      "createDialog.scope": "Scope",
      "createDialog.scopePlaceholder": "system | project",
      "createDialog.schedule": "Schedule",
      "createDialog.cancel": "Cancel",
      "createDialog.create": "Create",
      "createDialog.submitFailed": "Creation failed",
    };
    return map[key] ?? key;
  },
}));

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SchedulerJobCreateDialog } from "./scheduler-job-create-dialog";

describe("SchedulerJobCreateDialog", () => {
  it("disables submit when cron is invalid and surfaces validation inline", async () => {
    const onCreate = jest.fn().mockResolvedValue(true);

    render(
      <SchedulerJobCreateDialog
        open
        onOpenChange={jest.fn()}
        onCreate={onCreate}
        actionLoading={false}
      />,
    );

    const scheduleInput = screen.getByLabelText("Schedule");
    fireEvent.change(scheduleInput, { target: { value: "not a cron" } });
    fireEvent.blur(scheduleInput);

    expect(
      await screen.findByText(/Expected 5 fields/i),
    ).toBeInTheDocument();

    const submit = screen.getByRole("button", { name: "Create" });
    expect(submit).toBeDisabled();
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("submits valid cron + job key and closes on success", async () => {
    const user = userEvent.setup();
    const onCreate = jest.fn().mockResolvedValue(true);
    const onOpenChange = jest.fn();

    render(
      <SchedulerJobCreateDialog
        open
        onOpenChange={onOpenChange}
        onCreate={onCreate}
        actionLoading={false}
      />,
    );

    await user.type(screen.getByLabelText("Job key"), "new.job");
    await user.type(screen.getByLabelText("Name"), "New Job");
    fireEvent.change(screen.getByLabelText("Schedule"), {
      target: { value: "0 * * * *" },
    });

    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(onCreate).toHaveBeenCalledWith({
        jobKey: "new.job",
        name: "New Job",
        schedule: "0 * * * *",
        scope: "system",
      });
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("surfaces submit failure and keeps the dialog open", async () => {
    const user = userEvent.setup();
    const onCreate = jest.fn().mockResolvedValue(false);
    const onOpenChange = jest.fn();

    render(
      <SchedulerJobCreateDialog
        open
        onOpenChange={onOpenChange}
        onCreate={onCreate}
        actionLoading={false}
      />,
    );

    await user.type(screen.getByLabelText("Job key"), "new.job");
    await user.type(screen.getByLabelText("Name"), "New Job");

    await user.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(screen.getByText("Creation failed")).toBeInTheDocument();
    });
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
