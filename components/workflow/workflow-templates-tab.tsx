"use client"

import { useEffect, useMemo, useState } from "react"
import { useTranslations } from "next-intl"
import { LayoutTemplate, Search } from "lucide-react"
import { toast } from "sonner"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { EmptyState } from "@/components/shared/empty-state"
import { cn } from "@/lib/utils"
import { useWorkflowStore, type WorkflowDefinition } from "@/lib/stores/workflow-store"
import { WorkflowTemplateVarsDialog } from "./workflow-template-vars-dialog"

interface WorkflowTemplatesTabProps {
  projectId: string
  setActiveTab: (tab: string) => void
}

function categoryBadgeClass(category: string): string {
  switch (category) {
    case "system":
      return "bg-blue-500/15 text-blue-700 dark:text-blue-400"
    case "user":
      return "bg-green-500/15 text-green-700 dark:text-green-400"
    case "marketplace":
      return "bg-amber-500/15 text-amber-700 dark:text-amber-400"
    default:
      return ""
  }
}

export function WorkflowTemplatesTab({
  projectId,
  setActiveTab,
}: WorkflowTemplatesTabProps) {
  const t = useTranslations("workflow")
  const {
    templates,
    templatesLoading,
    fetchTemplates,
    cloneTemplate,
    executeTemplate,
    duplicateTemplate,
    deleteTemplate,
    selectDefinition,
  } = useWorkflowStore()

  const sourceFilters = [
    { value: "all", label: t("templates.source.all") },
    { value: "system", label: t("templates.source.system") },
    { value: "user", label: t("templates.source.user") },
    { value: "marketplace", label: t("templates.source.marketplace") },
  ] as const

  const [sourceFilter, setSourceFilter] = useState<(typeof sourceFilters)[number]["value"]>("all")
  const [query, setQuery] = useState("")
  const [dialogOpen, setDialogOpen] = useState(false)
  const [dialogTemplate, setDialogTemplate] = useState<WorkflowDefinition | null>(null)
  const [dialogMode, setDialogMode] = useState<"clone" | "execute">("clone")
  const [dialogLoading, setDialogLoading] = useState(false)
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>("")

  useEffect(() => {
    void fetchTemplates(projectId)
  }, [fetchTemplates, projectId])

  const filteredTemplates = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    return templates.filter((template) => {
      if (sourceFilter !== "all" && template.category !== sourceFilter) {
        return false
      }
      if (!normalizedQuery) {
        return true
      }
      const haystack = [
        template.name,
        template.description,
        template.category,
        Object.keys(template.templateVars ?? {}).join(" "),
      ]
        .join(" ")
        .toLowerCase()
      return haystack.includes(normalizedQuery)
    })
  }, [query, sourceFilter, templates])

  const selectedTemplate =
    filteredTemplates.find((template) => template.id === selectedTemplateId) ??
    filteredTemplates[0] ??
    null

  async function handleClone(overrides: Record<string, unknown>) {
    if (!dialogTemplate) return
    setDialogLoading(true)
    const def = await cloneTemplate(dialogTemplate.id, projectId, overrides)
    setDialogLoading(false)
    if (def) {
      toast.success(t("templates.toast.cloned"))
      setDialogOpen(false)
      selectDefinition(def)
      setActiveTab("workflows")
    }
  }

  async function handleExecute(overrides: Record<string, unknown>, taskId?: string) {
    if (!dialogTemplate) return
    setDialogLoading(true)
    const exec = await executeTemplate(dialogTemplate.id, projectId, taskId, overrides)
    setDialogLoading(false)
    if (exec) {
      toast.success(t("templates.toast.started"))
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
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("templates.searchPlaceholder")}
          />
        </div>
        <div className="flex flex-wrap gap-2">
          {sourceFilters.map((filter) => (
            <Button
              key={filter.value}
              variant={sourceFilter === filter.value ? "default" : "outline"}
              size="sm"
              onClick={() => setSourceFilter(filter.value)}
            >
              {filter.label}
            </Button>
          ))}
        </div>
      </div>

      {templatesLoading && (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
          <div className="grid gap-4 md:grid-cols-2">
            <Skeleton className="h-40 rounded-lg" />
            <Skeleton className="h-40 rounded-lg" />
          </div>
          <Skeleton className="h-64 rounded-lg" />
        </div>
      )}

      {!templatesLoading && filteredTemplates.length === 0 && (
        <EmptyState
          icon={LayoutTemplate}
          title={t("templates.noTemplates")}
          description={t("templates.noTemplatesDesc")}
        />
      )}

      {!templatesLoading && filteredTemplates.length > 0 && (
        <div className="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
          <div className="grid gap-4 md:grid-cols-2">
            {filteredTemplates.map((template) => {
              const selected = template.id === selectedTemplate?.id
              const varKeys = Object.keys(template.templateVars ?? {})

              return (
                <button
                  key={template.id}
                  type="button"
                  className={`rounded-lg border text-left transition-colors ${
                    selected
                      ? "border-primary bg-primary/5"
                      : "border-border/60 hover:bg-accent/40"
                  }`}
                  onClick={() => setSelectedTemplateId(template.id)}
                >
                  <Card className="border-0 bg-transparent shadow-none">
                    <CardHeader className="pb-2">
                      <div className="flex items-start justify-between gap-2">
                        <CardTitle className="text-sm">{template.name}</CardTitle>
                        <Badge
                          className={cn(
                            "shrink-0 border-0 text-xs font-medium",
                            categoryBadgeClass(template.category),
                          )}
                        >
                          {template.templateSource ?? template.category}
                        </Badge>
                      </div>
                      <CardDescription className="text-xs">{template.description}</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-3">
                      <div className="flex gap-3 text-xs text-muted-foreground">
                        <span>{t("templates.nodes", { count: template.nodes.length })}</span>
                        <span>{t("templates.edges", { count: template.edges.length })}</span>
                      </div>
                      <p className="truncate text-xs text-muted-foreground">
                        {varKeys.length > 0 ? varKeys.join(", ") : t("templates.noVariables")}
                      </p>
                    </CardContent>
                  </Card>
                </button>
              )
            })}
          </div>

          <div className="rounded-lg border border-border/60 bg-background/70 p-4">
            {selectedTemplate ? (
              <div className="space-y-4">
                <div>
                  <h3 className="text-sm font-semibold">{t("templates.preview")}</h3>
                  <div className="mt-2 font-medium">{selectedTemplate.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {selectedTemplate.templateSource ?? selectedTemplate.category} · v{selectedTemplate.version}
                  </div>
                  <p className="mt-2 text-sm text-muted-foreground">{selectedTemplate.description}</p>
                </div>

                <div className="grid gap-2 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">{t("templates.nodes", { count: selectedTemplate.nodes.length })}</span>
                    <span>{selectedTemplate.nodes.length}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">{t("templates.edges", { count: selectedTemplate.edges.length })}</span>
                    <span>{selectedTemplate.edges.length}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">{t("templates.variables")}</span>
                    <span>{Object.keys(selectedTemplate.templateVars ?? {}).length}</span>
                  </div>
                </div>

                <div className="flex flex-wrap gap-2">
                  {(selectedTemplate.canDuplicate ?? true) && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={async () => {
                        const duplicated = await duplicateTemplate(selectedTemplate.id, projectId, {
                          name: `${selectedTemplate.name} ${t("templates.copySuffix")}`,
                          description: selectedTemplate.description,
                        })
                        if (duplicated) {
                          toast.success(t("templates.toast.duplicated"))
                        }
                      }}
                    >
                      {t("templates.actions.duplicate")}
                    </Button>
                  )}
                  {(selectedTemplate.canEdit ?? false) && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        selectDefinition(selectedTemplate)
                        setActiveTab("workflows")
                      }}
                    >
                      {t("templates.actions.edit")}
                    </Button>
                  )}
                  {(selectedTemplate.canDelete ?? false) && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={async () => {
                        const ok = await deleteTemplate(selectedTemplate.id, projectId)
                        if (ok) {
                          toast.success(t("templates.toast.deleted"))
                        }
                      }}
                    >
                      {t("templates.actions.delete")}
                    </Button>
                  )}
                  {(selectedTemplate.canClone ?? true) && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setDialogTemplate(selectedTemplate)
                        setDialogMode("clone")
                        setDialogOpen(true)
                      }}
                    >
                      {t("templates.actions.clone")}
                    </Button>
                  )}
                  {(selectedTemplate.canExecute ?? true) && (
                    <Button
                      size="sm"
                      onClick={() => {
                        setDialogTemplate(selectedTemplate)
                        setDialogMode("execute")
                        setDialogOpen(true)
                      }}
                    >
                      {t("templates.actions.execute")}
                    </Button>
                  )}
                </div>
              </div>
            ) : (
              <EmptyState
                icon={LayoutTemplate}
                title={t("templates.selectTitle")}
                description={t("templates.selectDesc")}
              />
            )}
          </div>
        </div>
      )}

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
