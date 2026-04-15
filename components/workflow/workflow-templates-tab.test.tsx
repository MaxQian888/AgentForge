import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { WorkflowTemplatesTab } from "./workflow-templates-tab"

const fetchTemplates = jest.fn()
const cloneTemplate = jest.fn()
const executeTemplate = jest.fn()
const duplicateTemplate = jest.fn()
const deleteTemplate = jest.fn()
const selectDefinition = jest.fn()

const workflowStoreState = {
  templates: [
    {
      id: "template-system",
      projectId: "00000000-0000-0000-0000-000000000000",
      name: "Plan Code Review",
      description: "System workflow template",
      status: "template",
      category: "system",
      templateSource: "system",
      canDuplicate: true,
      canClone: true,
      canExecute: true,
      nodes: [],
      edges: [],
      templateVars: { runtime: "claude_code" },
      version: 1,
      createdAt: "2026-04-15T00:00:00.000Z",
      updatedAt: "2026-04-15T00:00:00.000Z",
    },
    {
      id: "template-custom",
      projectId: "project-1",
      name: "Project Delivery",
      description: "Custom workflow template",
      status: "template",
      category: "user",
      templateSource: "user",
      canEdit: true,
      canDelete: true,
      canDuplicate: true,
      canClone: true,
      canExecute: true,
      nodes: [],
      edges: [],
      templateVars: { model: "gpt-5" },
      version: 1,
      createdAt: "2026-04-15T00:01:00.000Z",
      updatedAt: "2026-04-15T00:01:00.000Z",
    },
  ],
  templatesLoading: false,
  fetchTemplates,
  cloneTemplate,
  executeTemplate,
  duplicateTemplate,
  deleteTemplate,
  selectDefinition,
}

jest.mock("@/lib/stores/workflow-store", () => ({
  useWorkflowStore: () => workflowStoreState,
}))

describe("WorkflowTemplatesTab", () => {
  beforeEach(() => {
    jest.clearAllMocks()
    cloneTemplate.mockResolvedValue({ id: "workflow-1" })
    executeTemplate.mockResolvedValue({ id: "execution-1" })
    duplicateTemplate.mockResolvedValue({ id: "template-copy" })
    deleteTemplate.mockResolvedValue(true)
  })

  it("loads templates, filters them, and exposes manage actions", async () => {
    const user = userEvent.setup()
    const setActiveTab = jest.fn()

    render(<WorkflowTemplatesTab projectId="project-1" setActiveTab={setActiveTab} />)

    await waitFor(() => {
      expect(fetchTemplates).toHaveBeenCalledWith("project-1")
    })

    await user.click(screen.getByRole("button", { name: "Custom" }))
    expect(screen.queryByText("Plan Code Review")).not.toBeInTheDocument()
    expect(screen.getAllByText("Project Delivery").length).toBeGreaterThan(0)

    await user.click(screen.getByRole("button", { name: "Duplicate" }))
    expect(duplicateTemplate).toHaveBeenCalledWith("template-custom", "project-1", {
      name: "Project Delivery Copy",
      description: "Custom workflow template",
    })

    await user.click(screen.getByRole("button", { name: "Edit" }))
    expect(selectDefinition).toHaveBeenCalledWith(
      expect.objectContaining({ id: "template-custom" }),
    )
    expect(setActiveTab).toHaveBeenCalledWith("workflows")

    await user.click(screen.getByRole("button", { name: "Delete" }))
    expect(deleteTemplate).toHaveBeenCalledWith("template-custom", "project-1")
  })
})
