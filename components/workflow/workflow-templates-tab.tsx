"use client"

import { useEffect, useState } from "react"
import { LayoutTemplate } from "lucide-react"
import { toast } from "sonner"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { EmptyState } from "@/components/shared/empty-state"
import { cn } from "@/lib/utils"
import {
  useWorkflowStore,
  type WorkflowDefinition,
} from "@/lib/stores/workflow-store"
import { WorkflowTemplateVarsDialog } from "./workflow-template-vars-dialog"

interface WorkflowTemplatesTabProps {
  projectId: string
  setActiveTab: (tab: string) => void
}

const CATEGORY_FILTERS = [
  { value: "all", label: "All" },
  { value: "system", label: "System" },
  { value: "user", label: "User" },
  { value: "marketplace", label: "Marketplace" },
]

function categoryBadgeClass(category: string): string {
  switch (category) {
    case "system":
      return "bg-blue-500/15 text-blue-700 dark:text-blue-400"
    case "user":
      return "bg-green-500/15 text-green-700 dark:text-green-400"
    case "marketplace":
      return "bg-purple-500/15 text-purple-700 dark:text-purple-400"
    default:
      return ""
  }
}

export function WorkflowTemplatesTab({
  projectId,
  setActiveTab,
}: WorkflowTemplatesTabProps) {
  const { templates, templatesLoading, fetchTemplates, cloneTemplate, selectDefinition, executeTemplate } =
    useWorkflowStore()

  const [categoryFilter, setCategoryFilter] = useState("all")
  const [dialogOpen, setDialogOpen] = useState(false)
  const [dialogTemplate, setDialogTemplate] = useState<WorkflowDefinition | null>(null)
  const [dialogMode, setDialogMode] = useState<"clone" | "execute">("clone")
  const [dialogLoading, setDialogLoading] = useState(false)

  useEffect(() => {
    fetchTemplates()
  }, [fetchTemplates])

  const filtered = templates.filter(
    (t) => categoryFilter === "all" || t.category === categoryFilter
  )

  async function handleClone(overrides: Record<string, unknown>) {
    setDialogLoading(true)
    const def = await cloneTemplate(dialogTemplate!.id, projectId, overrides)
    setDialogLoading(false)
    if (def) {
      toast.success("Template cloned")
      setDialogOpen(false)
      selectDefinition(def)
      setActiveTab("workflows")
    }
  }

  async function handleExecute(overrides: Record<string, unknown>, taskId?: string) {
    setDialogLoading(true)
    const exec = await executeTemplate(dialogTemplate!.id, projectId, taskId, overrides)
    setDialogLoading(false)
    if (exec) {
      toast.success("Execution started")
      setDialogOpen(false)
      setActiveTab("executions")
    }
  }

  function handleSubmit(overrides: Record<string, unknown>, taskId?: string): Promise<void> {
    if (dialogMode === "clone") {
      return handleClone(overrides)
    }
    return handleExecute(overrides, taskId)
  }

  return (
    <div className="space-y-4">
      {/* Category filter bar */}
      <div className="flex flex-wrap gap-2">
        {CATEGORY_FILTERS.map((f) => (
          <Button
            key={f.value}
            variant={categoryFilter === f.value ? "default" : "outline"}
            size="sm"
            onClick={() => setCategoryFilter(f.value)}
          >
            {f.label}
          </Button>
        ))}
      </div>

      {/* Loading state */}
      {templatesLoading && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <Skeleton className="h-40 rounded-lg" />
          <Skeleton className="h-40 rounded-lg" />
          <Skeleton className="h-40 rounded-lg" />
        </div>
      )}

      {/* Empty state */}
      {!templatesLoading && filtered.length === 0 && (
        <EmptyState
          icon={LayoutTemplate}
          title="No templates found"
          description="No workflow templates match the selected category."
        />
      )}

      {/* Card grid */}
      {!templatesLoading && filtered.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((template) => {
            const varKeys = Object.keys(template.templateVars ?? {})
            return (
              <Card key={template.id}>
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between gap-2">
                    <CardTitle className="text-sm">{template.name}</CardTitle>
                    <Badge
                      className={cn(
                        "shrink-0 text-xs font-medium border-0",
                        categoryBadgeClass(template.category)
                      )}
                    >
                      {template.category}
                    </Badge>
                  </div>
                  <CardDescription className="text-xs">
                    {template.description}
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  {/* Stats */}
                  <div className="flex gap-3 text-xs text-muted-foreground">
                    <span>{template.nodes.length} nodes</span>
                    <span>{template.edges.length} edges</span>
                  </div>

                  {/* Template vars preview */}
                  <p className="text-xs text-muted-foreground truncate">
                    {varKeys.length > 0
                      ? varKeys.join(", ")
                      : "No variables"}
                  </p>

                  {/* Actions */}
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setDialogTemplate(template)
                        setDialogMode("clone")
                        setDialogOpen(true)
                      }}
                    >
                      Clone
                    </Button>
                    <Button
                      size="sm"
                      onClick={() => {
                        setDialogTemplate(template)
                        setDialogMode("execute")
                        setDialogOpen(true)
                      }}
                    >
                      Execute
                    </Button>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}

      {/* Template vars dialog */}
      {dialogTemplate && (
        <WorkflowTemplateVarsDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          template={dialogTemplate}
          mode={dialogMode}
          onSubmit={handleSubmit}
          loading={dialogLoading}
        />
      )}
    </div>
  )
}
