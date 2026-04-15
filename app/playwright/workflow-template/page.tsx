"use client"

import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { WorkflowTemplatesTab } from "@/components/workflow/workflow-templates-tab"
import { captureStoreSnapshot } from "@/lib/playwright-harness"
import { useAuthStore } from "@/lib/stores/auth-store"
import {
  type WorkflowDefinition,
  useWorkflowStore,
} from "@/lib/stores/workflow-store"

const projectId = "playwright-project"

function makeDefinition(overrides: Partial<WorkflowDefinition> = {}): WorkflowDefinition {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    projectId: overrides.projectId ?? projectId,
    name: overrides.name ?? "Delivery Flow",
    description: overrides.description ?? "Project workflow definition",
    status: overrides.status ?? "active",
    category: overrides.category ?? "user",
    nodes: overrides.nodes ?? [],
    edges: overrides.edges ?? [],
    templateVars: overrides.templateVars ?? { runtime: "claude_code" },
    version: overrides.version ?? 1,
    sourceId: overrides.sourceId,
    createdAt: overrides.createdAt ?? "2026-04-15T00:00:00.000Z",
    updatedAt: overrides.updatedAt ?? "2026-04-15T00:00:00.000Z",
    templateSource: overrides.templateSource,
    canEdit: overrides.canEdit,
    canDelete: overrides.canDelete,
    canDuplicate: overrides.canDuplicate,
    canClone: overrides.canClone,
    canExecute: overrides.canExecute,
  }
}

export default function WorkflowTemplatePlaywrightPage() {
  const [activeTab, setActiveTab] = useState("templates")
  const definitions = useWorkflowStore((state) => state.definitions)
  const executions = useWorkflowStore((state) => state.executions)
  const publishTemplate = useWorkflowStore((state) => state.publishTemplate)

  useEffect(() => {
    const restoreAuthStore = captureStoreSnapshot(useAuthStore)
    const restoreWorkflowStore = captureStoreSnapshot(useWorkflowStore)

    useAuthStore.setState({
      accessToken: "playwright-token",
      refreshToken: "playwright-refresh",
      user: {
        id: "playwright-user",
        email: "playwright@example.com",
        name: "Playwright User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never)

    const systemTemplate = makeDefinition({
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
    })
    const baseDefinition = makeDefinition({
      id: "workflow-definition",
      name: "Delivery Flow",
      description: "Project workflow definition",
      status: "active",
      category: "user",
    })

    useWorkflowStore.setState({
      definitions: [baseDefinition],
      templates: [systemTemplate],
      templatesLoading: false,
      executions: [],
      selectedDefinition: null,
      error: null,
      fetchTemplates: async () => {
        useWorkflowStore.setState((state) => ({
          templatesLoading: false,
          templates: state.templates,
        }))
      },
      publishTemplate: async (definitionId, nextProjectId, data) => {
        const template = makeDefinition({
          id: "template-custom",
          projectId: nextProjectId,
          name: data?.name ?? "Delivery Flow Template",
          description: data?.description ?? "Published from a workflow",
          status: "template",
          category: "user",
          templateSource: "user",
          sourceId: definitionId,
          canEdit: true,
          canDelete: true,
          canDuplicate: true,
          canClone: true,
          canExecute: true,
        })
        useWorkflowStore.setState((state) => ({
          templates: [template, ...state.templates.filter((item) => item.id !== template.id)],
        }))
        return template
      },
      duplicateTemplate: async (templateId, nextProjectId, data) => {
        const template = makeDefinition({
          id: "template-custom-copy",
          projectId: nextProjectId,
          name: data?.name ?? "Delivery Flow Template Copy",
          description: data?.description ?? "Duplicate",
          status: "template",
          category: "user",
          templateSource: "user",
          sourceId: templateId,
          canEdit: true,
          canDelete: true,
          canDuplicate: true,
          canClone: true,
          canExecute: true,
        })
        useWorkflowStore.setState((state) => ({
          templates: [template, ...state.templates],
        }))
        return template
      },
      deleteTemplate: async (templateId) => {
        useWorkflowStore.setState((state) => ({
          templates: state.templates.filter((template) => template.id !== templateId),
        }))
        return true
      },
      cloneTemplate: async (templateId) => {
        const definition = makeDefinition({
          id: "workflow-copy",
          name: "Workflow Copy",
          description: "Created from template",
          status: "active",
          category: "user",
          sourceId: templateId,
        })
        useWorkflowStore.setState((state) => ({
          definitions: [definition, ...state.definitions],
        }))
        return definition
      },
      executeTemplate: async (templateId, nextProjectId, taskId) => {
        const execution = {
          id: "execution-1",
          workflowId: templateId,
          projectId: nextProjectId,
          taskId,
          status: "running",
          currentNodes: [],
          createdAt: "2026-04-15T00:05:00.000Z",
          updatedAt: "2026-04-15T00:05:00.000Z",
        }
        useWorkflowStore.setState((state) => ({
          executions: [execution, ...state.executions],
        }))
        return execution
      },
      selectDefinition: (definition) => {
        useWorkflowStore.setState({ selectedDefinition: definition })
      },
    })

    return () => {
      restoreWorkflowStore()
      restoreAuthStore()
    }
  }, [])

  return (
    <section className="space-y-6">
      <div className="rounded-xl border border-border/60 bg-card/70 p-4">
        <div className="flex flex-wrap items-center gap-3">
          <Button
            onClick={() => {
              void publishTemplate("workflow-definition", projectId, {
                name: "Delivery Flow Template",
                description: "Published from current workflow",
              })
            }}
          >
            Publish Delivery Flow
          </Button>
          <span data-testid="workflow-active-tab">Active Tab: {activeTab}</span>
          <span data-testid="workflow-definition-count">Definitions: {definitions.length}</span>
          <span data-testid="workflow-execution-count">Executions: {executions.length}</span>
        </div>
      </div>

      <WorkflowTemplatesTab projectId={projectId} setActiveTab={setActiveTab} />
    </section>
  )
}
