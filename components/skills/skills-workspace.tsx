"use client";

import Link from "next/link";
import { useMemo } from "react";
import { AlertTriangle, CheckCircle2, ExternalLink, GitBranch, Search, ShieldAlert, Wrench } from "lucide-react";
import { SkillPackagePreviewPane } from "@/components/marketplace/skill-package-preview";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import type {
  GovernedSkillItem,
  SkillFamily,
  SkillHealthStatus,
  SkillsFilters,
} from "@/lib/stores/skills-store";

interface SkillsWorkspaceProps {
  items: GovernedSkillItem[];
  selectedSkill: GovernedSkillItem | null;
  loading: boolean;
  detailLoading: boolean;
  actionLoading: boolean;
  error: string | null;
  filters: SkillsFilters;
  onSelectSkill: (id: string) => void | Promise<void>;
  onVerifyInternal: () => void | Promise<void>;
  onVerifyBuiltIns: () => void | Promise<void>;
  onSyncMirrors: () => void | Promise<void>;
  onSetFilters: (next: Partial<SkillsFilters>) => void;
}

const familyOptions: Array<{ value: SkillsFilters["family"]; label: string }> = [
  { value: "all", label: "All Families" },
  { value: "built-in-runtime", label: "Built-in Runtime" },
  { value: "repo-assistant", label: "Repo Assistant" },
  { value: "workflow-mirror", label: "Workflow Mirror" },
];

const statusOptions: Array<{ value: SkillsFilters["status"]; label: string }> = [
  { value: "all", label: "All Statuses" },
  { value: "healthy", label: "Healthy" },
  { value: "warning", label: "Warning" },
  { value: "drifted", label: "Drifted" },
  { value: "blocked", label: "Blocked" },
];

function statusIcon(status: SkillHealthStatus) {
  switch (status) {
    case "healthy":
      return <CheckCircle2 className="size-4" />;
    case "warning":
      return <AlertTriangle className="size-4" />;
    case "drifted":
      return <GitBranch className="size-4" />;
    case "blocked":
      return <ShieldAlert className="size-4" />;
    default:
      return <Wrench className="size-4" />;
  }
}

function statusTone(status: SkillHealthStatus) {
  switch (status) {
    case "healthy":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "drifted":
      return "border-sky-200 bg-sky-50 text-sky-700";
    case "blocked":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-border bg-muted text-foreground";
  }
}

function familyLabel(value: SkillFamily) {
  switch (value) {
    case "built-in-runtime":
      return "Built-in Runtime";
    case "repo-assistant":
      return "Repo Assistant";
    case "workflow-mirror":
      return "Workflow Mirror";
    default:
      return value;
  }
}

export function SkillsWorkspace({
  items,
  selectedSkill,
  loading,
  detailLoading,
  actionLoading,
  error,
  filters,
  onSelectSkill,
  onVerifyInternal,
  onVerifyBuiltIns,
  onSyncMirrors,
  onSetFilters,
}: SkillsWorkspaceProps) {
  const filteredItems = useMemo(() => {
    return items.filter((item) => {
      if (filters.family !== "all" && item.family !== filters.family) {
        return false;
      }
      if (filters.status !== "all" && item.health.status !== filters.status) {
        return false;
      }
      if (!filters.query.trim()) {
        return true;
      }
      const query = filters.query.trim().toLowerCase();
      return (
        item.id.toLowerCase().includes(query) ||
        item.canonicalRoot.toLowerCase().includes(query) ||
        item.family.toLowerCase().includes(query)
      );
    });
  }, [filters.family, filters.query, filters.status, items]);

  const canVerifyBuiltIns =
    selectedSkill?.supportedActions?.includes("verify-builtins") ?? false;
  const canSyncMirrors =
    selectedSkill?.supportedActions?.includes("sync-mirrors") ?? false;

  return (
    <div className="flex h-full flex-col border-t bg-card">
      <div className="flex flex-col gap-4 border-b px-4 py-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-1">
          <h1 className="text-sm font-semibold">Skills</h1>
          <p className="text-xs text-muted-foreground">
            Inspect governed skills, verify health, sync workflow mirrors, and hand off into the right downstream workspace.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => void onVerifyInternal()} disabled={actionLoading}>
            Verify Internal Skills
          </Button>
          {canVerifyBuiltIns ? (
            <Button size="sm" variant="outline" onClick={() => void onVerifyBuiltIns()} disabled={actionLoading}>
              Verify Built-in Skills
            </Button>
          ) : null}
          {canSyncMirrors ? (
            <Button size="sm" onClick={() => void onSyncMirrors()} disabled={actionLoading}>
              Sync Mirrors
            </Button>
          ) : null}
        </div>
      </div>

      <div className="grid flex-1 gap-0 lg:grid-cols-[320px_minmax(0,1fr)]">
        <aside className="border-r">
          <div className="space-y-3 border-b p-4">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={filters.query}
                onChange={(event) => onSetFilters({ query: event.target.value })}
                placeholder="Search skills..."
                className="pl-9"
              />
            </div>
            <div className="flex flex-wrap gap-2">
              {familyOptions.map((option) => (
                <Button
                  key={option.value}
                  size="sm"
                  variant={filters.family === option.value ? "default" : "outline"}
                  onClick={() => onSetFilters({ family: option.value })}
                >
                  {option.label}
                </Button>
              ))}
            </div>
            <div className="flex flex-wrap gap-2">
              {statusOptions.map((option) => (
                <Button
                  key={option.value}
                  size="sm"
                  variant={filters.status === option.value ? "secondary" : "outline"}
                  onClick={() => onSetFilters({ status: option.value })}
                >
                  {option.label}
                </Button>
              ))}
            </div>
          </div>

          <ScrollArea className="h-[calc(100vh-240px)] lg:h-full">
            <div className="space-y-2 p-3">
              {loading ? (
                <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                  Loading governed skills...
                </div>
              ) : filteredItems.length > 0 ? (
                filteredItems.map((item) => {
                  const selected = selectedSkill?.id === item.id;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => void onSelectSkill(item.id)}
                      className={cn(
                        "flex w-full flex-col gap-2 rounded-lg border p-3 text-left transition-colors",
                        selected
                          ? "border-primary bg-primary/5"
                          : "border-border hover:bg-muted/40",
                      )}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-medium">{item.id}</p>
                          <p className="truncate text-xs text-muted-foreground">
                            {item.canonicalRoot}
                          </p>
                        </div>
                        <Badge
                          variant="outline"
                          className={cn("gap-1 capitalize", statusTone(item.health.status))}
                        >
                          {statusIcon(item.health.status)}
                          {item.health.status}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <span>{familyLabel(item.family)}</span>
                        {item.bundle.member ? <span>Built-in bundle</span> : null}
                        {item.previewAvailable ? <span>Preview</span> : null}
                      </div>
                    </button>
                  );
                })
              ) : (
                <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                  No governed skills match the current filters.
                </div>
              )}
            </div>
          </ScrollArea>
        </aside>

        <main className="min-h-0">
          <ScrollArea className="h-[calc(100vh-240px)] lg:h-full">
            <div className="space-y-4 p-4">
              {error ? (
                <Card className="border-rose-200 bg-rose-50">
                  <CardContent className="pt-6 text-sm text-rose-700">
                    {error}
                  </CardContent>
                </Card>
              ) : null}

              {detailLoading ? (
                <Card>
                  <CardContent className="pt-6 text-sm text-muted-foreground">
                    Loading skill detail...
                  </CardContent>
                </Card>
              ) : selectedSkill ? (
                <>
                  <Card>
                    <CardHeader className="space-y-3">
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <div className="space-y-1">
                          <CardTitle className="text-base">
                            {selectedSkill.preview?.label ?? selectedSkill.id}
                          </CardTitle>
                          <p className="text-sm text-muted-foreground">
                            {selectedSkill.canonicalRoot}
                          </p>
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant="outline">{familyLabel(selectedSkill.family)}</Badge>
                          <Badge variant="outline">{selectedSkill.sourceType}</Badge>
                          <Badge
                            variant="outline"
                            className={cn("capitalize", statusTone(selectedSkill.health.status))}
                          >
                            {selectedSkill.health.status}
                          </Badge>
                        </div>
                      </div>
                    </CardHeader>
                    <CardContent className="space-y-4 text-sm">
                      <div className="grid gap-3 md:grid-cols-2">
                        <div className="rounded-lg border p-3">
                          <p className="text-xs font-medium text-muted-foreground">Bundle</p>
                          <p className="mt-1 font-medium">
                            {selectedSkill.bundle.member ? "Official built-in member" : "Not in built-in bundle"}
                          </p>
                          {selectedSkill.bundle.category ? (
                            <p className="mt-1 text-xs text-muted-foreground">
                              Category: {selectedSkill.bundle.category}
                            </p>
                          ) : null}
                        </div>
                        <div className="rounded-lg border p-3">
                          <p className="text-xs font-medium text-muted-foreground">Provenance</p>
                          <p className="mt-1 font-medium">{selectedSkill.sourceType}</p>
                          {selectedSkill.lock?.source ? (
                            <p className="mt-1 text-xs text-muted-foreground">
                              Lock source: {selectedSkill.lock.source}
                            </p>
                          ) : null}
                        </div>
                      </div>

                      {selectedSkill.docsRef ? (
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                          <ExternalLink className="size-3.5" />
                          <span>{selectedSkill.docsRef}</span>
                        </div>
                      ) : null}

                      {selectedSkill.health.issues.length > 0 ? (
                        <div className="space-y-2">
                          <p className="text-xs font-medium">Diagnostics</p>
                          {selectedSkill.health.issues.map((issue) => (
                            <div
                              key={`${issue.code}-${issue.targetPath ?? issue.message}`}
                              className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900"
                            >
                              <p className="font-medium">{issue.message}</p>
                              {issue.targetPath ? (
                                <p className="mt-1 text-amber-800/80">{issue.targetPath}</p>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      ) : null}

                      {selectedSkill.blockedActions?.length ? (
                        <div className="space-y-2">
                          <p className="text-xs font-medium">Blocked Actions</p>
                          {selectedSkill.blockedActions.map((action) => (
                            <div key={action.id} className="rounded-lg border border-dashed p-3 text-xs text-muted-foreground">
                              {action.reason}
                            </div>
                          ))}
                        </div>
                      ) : null}

                      {selectedSkill.consumerSurfaces.length > 0 ? (
                        <div className="space-y-2">
                          <p className="text-xs font-medium">Downstream Handoffs</p>
                          <div className="flex flex-wrap gap-2">
                            {selectedSkill.consumerSurfaces.map((surface) =>
                              surface.href ? (
                                <Button key={surface.id} variant="outline" size="sm" asChild>
                                  <Link href={surface.href}>{surface.label}</Link>
                                </Button>
                              ) : (
                                <div key={surface.id} className="rounded-lg border px-3 py-2 text-xs text-muted-foreground">
                                  {surface.label}
                                  {surface.message ? ` · ${surface.message}` : ""}
                                </div>
                              ),
                            )}
                          </div>
                        </div>
                      ) : null}
                    </CardContent>
                  </Card>

                  {selectedSkill.preview ? (
                    <SkillPackagePreviewPane preview={selectedSkill.preview} />
                  ) : selectedSkill.previewError ? (
                    <Card className="border-amber-200 bg-amber-50">
                      <CardContent className="pt-6 text-sm text-amber-900">
                        {selectedSkill.previewError}
                      </CardContent>
                    </Card>
                  ) : null}

                  {selectedSkill.mirrorTargets?.length ? (
                    <>
                      <Separator />
                      <Card>
                        <CardHeader>
                          <CardTitle className="text-sm">Mirror Targets</CardTitle>
                        </CardHeader>
                        <CardContent className="space-y-2 text-xs text-muted-foreground">
                          {selectedSkill.mirrorTargets.map((target) => (
                            <div key={target} className="rounded-md border p-2">
                              {target}
                            </div>
                          ))}
                        </CardContent>
                      </Card>
                    </>
                  ) : null}
                </>
              ) : (
                <Card>
                  <CardContent className="pt-6 text-sm text-muted-foreground">
                    Select a governed skill to inspect its package preview, governance diagnostics, and downstream actions.
                  </CardContent>
                </Card>
              )}
            </div>
          </ScrollArea>
        </main>
      </div>
    </div>
  );
}
