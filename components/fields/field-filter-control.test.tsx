import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FieldFilterControl } from "./field-filter-control";

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

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string>) => {
    if (key === "fields.filterAll") {
      return "All values";
    }
    if (key === "fields.filterPlaceholder") {
      return `Filter ${values?.name ?? "field"}`;
    }
    return key;
  },
}));

describe("FieldFilterControl", () => {
  it("renders a select control for option-based fields", async () => {
    const user = userEvent.setup();
    const onChange = jest.fn();

    render(
      <FieldFilterControl
        field={{
          id: "field-1",
          projectId: "project-1",
          name: "Priority",
          fieldType: "select",
          options: ["P0", "P1"],
          sortOrder: 1,
          required: false,
          createdAt: "",
          updatedAt: "",
        }}
        value=""
        onChange={onChange}
      />,
    );

    expect(screen.getByRole("combobox")).toBeInTheDocument();
    await user.selectOptions(screen.getByRole("combobox"), "P1");

    expect(onChange).toHaveBeenCalledWith("P1");
    expect(screen.getByRole("option", { name: "All values" })).toBeInTheDocument();
  });

  it("renders a text input for free-form fields", async () => {
    const onChange = jest.fn();

    render(
      <FieldFilterControl
        field={{
          id: "field-2",
          projectId: "project-1",
          name: "Owner",
          fieldType: "text",
          options: null,
          sortOrder: 2,
          required: false,
          createdAt: "",
          updatedAt: "",
        }}
        value=""
        onChange={onChange}
      />,
    );

    const input = screen.getByPlaceholderText("Filter Owner");
    fireEvent.change(input, { target: { value: "alice" } });

    expect(onChange).toHaveBeenLastCalledWith("alice");
  });
});
