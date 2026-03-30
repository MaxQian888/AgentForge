import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MilestoneEditor } from "./milestone-editor";
import { useMilestoneStore } from "@/lib/stores/milestone-store";

const createMilestoneMock = jest.fn();

describe("MilestoneEditor", () => {
  beforeEach(() => {
    createMilestoneMock.mockReset();
    createMilestoneMock.mockResolvedValue(undefined);

    useMilestoneStore.setState({
      createMilestone: createMilestoneMock,
    });
  });

  it("requires a name and saves milestone details", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(
      <MilestoneEditor
        open
        onOpenChange={onOpenChange}
        projectId="project-1"
      />,
    );

    const saveButton = screen.getByRole("button", { name: "Save" });
    expect(saveButton).toBeDisabled();

    await user.type(screen.getByPlaceholderText("v2.0 Release"), "Release 2.0");
    const dateInput = document.querySelector('input[type="date"]') as HTMLInputElement;
    fireEvent.change(dateInput, { target: { value: "2026-04-30" } });
    await user.selectOptions(screen.getByRole("combobox"), "in_progress");
    await user.click(saveButton);

    await waitFor(() => {
      expect(createMilestoneMock).toHaveBeenCalledWith("project-1", {
        name: "Release 2.0",
        targetDate: "2026-04-30",
        status: "in_progress",
        description: "",
      });
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("closes without saving when cancelled", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(
      <MilestoneEditor
        open
        onOpenChange={onOpenChange}
        projectId="project-1"
      />,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(createMilestoneMock).not.toHaveBeenCalled();
  });
});
