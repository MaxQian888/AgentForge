import { render, screen } from "@testing-library/react";
import { FieldDefinitionEditor } from "./field-definition-editor";
import { useCustomFieldStore } from "@/lib/stores/custom-field-store";

const fetchDefinitions = jest.fn().mockResolvedValue(undefined);
const createDefinition = jest.fn().mockResolvedValue(undefined);
const deleteDefinition = jest.fn().mockResolvedValue(undefined);
const reorderDefinitions = jest.fn().mockResolvedValue(undefined);

describe("FieldDefinitionEditor", () => {
  beforeEach(() => {
    fetchDefinitions.mockClear();
    createDefinition.mockClear();
    deleteDefinition.mockClear();
    reorderDefinitions.mockClear();

    useCustomFieldStore.setState({
      definitionsByProject: {},
      fetchDefinitions,
      createDefinition,
      deleteDefinition,
      reorderDefinitions,
    });
  });

  it("renders an empty-state editor when the project has no custom fields yet", () => {
    render(<FieldDefinitionEditor projectId="project-1" />);

    expect(screen.getByRole("button", { name: "Add field" })).toBeInTheDocument();
    expect(fetchDefinitions).toHaveBeenCalledWith("project-1");
  });
});
