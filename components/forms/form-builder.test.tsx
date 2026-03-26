import { render, screen } from "@testing-library/react";
import { FormBuilder } from "./form-builder";
import { useFormStore } from "@/lib/stores/form-store";
import { useCustomFieldStore } from "@/lib/stores/custom-field-store";

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
