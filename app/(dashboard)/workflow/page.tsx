"use client";

import { Suspense, useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  FolderOpen,
  Plus,
  Play,
  Pencil,
  Trash2,
  Clock,
  CheckCircle2,
  XCircle,
  Loader2,
  LayoutTemplate,
} from "lucide-react";
import { toast } from "sonner";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import {
  useWorkflowStore,
  type WorkflowDefinition,
} from "@/lib/stores/workflow-store";
import { WorkflowConfigPanel } from "@/components/workflow/workflow-config-panel";
import { WorkflowEditor } from "@/components/workflow-editor";
import { WorkflowExecutionView } from "@/components/workflow/workflow-execution-view";
import { WorkflowReviewsTab } from "@/components/workflow/workflow-reviews-tab";
import { WorkflowRunsTab } from "@/components/workflow/workflow-runs-tab";
import { WorkflowTemplatesTab } from "@/components/workflow/workflow-templates-tab";
import { WorkflowTriggersSection } from "@/components/workflow/workflow-triggers-section";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

function CreateWorkflowDialog({
  projectId,
  onCreated,
}: {
  projectId: string;
  onCreated: (def: WorkflowDefinition) => void;
}) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const { createDefinition, saving } = useWorkflowStore();

  const handleCreate = useCallback(async () => {
    if (!name.trim()) {
      toast.error("Name is required");
      return;
    }
    const def = await createDefinition(projectId, {
      name: name.trim(),
      description: description.trim(),
      nodes: [],
      edges: [],
    });
    if (def) {
      toast.success("Workflow created");
      setOpen(false);
      setName("");
      setDescription("");
      onCreated(def);
    }
  }, [name, description, projectId, createDefinition, onCreated]);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="h-4 w-4 mr-1" />
          New Workflow
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Workflow</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="wf-name">Name</Label>
            <Input
              id="wf-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Workflow"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="wf-desc">Description</Label>
            <Input
              id="wf-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What does this workflow do?"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            Cancel
          </Button>
          <Button onClick={handleCreate} disabled={saving}>
            {saving ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function PublishTemplateDialog({
  definition,
  projectId,
  onPublished,
}: {
  definition: WorkflowDefinition;
  projectId: string;
  onPublished: (template: WorkflowDefinition) => void;
}) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(definition.name);
  const [description, setDescription] = useState(definition.description);
  const { publishTemplate, saving } = useWorkflowStore();

  const handlePublish = useCallback(async () => {
    const template = await publishTemplate(definition.id, projectId, {
      name: name.trim() || definition.name,
      description: description.trim() || definition.description,
    });
    if (template) {
      toast.success("Template published");
      setOpen(false);
      onPublished(template);
    }
  }, [
    definition.description,
    definition.id,
    definition.name,
    description,
    name,
    onPublished,
    projectId,
    publishTemplate,
  ]);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" className="h-7 w-7" title="Publish as template">
          <LayoutTemplate className="h-3.5 w-3.5" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Publish as Template</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor={`publish-template-name-${definition.id}`}>Template Name</Label>
            <Input
              id={`publish-template-name-${definition.id}`}
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`publish-template-description-${definition.id}`}>Description</Label>
            <Input
              id={`publish-template-description-${definition.id}`}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            Cancel
          </Button>
          <Button onClick={handlePublish} disabled={saving}>
            {saving ? "Publishing..." : "Publish"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function WorkflowListTab({
  projectId,
  setActiveTab,
}: {
  projectId: string;
  setActiveTab: (tab: string) => void;
}) {
  const isDirtyRef = useRef(false);
  const {
    definitions,
    definitionsLoading,
    fetchDefinitions,
    deleteDefinition,
    selectDefinition,
    selectedDefinition,
    startExecution,
    updateDefinition,
  } = useWorkflowStore();

  useEffect(() => {
    fetchDefinitions(projectId);
  }, [projectId, fetchDefinitions]);

  const handleDelete = useCallback(
    async (id: string) => {
      if (!confirm("Delete this workflow?")) return;
      const ok = await deleteDefinition(id);
      if (ok) toast.success("Workflow deleted");
    },
    [deleteDefinition]
  );

  const handleExecute = useCallback(
    async (id: string) => {
      const exec = await startExecution(id);
      if (exec) {
        toast.success("Execution started");
      }
    },
    [startExecution]
  );

  const handleActivate = useCallback(
    async (def: WorkflowDefinition) => {
      const newStatus =
        def.status === "active" ? "draft" : "active";
      const ok = await updateDefinition(def.id, { status: newStatus });
      if (ok) toast.success(`Workflow ${newStatus === "active" ? "activated" : "deactivated"}`);
      fetchDefinitions(projectId);
    },
    [updateDefinition, fetchDefinitions, projectId]
  );

  if (selectedDefinition) {
    return (
      <div className="flex flex-col gap-4 h-[calc(100vh-240px)]">
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              if (isDirtyRef.current && !confirm("You have unsaved changes. Discard and leave?")) return;
              selectDefinition(null);
            }}
          >
            Back to list
          </Button>
          <span className="text-sm text-muted-foreground">
            / {selectedDefinition.name}
          </span>
          {selectedDefinition.status !== "template" ? (
            <PublishTemplateDialog
              definition={selectedDefinition}
              projectId={projectId}
              onPublished={() => setActiveTab("templates")}
            />
          ) : null}
        </div>
        <WorkflowEditor
          definition={selectedDefinition}
          onSave={async (data) => {
            const ok = await updateDefinition(selectedDefinition.id, data);
            if (ok) toast.success("Workflow saved");
            return ok;
          }}
          onExecute={handleExecute}
          onDirtyChange={(dirty) => {
            // Store dirty state for navigation guard
            isDirtyRef.current = dirty;
          }}
        />
      </div>
    );
  }

  if (definitionsLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground p-4">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading workflows...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-muted-foreground">
          {definitions.length} workflow{definitions.length !== 1 ? "s" : ""}
        </h3>
        <CreateWorkflowDialog
          projectId={projectId}
          onCreated={(def) => {
            selectDefinition(def);
          }}
        />
      </div>

      {definitions.length === 0 ? (
        <EmptyState
          icon={FolderOpen}
          title="No workflows"
          description="Create a workflow to automate your task pipeline."
        />
      ) : (
        <div className="grid gap-3">
          {definitions.map((def) => (
            <Card key={def.id} className="group">
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <CardTitle className="text-sm">{def.name}</CardTitle>
                    <Badge
                      variant={
                        def.status === "active" ? "default" : "secondary"
                      }
                      className="text-[10px]"
                    >
                      {def.status}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    {def.status !== "template" ? (
                      <PublishTemplateDialog
                        definition={def}
                        projectId={projectId}
                        onPublished={() => setActiveTab("templates")}
                      />
                    ) : null}
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => handleActivate(def)}
                      title={
                        def.status === "active"
                          ? "Deactivate"
                          : "Activate"
                      }
                    >
                      {def.status === "active" ? (
                        <XCircle className="h-3.5 w-3.5" />
                      ) : (
                        <CheckCircle2 className="h-3.5 w-3.5" />
                      )}
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => selectDefinition(def)}
                    >
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => handleExecute(def.id)}
                      disabled={def.status !== "active"}
                    >
                      <Play className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-destructive"
                      onClick={() => handleDelete(def.id)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
                {def.description && (
                  <CardDescription className="text-xs">
                    {def.description}
                  </CardDescription>
                )}
              </CardHeader>
              <CardContent className="pt-0">
                <div className="flex items-center gap-3 text-xs text-muted-foreground">
                  <span>
                    {(def.nodes ?? []).length} node
                    {(def.nodes ?? []).length !== 1 ? "s" : ""}
                  </span>
                  <span>
                    {(def.edges ?? []).length} edge
                    {(def.edges ?? []).length !== 1 ? "s" : ""}
                  </span>
                  <span>
                    Updated{" "}
                    {new Date(def.updatedAt).toLocaleDateString()}
                  </span>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

function ExecutionsTab({ projectId }: { projectId: string }) {
  const {
    definitions,
    fetchDefinitions,
    executions,
    executionsLoading,
    fetchExecutions,
    cancelExecution,
  } = useWorkflowStore();

  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(
    null
  );
  const [selectedExecId, setSelectedExecId] = useState<string | null>(null);

  useEffect(() => {
    fetchDefinitions(projectId);
  }, [projectId, fetchDefinitions]);

  useEffect(() => {
    if (selectedWorkflowId) {
      fetchExecutions(selectedWorkflowId);
    }
  }, [selectedWorkflowId, fetchExecutions]);

  const selectedDef = definitions.find((d) => d.id === selectedWorkflowId);

  if (selectedExecId && selectedDef) {
    return (
      <div className="space-y-4">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setSelectedExecId(null)}
        >
          Back to executions
        </Button>
        <WorkflowExecutionView
          executionId={selectedExecId}
          nodes={selectedDef.nodes}
          edges={selectedDef.edges}
        />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Label className="text-sm">Workflow:</Label>
        <select
          className="border rounded px-2 py-1 text-sm bg-background"
          value={selectedWorkflowId ?? ""}
          onChange={(e) =>
            setSelectedWorkflowId(e.target.value || null)
          }
        >
          <option value="">Select a workflow</option>
          {definitions.map((def) => (
            <option key={def.id} value={def.id}>
              {def.name}
            </option>
          ))}
        </select>
      </div>

      {!selectedWorkflowId && (
        <EmptyState
          icon={Clock}
          title="Select a workflow"
          description="Choose a workflow above to view its execution history."
        />
      )}

      {selectedWorkflowId && executionsLoading && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading executions...
        </div>
      )}

      {selectedWorkflowId && !executionsLoading && executions.length === 0 && (
        <EmptyState
          icon={Clock}
          title="No executions"
          description="This workflow has not been executed yet."
        />
      )}

      {selectedWorkflowId && !executionsLoading && executions.length > 0 && (
        <div className="grid gap-2">
          {executions.map((exec) => (
            <Card
              key={exec.id}
              className="cursor-pointer hover:bg-muted/50 transition-colors"
              onClick={() => setSelectedExecId(exec.id)}
            >
              <CardContent className="p-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Badge
                      variant={
                        exec.status === "completed"
                          ? "default"
                          : exec.status === "failed"
                            ? "destructive"
                            : "secondary"
                      }
                      className={cn(
                        "text-[10px]",
                        exec.status === "completed" &&
                          "bg-green-500/15 text-green-700 dark:text-green-400",
                        exec.status === "running" && "animate-pulse"
                      )}
                    >
                      {exec.status}
                    </Badge>
                    <span className="text-xs text-muted-foreground font-mono">
                      {exec.id.slice(0, 8)}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    {exec.startedAt && (
                      <span className="text-xs text-muted-foreground">
                        {new Date(exec.startedAt).toLocaleString()}
                      </span>
                    )}
                    {(exec.status === "running" ||
                      exec.status === "pending") && (
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={async (e) => {
                          e.stopPropagation();
                          await cancelExecution(exec.id);
                          toast.success("Execution cancelled");
                          fetchExecutions(selectedWorkflowId);
                        }}
                      >
                        <XCircle className="h-3.5 w-3.5 text-destructive" />
                      </Button>
                    )}
                  </div>
                </div>
                {exec.errorMessage && (
                  <p className="text-xs text-destructive mt-1 truncate">
                    {exec.errorMessage}
                  </p>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

function TriggersTab({ projectId }: { projectId: string }) {
  const { definitions, fetchDefinitions } = useWorkflowStore();
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(null);

  useEffect(() => {
    fetchDefinitions(projectId);
  }, [projectId, fetchDefinitions]);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Label className="text-sm">Workflow:</Label>
        <select
          className="border rounded px-2 py-1 text-sm bg-background"
          value={selectedWorkflowId ?? ""}
          onChange={(e) => setSelectedWorkflowId(e.target.value || null)}
        >
          <option value="">Select a workflow</option>
          {definitions.map((def) => (
            <option key={def.id} value={def.id}>
              {def.name}
            </option>
          ))}
        </select>
      </div>
      <WorkflowTriggersSection workflowId={selectedWorkflowId} projectId={projectId} />
    </div>
  );
}

function WorkflowPageContent() {
  const tc = useTranslations("common");
  useBreadcrumbs([{ label: tc("nav.group.operations"), href: "/" }, { label: tc("nav.workflow") }]);
  const t = useTranslations("workflow");
  const searchParams = useSearchParams();
  const requestedProjectId = searchParams.get("project");
  const requestedTab = searchParams.get("tab");
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);
  const activeProjectId = requestedProjectId ?? selectedProjectId;
  const requestedTabSeed = requestedTab ?? "__default__";
  const [manualActiveTab, setManualActiveTab] = useState<string | null>(null);
  const [manualTabSeed, setManualTabSeed] = useState<string | null>(null);
  const activeTab =
    manualTabSeed === requestedTabSeed
      ? manualActiveTab ?? requestedTab ?? "workflows"
      : requestedTab ?? "workflows";
  const handleActiveTabChange = useCallback(
    (nextTab: string) => {
      setManualTabSeed(requestedTabSeed);
      setManualActiveTab(nextTab);
    },
    [requestedTabSeed],
  );

  if (!activeProjectId) {
    return (
      <div className="flex flex-col gap-[var(--space-section-gap)]">
        <PageHeader title={t("title")} />
        <EmptyState icon={FolderOpen} title={t("selectProject")} />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title={t("title")} />
      <Tabs value={activeTab} onValueChange={handleActiveTabChange} className="w-full">
        <TabsList>
          <TabsTrigger value="config">Config</TabsTrigger>
          <TabsTrigger value="workflows">Workflows</TabsTrigger>
          <TabsTrigger value="runs">Runs</TabsTrigger>
          <TabsTrigger value="executions">Executions</TabsTrigger>
          <TabsTrigger value="triggers">Triggers</TabsTrigger>
          <TabsTrigger value="reviews">Reviews</TabsTrigger>
          <TabsTrigger value="templates">Templates</TabsTrigger>
        </TabsList>
        <TabsContent value="config" className="mt-4">
          <WorkflowConfigPanel projectId={activeProjectId} />
        </TabsContent>
        <TabsContent value="workflows" className="mt-4">
          <WorkflowListTab projectId={activeProjectId} setActiveTab={handleActiveTabChange} />
        </TabsContent>
        <TabsContent value="runs" className="mt-4">
          <WorkflowRunsTab projectId={activeProjectId} />
        </TabsContent>
        <TabsContent value="executions" className="mt-4">
          <ExecutionsTab projectId={activeProjectId} />
        </TabsContent>
        <TabsContent value="triggers" className="mt-4">
          <TriggersTab projectId={activeProjectId} />
        </TabsContent>
        <TabsContent value="reviews" className="mt-4">
          <WorkflowReviewsTab projectId={activeProjectId} />
        </TabsContent>
        <TabsContent value="templates" className="mt-4">
          <WorkflowTemplatesTab projectId={activeProjectId} setActiveTab={handleActiveTabChange} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default function WorkflowPage() {
  return (
    <Suspense fallback={null}>
      <WorkflowPageContent />
    </Suspense>
  );
}
