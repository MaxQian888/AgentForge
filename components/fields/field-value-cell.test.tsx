import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FieldValueCell } from "./field-value-cell";
import { useCustomFieldStore } from "@/lib/stores/custom-field-store";

const setTaskValueMock = jest.fn();
const clearTaskValueMock = jest.fn();

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) =>
    key === "fields.unset" ? "Unset" : key,
}));

describe("FieldValueCell", () => {
  beforeEach(() => {
    setTaskValueMock.mockReset();
    clearTaskValueMock.mockReset();

    useCustomFieldStore.setState({
      setTaskValue: setTaskValueMock,
      clearTaskValue: clearTaskValueMock,
    });
  });

  it("commits checkbox values immediately", async () => {
    const user = userEvent.setup();

    render(
      <FieldValueCell
        projectId="project-1"
        taskId="task-1"
        field={{
          id: "field-checkbox",
          projectId: "project-1",
          name: "Blocked",
          fieldType: "checkbox",
          options: null,
          sortOrder: 1,
          required: false,
          createdAt: "",
          updatedAt: "",
        }}
        value={null}
      />,
    );

    await user.click(screen.getByRole("checkbox"));

    expect(setTaskValueMock).toHaveBeenCalledWith(
      "project-1",
      "task-1",
      "field-checkbox",
      true,
    );
  });

  it("supports select fields and clears the value when unset is chosen", async () => {
    const user = userEvent.setup();

    render(
      <FieldValueCell
        projectId="project-1"
        taskId="task-1"
        field={{
          id: "field-select",
          projectId: "project-1",
          name: "Priority",
          fieldType: "select",
          options: ["P0", "P1"],
          sortOrder: 2,
          required: false,
          createdAt: "",
          updatedAt: "",
        }}
        value={{ id: "value-1", taskId: "task-1", fieldDefId: "field-select", value: "P0", createdAt: "", updatedAt: "" }}
      />,
    );

    await user.selectOptions(screen.getByRole("combobox"), "P1");
    expect(setTaskValueMock).toHaveBeenCalledWith(
      "project-1",
      "task-1",
      "field-select",
      "P1",
    );

    await user.selectOptions(screen.getByRole("combobox"), "");
    await waitFor(() => {
      expect(clearTaskValueMock).toHaveBeenCalledWith(
        "project-1",
        "task-1",
        "field-select",
      );
    });
  });

  it("commits text values on blur and clears empty input", async () => {
    const user = userEvent.setup();

    render(
      <FieldValueCell
        projectId="project-1"
        taskId="task-1"
        field={{
          id: "field-text",
          projectId: "project-1",
          name: "Owner",
          fieldType: "text",
          options: null,
          sortOrder: 3,
          required: false,
          createdAt: "",
          updatedAt: "",
        }}
        value={{ id: "value-2", taskId: "task-1", fieldDefId: "field-text", value: "alice", createdAt: "", updatedAt: "" }}
      />,
    );

    const input = screen.getByDisplayValue("alice");
    await user.clear(input);
    await user.tab();

    await waitFor(() => {
      expect(clearTaskValueMock).toHaveBeenCalledWith(
        "project-1",
        "task-1",
        "field-text",
      );
    });
  });
});
