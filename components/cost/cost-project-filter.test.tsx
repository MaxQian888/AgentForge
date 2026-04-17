import { render, screen } from "@testing-library/react";
import { CostProjectFilter } from "./cost-project-filter";

describe("CostProjectFilter", () => {
  it("renders the filter with the selected project label in the trigger", () => {
    const onChange = jest.fn();
    render(
      <CostProjectFilter
        projects={[
          { id: "p-1", name: "Alpha" },
          { id: "p-2", name: "Beta" },
        ]}
        selectedProjectId="p-1"
        onChange={onChange}
      />,
    );
    expect(screen.getByText("Project")).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toHaveAttribute(
      "aria-label",
      "Project",
    );
    expect(screen.getByText("Alpha")).toBeInTheDocument();
  });

  it("renders with a combobox trigger when no project is selected", () => {
    const onChange = jest.fn();
    render(
      <CostProjectFilter
        projects={[{ id: "p-1", name: "Alpha" }]}
        selectedProjectId={null}
        onChange={onChange}
      />,
    );
    expect(screen.getByText("Project")).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toBeInTheDocument();
  });
});
