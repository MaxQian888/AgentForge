import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { WorkflowTemplateVarsDialog } from "./workflow-template-vars-dialog"

const template = {
  id: "template-1",
  projectId: "project-1",
  name: "Plan Code Review",
  description: "System template",
  status: "template",
  category: "system",
  nodes: [],
  edges: [],
  templateVars: {
    runtime: "claude_code",
  },
  version: 1,
  createdAt: "2026-04-15T00:00:00.000Z",
  updatedAt: "2026-04-15T00:00:00.000Z",
}

describe("WorkflowTemplateVarsDialog", () => {
  it("clarifies clone mode as creating a project-owned copy", async () => {
    const user = userEvent.setup()
    const onSubmit = jest.fn().mockResolvedValue(undefined)

    render(
      <WorkflowTemplateVarsDialog
        open
        onOpenChange={jest.fn()}
        template={template}
        mode="clone"
        onSubmit={onSubmit}
        loading={false}
      />,
    )

    expect(screen.getByText("Create Workflow Copy: Plan Code Review")).toBeInTheDocument()
    expect(
      screen.getByText(
        "This creates a project-owned workflow definition you can continue editing safely.",
      ),
    ).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "Create Workflow Copy" }))

    expect(onSubmit).toHaveBeenCalledWith({ runtime: "claude_code" }, undefined)
  })
})
