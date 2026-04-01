import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import { FormBuilder } from "./form-builder";
import { useFormStore } from "@/lib/stores/form-store";
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

const fetchForms = jest.fn().mockResolvedValue(undefined);
const createForm = jest.fn().mockResolvedValue(undefined);
const deleteForm = jest.fn().mockResolvedValue(undefined);
const fetchDefinitions = jest.fn().mockResolvedValue(undefined);

describe("FormBuilder", () => {
  beforeEach(() => {
    fetchForms.mockClear();
    createForm.mockClear();
    deleteForm.mockClear();
    fetchDefinitions.mockClear();

    useFormStore.setState({
      formsByProject: {},
      fetchForms,
      createForm,
      deleteForm,
    });

    useCustomFieldStore.setState({
      definitionsByProject: {},
      fetchDefinitions,
    });
  });

  it("renders the builder when a project has no forms or custom fields yet", () => {
    render(<FormBuilder projectId="project-1" />);

    expect(screen.getByRole("button", { name: "Create form" })).toBeInTheDocument();
    expect(fetchForms).toHaveBeenCalledWith("project-1");
    expect(fetchDefinitions).toHaveBeenCalledWith("project-1");
  });
});
