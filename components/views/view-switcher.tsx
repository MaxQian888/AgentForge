"use client";

import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { useSavedViewStore } from "@/lib/stores/saved-view-store";
import { useTaskWorkspaceStore } from "@/lib/stores/task-workspace-store";
import { SaveViewDialog } from "./save-view-dialog";
import { ViewShareDialog } from "./view-share-dialog";

export function ViewSwitcher({ projectId }: { projectId: string }) {
  const fetchViews = useSavedViewStore((state) => state.fetchViews);
  const viewsByProject = useSavedViewStore((state) => state.viewsByProject);
  const currentViewByProject = useSavedViewStore((state) => state.currentViewByProject);
  const selectView = useSavedViewStore((state) => state.selectView);
  const setDefaultView = useSavedViewStore((state) => state.setDefaultView);
  const applySavedViewConfig = useTaskWorkspaceStore((state) => state.applySavedViewConfig);
  const viewMode = useTaskWorkspaceStore((state) => state.viewMode);
  const filters = useTaskWorkspaceStore((state) => state.filters);
  const views = useMemo(() => viewsByProject[projectId] ?? [], [projectId, viewsByProject]);
  const currentViewId = useMemo(
    () => currentViewByProject[projectId] ?? null,
    [currentViewByProject, projectId]
  );

  const [saveOpen, setSaveOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);

  useEffect(() => {
    void fetchViews(projectId);
  }, [fetchViews, projectId]);

  const selectedView = useMemo(
    () => views.find((item) => item.id === currentViewId) ?? null,
    [currentViewId, views]
  );

  const currentConfig = useMemo(
    () => ({
      layout: viewMode,
      filters: [
        { field: "status", op: "eq", value: filters.status },
        { field: "priority", op: "eq", value: filters.priority },
        { field: "assigneeId", op: "eq", value: filters.assigneeId },
        { field: "sprintId", op: "eq", value: filters.sprintId },
        { field: "search", op: "contains", value: filters.search },
      ],
    }),
    [filters, viewMode]
  );

  return (
    <div className="flex flex-wrap items-center gap-2">
      <select
        className="h-9 rounded-md border bg-background px-3 text-sm"
        value={currentViewId ?? ""}
        onChange={(event) => {
          const nextId = event.target.value || null;
          selectView(projectId, nextId);
          const nextView = views.find((item) => item.id === nextId);
          if (nextView) {
            applySavedViewConfig(nextView.config);
          }
        }}
      >
        <option value="">Unsaved view</option>
        {views.map((view) => (
          <option key={view.id} value={view.id}>
            {view.name}
          </option>
        ))}
      </select>
      <Button type="button" size="sm" variant="outline" onClick={() => setSaveOpen(true)}>
        Save View
      </Button>
      <Button
        type="button"
        size="sm"
        variant="outline"
        disabled={!selectedView}
        onClick={() => setShareOpen(true)}
      >
        Share
      </Button>
      <Button
        type="button"
        size="sm"
        variant="outline"
        disabled={!selectedView}
        onClick={() => selectedView && void setDefaultView(projectId, selectedView.id)}
      >
        Set Default
      </Button>
      <SaveViewDialog open={saveOpen} onOpenChange={setSaveOpen} projectId={projectId} config={currentConfig} />
      <ViewShareDialog open={shareOpen} onOpenChange={setShareOpen} projectId={projectId} view={selectedView} />
    </div>
  );
}
