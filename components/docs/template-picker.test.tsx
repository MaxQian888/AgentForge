import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { DocsPage } from "@/lib/stores/docs-store";
import { TemplatePicker } from "./template-picker";

function makeTemplate(overrides: Partial<DocsPage> = {}): DocsPage {
  return {
    id: "template-1",
    spaceId: "space-1",
    parentId: null,
    title: "Incident Runbook",
    content: "[]",
    contentText: "",
    path: "/incident-runbook",
    sortOrder: 0,
    isTemplate: true,
    templateCategory: "runbook",
    isSystem: true,
    isPinned: false,
    createdBy: "user-1",
    updatedBy: "user-1",
    createdAt: "2026-03-26T12:00:00.000Z",
    updatedAt: "2026-03-26T12:00:00.000Z",
    deletedAt: null,
    ...overrides,
  };
}

describe("TemplatePicker", () => {
  it("renders available templates and supports picking or closing the dialog", async () => {
    const user = userEvent.setup();
    const onOpenChange = jest.fn();
    const onPick = jest.fn();

    render(
      <TemplatePicker
        open
        onOpenChange={onOpenChange}
        templates={[makeTemplate()]}
        onPick={onPick}
      />,
    );

    expect(screen.getByText("Select a template")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Incident Runbook/i }));
    expect(onPick).toHaveBeenCalledWith("template-1");

    await user.click(screen.getAllByRole("button", { name: "Close" })[0]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("shows an empty state when there are no templates to pick", () => {
    render(
      <TemplatePicker open onOpenChange={jest.fn()} templates={[]} onPick={jest.fn()} />,
    );

    expect(screen.getByText("No templates available yet.")).toBeInTheDocument();
  });
});
