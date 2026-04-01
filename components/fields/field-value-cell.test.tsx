import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FieldValueCell } from "./field-value-cell";
import { useCustomFieldStore } from "@/lib/stores/custom-field-store";

jest.mock("@/components/ui/select", () => {
  function flattenOptions(children: ReactNode): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [];
    function visit(node: ReactNode) {
      Children.forEach(node, (child) => {
        if (!isValidElement(child)) return;
        const el = child as ReactElement<{ children?: ReactNode; value?: string }>;
        if (el.props.value !== undefined) {
          options.push({
            value: el.props.value,
            label: typeof el.props.children === "string" ? el.props.children : String(el.props.value),
          });
          return;
        }
        visit(el.props.children);
      });
    }
    visit(children);
    return options;
  }

  return {
    Select: ({
      value,
      onValueChange,
      children,
    }: {
      value?: string;
      onValueChange?: (value: string) => void;
      children?: ReactNode;
    }) => {
      const options = flattenOptions(children);
      return (
        <select
          value={value}
          onChange={(e) => onValueChange?.(e.target.value)}
        >
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

    await user.selectOptions(screen.getByRole("combobox"), "__none__");
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
