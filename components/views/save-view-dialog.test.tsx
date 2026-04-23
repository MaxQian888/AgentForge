import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SaveViewDialog } from "./save-view-dialog";

jest.mock("next-intl", () => ({
  useTranslations: (ns: string) => (key: string) => `${ns}.${key}`,
}));

const createViewMock = jest.fn();

jest.mock("@/lib/stores/saved-view-store", () => ({
  useSavedViewStore: (
    selector: (state: { createView: typeof createViewMock }) => unknown,
  ) =>
    selector({
      createView: createViewMock,
    }),
}));

describe("SaveViewDialog", () => {
  beforeEach(() => {
    createViewMock.mockReset();
  });

  it("requires a trimmed name before saving and persists shared visibility", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();
    const config = { layout: "board", filters: [] };
    createViewMock.mockResolvedValue(undefined);

    render(
      <SaveViewDialog
        open
        onOpenChange={onOpenChange}
        projectId="project-1"
        config={config}
      />,
    );

    const saveButton = screen.getByRole("button", { name: "common.action.save" });
    expect(saveButton).toBeDisabled();

    await user.type(screen.getByPlaceholderText("views.saveViewDialog.namePlaceholder"), "  Triage  ");
    await user.click(
      screen.getByRole("checkbox", { name: "views.saveViewDialog.sharedLabel" }),
    );

    expect(saveButton).toBeEnabled();

    await user.click(saveButton);

    await waitFor(() => {
      expect(createViewMock).toHaveBeenCalledWith("project-1", {
        name: "  Triage  ",
        config,
        isDefault: false,
        sharedWith: {
          roleIds: [],
          memberIds: [],
        },
      });
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("closes without saving when cancelled", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();

    render(
      <SaveViewDialog
        open
        onOpenChange={onOpenChange}
        projectId="project-1"
        config={{ layout: "list" }}
      />,
    );

    await user.click(screen.getByRole("button", { name: "common.action.cancel" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(createViewMock).not.toHaveBeenCalled();
  });
});
