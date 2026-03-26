import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { DocsPage } from "@/lib/stores/docs-store";
import { TemplateCenter } from "./template-center";

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

describe("TemplateCenter", () => {
  it("renders templates and emits the selected template id", async () => {
    const user = userEvent.setup();
    const onCreateFromTemplate = jest.fn();

    render(
      <TemplateCenter
        templates={[makeTemplate(), makeTemplate({ id: "template-2", isSystem: false })]}
        onCreateFromTemplate={onCreateFromTemplate}
      />,
    );

    expect(screen.getByText("Template Center")).toBeInTheDocument();
    expect(screen.getByText("runbook · System")).toBeInTheDocument();
    expect(screen.getByText("runbook · Custom")).toBeInTheDocument();

    await user.click(screen.getAllByRole("button", { name: "Use Template" })[1]);
    expect(onCreateFromTemplate).toHaveBeenCalledWith("template-2");
  });

  it("shows an empty message when templates have not loaded yet", () => {
    render(<TemplateCenter templates={[]} />);

    expect(
      screen.getByText("Templates will appear here after they load."),
    ).toBeInTheDocument();
  });
});
