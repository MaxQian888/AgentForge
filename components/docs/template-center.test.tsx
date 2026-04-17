import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { KnowledgeAsset } from "@/lib/stores/knowledge-store";
import { TemplateCenter } from "./template-center";

function makeTemplate(overrides: Partial<KnowledgeAsset> = {}): KnowledgeAsset {
  return {
    id: "template-1",
    projectId: "project-1",
    kind: "template",
    spaceId: "space-1",
    parentId: null,
    title: "Incident Runbook",
    contentJson: "[]",
    contentText: "",
    path: "/incident-runbook",
    sortOrder: 0,
    templateCategory: "runbook",
    isSystemTemplate: true,
    isPinned: false,
    createdBy: "user-1",
    updatedBy: "user-1",
    createdAt: "2026-03-26T12:00:00.000Z",
    updatedAt: "2026-03-26T12:00:00.000Z",
    deletedAt: null,
    version: 1,
    templateSource: "system",
    previewSnippet: "Operational checklist",
    canEdit: false,
    canDelete: false,
    canDuplicate: true,
    canUse: true,
    ...overrides,
  };
}

describe("TemplateCenter", () => {
  it("renders templates, filters them, and exposes management actions", async () => {
    const user = userEvent.setup();
    const onCreateFromTemplate = jest.fn();
    const onCreateTemplate = jest.fn();
    const onEditTemplate = jest.fn();
    const onDuplicateTemplate = jest.fn();
    const onDeleteTemplate = jest.fn().mockResolvedValue(undefined);
    const confirmSpy = jest.spyOn(window, "confirm").mockReturnValue(true);

    render(
      <TemplateCenter
        templates={[
          makeTemplate(),
          makeTemplate({
            id: "template-2",
            title: "Custom Postmortem",
            templateCategory: "postmortem",
            isSystemTemplate: false,
            templateSource: "custom",
            canEdit: true,
            canDelete: true,
          }),
        ]}
        onCreateFromTemplate={onCreateFromTemplate}
        onCreateTemplate={onCreateTemplate}
        onEditTemplate={onEditTemplate}
        onDuplicateTemplate={onDuplicateTemplate}
        onDeleteTemplate={onDeleteTemplate}
      />,
    );

    expect(screen.getByText("Template Center")).toBeInTheDocument();
    expect(screen.getAllByText("Operational checklist").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Custom" }));
    expect(screen.queryByText("Incident Runbook")).not.toBeInTheDocument();
    expect(screen.getAllByText("Custom Postmortem").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "Use Template" }));
    expect(onCreateFromTemplate).toHaveBeenCalledWith("template-2");

    await user.click(screen.getByRole("button", { name: "Edit Template" }));
    expect(onEditTemplate).toHaveBeenCalledWith("template-2");

    await user.click(screen.getByRole("button", { name: "Duplicate Template" }));
    await user.click(screen.getByRole("button", { name: "Create Duplicate" }));
    expect(onDuplicateTemplate).toHaveBeenCalledWith({
      templateId: "template-2",
      name: "Custom Postmortem Copy",
      category: "postmortem",
    });

    await user.click(screen.getByRole("button", { name: "Delete Template" }));
    expect(confirmSpy).toHaveBeenCalled();
    expect(onDeleteTemplate).toHaveBeenCalledWith("template-2");

    await user.click(screen.getByRole("button", { name: "New Template" }));
    await user.type(screen.getByLabelText("Template Title"), "Playbook");
    await user.click(screen.getByRole("button", { name: "Create Template" }));
    expect(onCreateTemplate).toHaveBeenCalledWith({
      title: "Playbook",
      category: "custom",
    });

    confirmSpy.mockRestore();
  });

  it("shows an empty message when templates have not loaded yet", () => {
    render(<TemplateCenter templates={[]} />);

    expect(
      screen.getByText("Templates will appear here after they load."),
    ).toBeInTheDocument();
  });
});
