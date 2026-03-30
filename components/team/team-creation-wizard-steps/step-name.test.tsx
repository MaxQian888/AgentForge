jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "wizard.nameLabel": "Team Name",
      "wizard.namePlaceholder": "Name your team",
      "wizard.descriptionLabel": "Description",
      "wizard.descriptionPlaceholder": "Describe the team",
      "wizard.objectiveLabel": "Objective",
      "wizard.objectivePlaceholder": "Ship the task",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { fireEvent } from "@testing-library/react";
import { StepName } from "./step-name";

describe("StepName", () => {
  it("emits updates for name, description, and objective fields", () => {
    const onChange = jest.fn();

    render(
      <StepName
        data={{ name: "", description: "", objective: "" }}
        onChange={onChange}
      />,
    );

    fireEvent.change(screen.getByLabelText("Team Name *"), {
      target: { value: "Release Squad" },
    });
    fireEvent.change(screen.getByLabelText("Description"), {
      target: { value: "Handles release flow" },
    });
    fireEvent.change(screen.getByLabelText("Objective"), {
      target: { value: "Ship stable builds" },
    });

    expect(onChange).toHaveBeenCalledWith({
      name: "Release Squad",
      description: "",
      objective: "",
    });
    expect(onChange).toHaveBeenLastCalledWith({
      name: "",
      description: "",
      objective: "Ship stable builds",
    });
  });
});
