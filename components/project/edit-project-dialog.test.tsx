jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "editProject.title": "Edit project",
      "editProject.description": "Update project details",
      "editProject.nameLabel": "Project name",
      "editProject.descriptionLabel": "Description",
      "editProject.repoUrlLabel": "Repository URL",
      "editProject.defaultBranchLabel": "Default branch",
      "editProject.cancel": "Cancel",
      "editProject.save": "Save",
      "editProject.saving": "Saving",
    };
    return map[key] ?? key;
  },
}));

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditProjectDialog } from "./edit-project-dialog";
import type { Project } from "@/lib/stores/project-store";

const project: Project = {
  id: "project-1",
  name: "Release Ops",
  description: "Current release control plane",
  status: "active",
  taskCount: 12,
  agentCount: 4,
  createdAt: "2026-03-30T00:00:00.000Z",
  repoUrl: "https://github.com/acme/release-ops",
  defaultBranch: "main",
  settings: {
    codingAgent: {
      runtime: "codex",
      provider: "openai",
      model: "gpt-5.4",
    },
  },
};

describe("EditProjectDialog", () => {
  it("saves only changed fields and trims the project name", async () => {
    const user = userEvent.setup();
    const onSave = jest.fn().mockResolvedValue(undefined);
    const onClose = jest.fn();

    render(
      <EditProjectDialog
        open
        project={project}
        onSave={onSave}
        onClose={onClose}
      />,
    );

    const nameInput = screen.getByLabelText("Project name");
    await user.clear(nameInput);
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    await user.type(nameInput, "  New release ops  ");
    await user.clear(screen.getByLabelText("Description"));
    await user.type(screen.getByLabelText("Description"), "Release automation");
    await user.clear(screen.getByLabelText("Repository URL"));
    await user.type(
      screen.getByLabelText("Repository URL"),
      "https://github.com/acme/new-release-ops",
    );
    await user.clear(screen.getByLabelText("Default branch"));
    await user.type(screen.getByLabelText("Default branch"), "develop");

    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith("project-1", {
        name: "New release ops",
        description: "Release automation",
        repoUrl: "https://github.com/acme/new-release-ops",
        defaultBranch: "develop",
      });
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("closes immediately when cancelled", async () => {
    const user = userEvent.setup();
    const onClose = jest.fn();

    render(
      <EditProjectDialog
        open
        project={project}
        onSave={jest.fn().mockResolvedValue(undefined)}
        onClose={onClose}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onClose).toHaveBeenCalled();
  });
});
