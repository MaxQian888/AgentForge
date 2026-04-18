jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "saveAs.title": "Save as template",
      "saveAs.description": "Describe",
      "saveAs.nameLabel": "Template name",
      "saveAs.descriptionLabel": "Description",
      "saveAs.descriptionPlaceholder": "Notes",
      "saveAs.cancel": "Cancel",
      "saveAs.save": "Save as template",
      "saveAs.saving": "Saving",
    };
    return map[key] ?? key;
  },
}));

const saveAsTemplateMock = jest.fn().mockResolvedValue({ id: "tpl-1" });

jest.mock("@/lib/stores/project-template-store", () => ({
  useProjectTemplateStore: <T,>(selector: (s: unknown) => T) =>
    selector({
      saveAsTemplate: saveAsTemplateMock,
      saving: false,
    }),
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SaveAsTemplateDialog } from "./save-as-template-dialog";

describe("SaveAsTemplateDialog", () => {
  beforeEach(() => {
    saveAsTemplateMock.mockClear();
  });

  it("submits name and description then closes on success", async () => {
    const user = userEvent.setup();
    const onClose = jest.fn();
    render(
      <SaveAsTemplateDialog
        open
        projectId="proj-1"
        projectName="Release Ops"
        onClose={onClose}
      />,
    );

    // Prefilled name starts with projectName + " Template".
    const nameInput = screen.getByLabelText("Template name") as HTMLInputElement;
    expect(nameInput.value).toContain("Release Ops");

    const descriptionField = screen.getByLabelText("Description");
    await user.type(descriptionField, "Baseline for release ops");
    await user.click(screen.getByRole("button", { name: "Save as template" }));

    await waitFor(() => {
      expect(saveAsTemplateMock).toHaveBeenCalledWith(
        "proj-1",
        expect.objectContaining({
          description: "Baseline for release ops",
        }),
      );
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("requires a non-empty name", async () => {
    const user = userEvent.setup();
    const onClose = jest.fn();
    render(
      <SaveAsTemplateDialog
        open
        projectId="proj-1"
        projectName=""
        onClose={onClose}
      />,
    );
    const nameInput = screen.getByLabelText("Template name") as HTMLInputElement;
    await user.clear(nameInput);
    expect(screen.getByRole("button", { name: "Save as template" })).toBeDisabled();
    expect(saveAsTemplateMock).not.toHaveBeenCalled();
  });
});
