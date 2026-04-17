"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  Copy,
  Database,
  Download,
  FileSearch,
  HardDrive,
  Pencil,
  Plus,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { ErrorBanner } from "@/components/shared/error-banner";
import { EmptyState } from "@/components/shared/empty-state";
import { MetricCard } from "@/components/shared/metric-card";
import { HighlightedText } from "@/components/shared/highlighted-text";
import { useBreakpoint } from "@/hooks/use-breakpoint";
import { cn } from "@/lib/utils";
import {
  useMemoryStore,
  type AgentMemoryDetail,
  type AgentMemoryEntry,
  type MemoryExportFormat,
} from "@/lib/stores/memory-store";
import { useRoleStore } from "@/lib/stores/role-store";

const ALL_VALUE = "all";
const ALL_ROLES_VALUE = "__all_roles__";
const scopeColors: Record<string, string> = {
  global: "bg-purple-500/15 text-purple-700 dark:text-purple-300",
  project: "bg-blue-500/15 text-blue-700 dark:text-blue-300",
  role: "bg-amber-500/15 text-amber-700 dark:text-amber-300",
};

interface MemoryPanelProps {
  projectId: string;
}

function formatBytes(bytes: number) {
  if (!bytes) return "0 B";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatTime(value?: string | null) {
  return value ? new Date(value).toLocaleString() : "—";
}

function toDateTimeInputValue(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (input: number) => String(input).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function fromDateTimeInputValue(value: string) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function downloadBlob(filename: string, content: string, mimeType: string) {
  if (typeof window === "undefined" || typeof document === "undefined") return;
  if (typeof window.URL?.createObjectURL !== "function") return;
  const blob = new Blob([content], { type: mimeType });
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  window.URL.revokeObjectURL?.(url);
}

export function MemoryPanel({ projectId }: MemoryPanelProps) {
  const t = useTranslations("memory");
  const { isDesktop } = useBreakpoint();
  const roles = useRoleStore((s) => s.roles);
  const fetchRoles = useRoleStore((s) => s.fetchRoles);

  const filters = useMemoryStore((s) => s.filters);
  const pagination = useMemoryStore((s) => s.pagination);
  const entries = useMemoryStore((s) => s.entries);
  const stats = useMemoryStore((s) => s.stats);
  const detail = useMemoryStore((s) => s.detail);
  const selectedMemoryId = useMemoryStore((s) => s.selectedMemoryId);
  const selectedMemoryIds = useMemoryStore((s) => s.selectedMemoryIds);
  const loading = useMemoryStore((s) => s.loading);
  const statsLoading = useMemoryStore((s) => s.statsLoading);
  const detailLoading = useMemoryStore((s) => s.detailLoading);
  const actionLoading = useMemoryStore((s) => s.actionLoading);
  const error = useMemoryStore((s) => s.error);
  const statsError = useMemoryStore((s) => s.statsError);
  const detailError = useMemoryStore((s) => s.detailError);
  const actionError = useMemoryStore((s) => s.actionError);
  const lastMutation = useMemoryStore((s) => s.lastMutation);
  const loadWorkspace = useMemoryStore((s) => s.loadWorkspace);
  const setFilters = useMemoryStore((s) => s.setFilters);
  const resetFilters = useMemoryStore((s) => s.resetFilters);
  const setPagination = useMemoryStore((s) => s.setPagination);
  const fetchMemoryDetail = useMemoryStore((s) => s.fetchMemoryDetail);
  const selectMemory = useMemoryStore((s) => s.selectMemory);
  const toggleMemorySelection = useMemoryStore((s) => s.toggleMemorySelection);
  const clearSelection = useMemoryStore((s) => s.clearSelection);
  const deleteMemory = useMemoryStore((s) => s.deleteMemory);
  const bulkDeleteMemories = useMemoryStore((s) => s.bulkDeleteMemories);
  const bulkDeleteByCriteria = useMemoryStore((s) => s.bulkDeleteByCriteria);
  const cleanupMemories = useMemoryStore((s) => s.cleanupMemories);
  const exportMemories = useMemoryStore((s) => s.exportMemories);
  const exportMemoryEntry = useMemoryStore((s) => s.exportMemoryEntry);
  const storeMemory = useMemoryStore((s) => s.storeMemory);
  const updateMemory = useMemoryStore((s) => s.updateMemory);
  const addMemoryTag = useMemoryStore((s) => s.addMemoryTag);
  const removeMemoryTag = useMemoryStore((s) => s.removeMemoryTag);
  const buildExportBlob = useMemoryStore((s) => s.buildExportBlob);
  const clearActionFeedback = useMemoryStore((s) => s.clearActionFeedback);

  const [singleDeleteTarget, setSingleDeleteTarget] = useState<AgentMemoryEntry | null>(null);
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false);
  const [bulkCriteriaOpen, setBulkCriteriaOpen] = useState(false);
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [detailSheetOpen, setDetailSheetOpen] = useState(false);
  const [queryDraft, setQueryDraft] = useState(filters.query);
  const [tagDraft, setTagDraft] = useState(filters.tag);
  const [detailTagDraft, setDetailTagDraft] = useState("");
  const [noteDraft, setNoteDraft] = useState({
    key: "",
    content: "",
    tags: "",
  });
  const [editDraft, setEditDraft] = useState({
    key: "",
    content: "",
    tags: "",
  });
  const [cleanupDraft, setCleanupDraft] = useState({
    retentionDays: 30,
    before: "",
  });

  useEffect(() => {
    if (roles.length === 0) void fetchRoles();
  }, [fetchRoles, roles.length]);

  useEffect(() => {
    void loadWorkspace(projectId);
  }, [projectId, filters, loadWorkspace]);

  const selectedRoleLabel = useMemo(
    () => roles.find((role) => role.metadata.id === filters.roleId)?.metadata.name,
    [filters.roleId, roles],
  );
  const selectedEntry = entries.find((entry) => entry.id === selectedMemoryId) ?? null;
  const currentDetail = detail && detail.id === selectedMemoryId
    ? ({ ...(selectedEntry ?? {}), ...detail } as AgentMemoryDetail)
    : selectedEntry;

  const handleOpenEntry = async (entry: AgentMemoryEntry) => {
    selectMemory(entry.id);
    if (!isDesktop) setDetailSheetOpen(true);
    await fetchMemoryDetail(projectId, entry.id, filters.roleId || undefined);
  };

  const applyTagFilter = (tag: string) => {
    setTagDraft(tag);
    setFilters({ tag });
  };

  const activeBadges = [
    filters.query ? t("activeFilter.query", { value: filters.query }) : null,
    filters.scope !== ALL_VALUE ? t(`scopeOption.${filters.scope}`) : null,
    filters.category !== ALL_VALUE ? t(`categoryOption.${filters.category}`) : null,
    filters.roleId ? t("activeFilter.role", { value: selectedRoleLabel ?? filters.roleId }) : null,
    filters.tag ? t("activeFilter.tag", { value: filters.tag }) : null,
    t("activeFilter.limit", { count: filters.limit }),
  ].filter(Boolean) as string[];

  const totalEntries = entries.length;
  const pageSize = pagination && pagination.pageSize > 0 ? pagination.pageSize : 10;
  const totalPages = totalEntries > 0 ? Math.ceil(totalEntries / pageSize) : 1;
  const currentPage = Math.min(Math.max(pagination?.page ?? 1, 1), totalPages);
  const pageStart = totalEntries === 0 ? 0 : (currentPage - 1) * pageSize;
  const pageEnd = Math.min(pageStart + pageSize, totalEntries);
  const pagedEntries = entries.slice(pageStart, pageEnd);

  const handleExport = async (format: MemoryExportFormat) => {
    const exported = await exportMemories(projectId);
    if (!exported) return;
    const blob = buildExportBlob(exported, format);
    downloadBlob(
      `memory-export-${projectId}.${blob.extension}`,
      blob.content,
      blob.mimeType,
    );
  };

  const detailPanelProps = {
    detail: currentDetail,
    detailLoading,
    detailError,
    queryHighlight: filters.query,
    tagDraft: detailTagDraft,
    onTagDraftChange: setDetailTagDraft,
    onAddTag: async () => {
      if (!currentDetail) return;
      const value = detailTagDraft.trim();
      if (!value) return;
      await addMemoryTag(projectId, currentDetail.id, value);
      setDetailTagDraft("");
    },
    onRemoveTag: (tag: string) => {
      if (!currentDetail) return;
      void removeMemoryTag(projectId, currentDetail.id, tag);
    },
    onCopy: async () => {
      if (currentDetail && navigator?.clipboard?.writeText)
        await navigator.clipboard.writeText(currentDetail.content);
    },
    onDelete: () => currentDetail && setSingleDeleteTarget(currentDetail),
    onEdit: () => {
      if (!currentDetail || !currentDetail.editable) return;
      setEditDraft({
        key: currentDetail.key,
        content: currentDetail.content,
        tags: (currentDetail.tags ?? []).join(", "),
      });
      setEditOpen(true);
    },
    onExport: async (format: MemoryExportFormat) => {
      if (!currentDetail) return;
      const exported = await exportMemoryEntry(
        projectId,
        currentDetail.id,
        filters.roleId || undefined,
      );
      if (exported) {
        const blob = buildExportBlob(exported, format);
        downloadBlob(
          `memory-entry-${currentDetail.id}.${blob.extension}`,
          blob.content,
          blob.mimeType,
        );
      }
    },
    onTagClick: applyTagFilter,
    t,
  };

  return (
    <div className="flex flex-col gap-6">
      {(error || statsError || actionError) && (
        <ErrorBanner
          message={actionError ?? error ?? statsError ?? ""}
          onRetry={() => {
            clearActionFeedback();
            void loadWorkspace(projectId);
          }}
        />
      )}

      {lastMutation && (
        <Card className="border-emerald-500/30 bg-emerald-500/5">
          <CardContent className="flex items-center gap-3 py-3 text-sm text-emerald-700 dark:text-emerald-300">
            <span>{t(`mutation.${lastMutation.type}`, { count: lastMutation.deletedCount })}</span>
            <Button variant="ghost" size="sm" className="ml-auto h-7 text-xs" onClick={clearActionFeedback}>
              {t("dismiss")}
            </Button>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <div data-testid="memory-stat-total">
          <MetricCard
            icon={Database}
            label={t("stats.total")}
            value={statsLoading ? t("loadingShort") : String(stats?.totalCount ?? 0)}
          />
        </div>
        <MetricCard
          icon={HardDrive}
          label={t("stats.storage")}
          value={statsLoading ? t("loadingShort") : formatBytes(stats?.approxStorageBytes ?? 0)}
        />
        <MetricCard
          icon={FileSearch}
          label={t("stats.categories")}
          value={Object.keys(stats?.byCategory ?? {}).length}
        />
        <MetricCard
          icon={CalendarClock}
          label={t("stats.lastAccessed")}
          value={statsLoading ? t("loadingShort") : formatTime(stats?.lastAccessedAt)}
        />
      </div>

      <Card>
        <CardHeader className="gap-4">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div>
              <CardTitle>{t("workspaceTitle")}</CardTitle>
              <CardDescription>{t("workspaceDescription")}</CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                className="gap-2"
                onClick={() => void handleExport("json")}
                disabled={actionLoading}
              >
                <Download className="size-4" />
                {t("actionExport")}
              </Button>
              <Button
                variant="outline"
                className="gap-2"
                onClick={() => void handleExport("csv")}
                disabled={actionLoading}
              >
                <Download className="size-4" />
                {t("actionExportCsv")}
              </Button>
              <Button variant="outline" className="gap-2" onClick={() => setCleanupOpen(true)} disabled={actionLoading}>
                <CalendarClock className="size-4" />
                {t("actionCleanup")}
              </Button>
              {entries.length > 0 && (
                <Button
                  variant="outline"
                  className="gap-2"
                  onClick={() => setBulkCriteriaOpen(true)}
                  disabled={actionLoading}
                >
                  <Trash2 className="size-4" />
                  {t("actionBulkDeleteByCriteria", { count: entries.length })}
                </Button>
              )}
              {selectedMemoryIds.length > 0 && (
                <Button variant="destructive" className="gap-2" onClick={() => setBulkDeleteOpen(true)} disabled={actionLoading}>
                  <Trash2 className="size-4" />
                  {t("actionBulkDelete", { count: selectedMemoryIds.length })}
                </Button>
              )}
            </div>
          </div>

          <div className="grid gap-3 xl:grid-cols-[minmax(0,2fr)_repeat(4,minmax(0,1fr))]">
            <div className="relative xl:col-span-2">
              <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input placeholder={t("searchPlaceholder")} value={queryDraft} onChange={(event) => {
                setQueryDraft(event.target.value);
                setFilters({ query: event.target.value });
              }} className="pl-9" />
            </div>

            <Select value={filters.scope as "global" | "project" | "role" | "all"} onValueChange={(value) => setFilters({
              scope: value as "global" | "project" | "role" | "all",
              ...(value === "role" && !filters.roleId && roles[0] ? { roleId: roles[0].metadata.id } : {}),
            })}>
              <SelectTrigger aria-label={t("scopeFilterLabel")}><SelectValue placeholder={t("scopeFilterLabel")} /></SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_VALUE}>{t("scopeOption.all")}</SelectItem>
                <SelectItem value="global">{t("scopeOption.global")}</SelectItem>
                <SelectItem value="project">{t("scopeOption.project")}</SelectItem>
                <SelectItem value="role">{t("scopeOption.role")}</SelectItem>
              </SelectContent>
            </Select>

            <Select value={filters.roleId || ALL_ROLES_VALUE} onValueChange={(value) => setFilters({ roleId: value === ALL_ROLES_VALUE ? "" : value })}>
              <SelectTrigger aria-label={t("roleFilterLabel")}><SelectValue placeholder={t("roleFilterLabel")} /></SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_ROLES_VALUE}>{t("allRoles")}</SelectItem>
                {roles.map((role) => <SelectItem key={role.metadata.id} value={role.metadata.id}>{role.metadata.name || role.metadata.id}</SelectItem>)}
              </SelectContent>
            </Select>

            <Input aria-label={t("startAtLabel")} type="datetime-local" value={toDateTimeInputValue(filters.startAt)} onChange={(event) => setFilters({ startAt: fromDateTimeInputValue(event.target.value) })} />

            <Select value={String(filters.limit)} onValueChange={(value) => setFilters({ limit: Number(value) })}>
              <SelectTrigger aria-label={t("limitLabel")}><SelectValue placeholder={t("limitLabel")} /></SelectTrigger>
              <SelectContent>
                {[20, 50, 100].map((value) => <SelectItem key={value} value={String(value)}>{String(value)}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>

          <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
            <Input aria-label={t("endAtLabel")} type="datetime-local" value={toDateTimeInputValue(filters.endAt)} onChange={(event) => setFilters({ endAt: fromDateTimeInputValue(event.target.value) })} />
            <Input
              aria-label={t("tagFilterLabel")}
              placeholder={t("tagFilterPlaceholder")}
              value={tagDraft}
              onChange={(event) => {
                setTagDraft(event.target.value);
                setFilters({ tag: event.target.value });
              }}
            />
            <Tabs value={filters.category as "episodic" | "semantic" | "procedural" | "all"} onValueChange={(value) => setFilters({ category: value as "episodic" | "semantic" | "procedural" | "all" })}>
              <TabsList className="w-full justify-start">
                <TabsTrigger value={ALL_VALUE}>{t("categoryOption.all")}</TabsTrigger>
                <TabsTrigger value="episodic">{t("categoryOption.episodic")}</TabsTrigger>
                <TabsTrigger value="semantic">{t("categoryOption.semantic")}</TabsTrigger>
                <TabsTrigger value="procedural">{t("categoryOption.procedural")}</TabsTrigger>
              </TabsList>
            </Tabs>
          </div>
          <div className="flex justify-end">
            <Button variant="ghost" onClick={() => { setQueryDraft(""); setTagDraft(""); resetFilters(); clearSelection(); }}>{t("actionResetFilters")}</Button>
          </div>

          <div className="flex flex-wrap gap-2">
            {activeBadges.map((badge) => <Badge key={badge} variant="secondary">{badge}</Badge>)}
          </div>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("noteComposerTitle")}</CardTitle>
          <CardDescription>{t("noteComposerDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label htmlFor="memory-note-title" className="text-sm font-medium">{t("noteTitleLabel")}</label>
              <Input
                id="memory-note-title"
                aria-label={t("noteTitleLabel")}
                value={noteDraft.key}
                onChange={(event) => setNoteDraft((current) => ({ ...current, key: event.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="memory-note-tags" className="text-sm font-medium">{t("noteTagsLabel")}</label>
              <Input
                id="memory-note-tags"
                aria-label={t("noteTagsLabel")}
                placeholder={t("noteTagsPlaceholder")}
                value={noteDraft.tags}
                onChange={(event) => setNoteDraft((current) => ({ ...current, tags: event.target.value }))}
              />
            </div>
          </div>
          <div className="space-y-2">
            <label htmlFor="memory-note-content" className="text-sm font-medium">{t("noteContentLabel")}</label>
            <Textarea
              id="memory-note-content"
              aria-label={t("noteContentLabel")}
              value={noteDraft.content}
              onChange={(event) => setNoteDraft((current) => ({ ...current, content: event.target.value }))}
            />
          </div>
          <div className="flex justify-end">
            <Button
              onClick={async () => {
                await storeMemory(projectId, {
                  key: noteDraft.key.trim(),
                  content: noteDraft.content.trim(),
                  scope: "project",
                  category: "episodic",
                  kind: "operator_note",
                  tags: noteDraft.tags.split(",").map((tag) => tag.trim()).filter(Boolean),
                });
                setNoteDraft({ key: "", content: "", tags: "" });
              }}
              disabled={actionLoading || !noteDraft.key.trim() || !noteDraft.content.trim()}
            >
              {t("createNote")}
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <Card>
          <CardHeader className="gap-3">
            <div className="flex items-center justify-between gap-3">
              <div>
                <CardTitle>{t("resultsTitle")}</CardTitle>
                <CardDescription>{t("resultsDescription", { count: entries.length, total: stats?.totalCount ?? entries.length })}</CardDescription>
              </div>
              {selectedMemoryIds.length > 0 && <Button variant="ghost" size="sm" onClick={clearSelection}>{t("clearSelection")}</Button>}
            </div>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            {loading ? (
              <p className="text-sm text-muted-foreground">{t("loading")}</p>
            ) : entries.length === 0 ? (
              <EmptyState icon={FileSearch} title={t("noEntries")} description={t("noEntriesDescription")} action={{ label: t("actionResetFilters"), onClick: () => { setQueryDraft(""); setTagDraft(""); resetFilters(); clearSelection(); } }} />
            ) : (
              <>
                {pagedEntries.map((entry) => {
                  const selected = selectedMemoryId === entry.id;
                  return (
                    <Card key={entry.id} className={cn("border-border/60", selected && "border-primary bg-primary/5")}>
                      <CardContent className="flex flex-col gap-3 py-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="flex items-start gap-3">
                            <input type="checkbox" aria-label={t("selectEntry", { key: entry.key })} checked={selectedMemoryIds.includes(entry.id)} onChange={() => toggleMemorySelection(entry.id)} className="mt-1 size-4" />
                            <div className="space-y-2">
                              <div className="flex flex-wrap items-center gap-2">
                                <h3 className="text-sm font-semibold">
                                  <HighlightedText text={entry.key} query={filters.query} />
                                </h3>
                                <Badge variant="secondary" className={cn(scopeColors[entry.scope] ?? "")}>{t(`scopeOption.${entry.scope}`)}</Badge>
                                <Badge variant="outline">{t(`categoryOption.${entry.category}`)}</Badge>
                                {entry.kind === "operator_note" && <Badge variant="outline">{t("kind.operator_note")}</Badge>}
                                {entry.tags.map((tag) => (
                                  <span
                                    key={`${entry.id}-${tag}`}
                                    className="inline-flex items-center gap-1 rounded-md bg-muted/60 px-1.5 py-0.5 text-xs"
                                  >
                                    <button
                                      type="button"
                                      onClick={() => applyTagFilter(tag)}
                                      className="font-medium text-foreground/80 hover:text-foreground"
                                    >
                                      {tag}
                                    </button>
                                    {entry.editable && (
                                      <button
                                        type="button"
                                        aria-label={t("tag.remove", { tag })}
                                        onClick={() => void removeMemoryTag(projectId, entry.id, tag)}
                                        className="text-muted-foreground hover:text-destructive"
                                      >
                                        <X className="size-3" />
                                      </button>
                                    )}
                                  </span>
                                ))}
                              </div>
                              <p className="line-clamp-2 text-sm text-muted-foreground">
                                <HighlightedText text={entry.content} query={filters.query} />
                              </p>
                              <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
                                <span>{t("accessed", { count: entry.accessCount })}</span>
                                <span>{formatTime(entry.createdAt)}</span>
                              </div>
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            <Button variant="outline" size="sm" aria-label={t("openEntry", { key: entry.key })} onClick={() => void handleOpenEntry(entry)}>
                              {t("openEntryShort")}
                            </Button>
                            <Button variant="ghost" size="icon-sm" aria-label={t("deleteEntry", { key: entry.key })} onClick={() => setSingleDeleteTarget(entry)}>
                              <Trash2 className="size-4" />
                            </Button>
                          </div>
                        </div>
                      </CardContent>
                    </Card>
                  );
                })}
                <div
                  data-testid="memory-pagination"
                  className="flex flex-col gap-2 pt-2 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between"
                >
                  <span>
                    {t("pagination.summary", {
                      from: totalEntries === 0 ? 0 : pageStart + 1,
                      to: pageEnd,
                      count: totalEntries,
                    })}
                  </span>
                  <div className="flex flex-wrap items-center gap-2">
                    <label
                      htmlFor="memory-page-size"
                      className="text-xs text-muted-foreground"
                    >
                      {t("pagination.pageSize")}
                    </label>
                    <Select
                      value={String(pageSize)}
                      onValueChange={(value) => setPagination({ pageSize: Number(value), page: 1 })}
                    >
                      <SelectTrigger
                        id="memory-page-size"
                        aria-label={t("pagination.pageSize")}
                        className="h-8 w-[5rem]"
                      >
                        <SelectValue placeholder={String(pageSize)} />
                      </SelectTrigger>
                      <SelectContent>
                        {[5, 10, 20, 50].map((size) => (
                          <SelectItem key={size} value={String(size)}>
                            {size}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Button
                      variant="outline"
                      size="sm"
                      aria-label={t("pagination.prev")}
                      onClick={() => setPagination({ page: Math.max(1, currentPage - 1) })}
                      disabled={currentPage <= 1}
                    >
                      <ChevronLeft className="size-4" />
                    </Button>
                    <span data-testid="memory-page-indicator">
                      {t("pagination.page", { page: currentPage, total: totalPages })}
                    </span>
                    <Button
                      variant="outline"
                      size="sm"
                      aria-label={t("pagination.next")}
                      onClick={() => setPagination({ page: Math.min(totalPages, currentPage + 1) })}
                      disabled={currentPage >= totalPages}
                    >
                      <ChevronRight className="size-4" />
                    </Button>
                  </div>
                </div>
              </>
            )}
          </CardContent>
        </Card>
        {isDesktop && (
          <Card data-testid="memory-detail-panel">
            <CardHeader>
              <CardTitle>{t("detailTitle")}</CardTitle>
              <CardDescription>{t("detailDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <MemoryDetail {...detailPanelProps} />
            </CardContent>
          </Card>
        )}
      </div>

      <Sheet open={!isDesktop && detailSheetOpen} onOpenChange={setDetailSheetOpen}>
        <SheetContent side="right" className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{t("detailTitle")}</SheetTitle>
            <SheetDescription>{t("detailDescription")}</SheetDescription>
          </SheetHeader>
          <div className="px-4 pb-4">
            <MemoryDetail {...detailPanelProps} />
          </div>
        </SheetContent>
      </Sheet>

      <AlertDialog open={!!singleDeleteTarget} onOpenChange={(open) => !open && setSingleDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("confirmSingleDeleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("confirmSingleDeleteDescription", { key: singleDeleteTarget?.key ?? "" })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => {
              if (!singleDeleteTarget) return;
              void deleteMemory(projectId, singleDeleteTarget.id);
              setSingleDeleteTarget(null);
            }}>{t("confirmSingleDelete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={bulkDeleteOpen} onOpenChange={setBulkDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("confirmBulkDeleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("confirmBulkDeleteDescription", { count: selectedMemoryIds.length })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" aria-label={t("confirmBulkDelete")} onClick={() => void bulkDeleteMemories(projectId, selectedMemoryIds, filters.roleId || undefined)}>
              {t("confirmBulkDelete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={bulkCriteriaOpen} onOpenChange={setBulkCriteriaOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("confirmBulkDeleteByCriteriaTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("confirmBulkDeleteByCriteriaDescription", { count: entries.length })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              aria-label={t("confirmBulkDeleteByCriteria")}
              onClick={() => {
                void bulkDeleteByCriteria(projectId, {
                  ids: entries.map((entry) => entry.id),
                  roleId: filters.roleId || undefined,
                });
                setBulkCriteriaOpen(false);
              }}
            >
              {t("confirmBulkDeleteByCriteria")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={cleanupOpen} onOpenChange={setCleanupOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("cleanupTitle")}</DialogTitle>
            <DialogDescription>{t("cleanupDescription")}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="space-y-2">
              <label htmlFor="cleanup-retention" className="text-sm font-medium">{t("cleanupRetentionLabel")}</label>
              <Input id="cleanup-retention" type="number" min={1} value={cleanupDraft.retentionDays} onChange={(event) => setCleanupDraft((current) => ({ ...current, retentionDays: Number(event.target.value || 0) }))} />
            </div>
            <div className="space-y-2">
              <label htmlFor="cleanup-before" className="text-sm font-medium">{t("cleanupBeforeLabel")}</label>
              <Input id="cleanup-before" type="datetime-local" value={cleanupDraft.before} onChange={(event) => setCleanupDraft((current) => ({ ...current, before: event.target.value }))} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCleanupOpen(false)}>{t("cancel")}</Button>
            <Button onClick={() => void cleanupMemories(projectId, { ...cleanupDraft, before: fromDateTimeInputValue(cleanupDraft.before) || undefined })}>{t("confirmCleanup")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("editNoteTitle")}</DialogTitle>
            <DialogDescription>{t("editNoteDescription")}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="space-y-2">
              <label htmlFor="memory-edit-title" className="text-sm font-medium">{t("noteTitleLabel")}</label>
              <Input
                id="memory-edit-title"
                aria-label={t("noteTitleLabel")}
                value={editDraft.key}
                onChange={(event) => setEditDraft((current) => ({ ...current, key: event.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="memory-edit-content" className="text-sm font-medium">{t("noteContentLabel")}</label>
              <Textarea
                id="memory-edit-content"
                aria-label={t("noteContentLabel")}
                value={editDraft.content}
                onChange={(event) => setEditDraft((current) => ({ ...current, content: event.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="memory-edit-tags" className="text-sm font-medium">{t("editNoteTagsLabel")}</label>
              <Input
                id="memory-edit-tags"
                aria-label={t("editNoteTagsLabel")}
                value={editDraft.tags}
                onChange={(event) => setEditDraft((current) => ({ ...current, tags: event.target.value }))}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>{t("cancel")}</Button>
            <Button
              onClick={async () => {
                if (!currentDetail) return;
                await updateMemory(projectId, currentDetail.id, {
                  key: editDraft.key.trim(),
                  content: editDraft.content.trim(),
                  tags: editDraft.tags.split(",").map((tag) => tag.trim()).filter(Boolean),
                  roleId: filters.roleId || undefined,
                });
                setEditOpen(false);
              }}
              disabled={!editDraft.key.trim() || !editDraft.content.trim()}
            >
              {t("saveNote")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function MemoryDetail({
  detail,
  detailLoading,
  detailError,
  queryHighlight,
  tagDraft,
  onTagDraftChange,
  onAddTag,
  onRemoveTag,
  onCopy,
  onDelete,
  onEdit,
  onExport,
  onTagClick,
  t,
}: {
  detail: AgentMemoryDetail | AgentMemoryEntry | null;
  detailLoading: boolean;
  detailError: string | null;
  queryHighlight?: string;
  tagDraft: string;
  onTagDraftChange: (value: string) => void;
  onAddTag: () => void;
  onRemoveTag: (tag: string) => void;
  onCopy: () => void;
  onDelete: () => void;
  onEdit: () => void;
  onExport: (format: MemoryExportFormat) => void;
  onTagClick: (tag: string) => void;
  t: ReturnType<typeof useTranslations<"memory">>;
}) {
  if (detailLoading) return <p className="text-sm text-muted-foreground">{t("loadingDetail")}</p>;
  if (detailError) return <ErrorBanner message={detailError} />;
  if (!detail) return <EmptyState icon={FileSearch} title={t("detailEmptyTitle")} description={t("detailEmptyDescription")} />;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="secondary" className={cn(scopeColors[detail.scope] ?? "")}>{t(`scopeOption.${detail.scope}`)}</Badge>
        <Badge variant="outline">{t(`categoryOption.${detail.category}`)}</Badge>
        {detail.kind === "operator_note" && <Badge variant="outline">{t("kind.operator_note")}</Badge>}
        {(detail.tags ?? []).map((tag) => (
          <span
            key={`${detail.id}-${tag}`}
            className="inline-flex items-center gap-1 rounded-md bg-muted/60 px-1.5 py-0.5 text-xs"
          >
            <button
              type="button"
              onClick={() => onTagClick(tag)}
              className="font-medium text-foreground/80 hover:text-foreground"
            >
              {tag}
            </button>
            {detail.editable && (
              <button
                type="button"
                aria-label={t("tag.remove", { tag })}
                onClick={() => onRemoveTag(tag)}
                className="text-muted-foreground hover:text-destructive"
              >
                <X className="size-3" />
              </button>
            )}
          </span>
        ))}
        <Button
          variant="outline"
          size="sm"
          className="gap-2"
          onClick={() => onExport("json")}
        >
          {t("exportEntry")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="gap-2"
          onClick={() => onExport("csv")}
        >
          {t("exportEntryCsv")}
        </Button>
        {detail.editable ? (
          <Button variant="outline" size="sm" className="gap-2" onClick={onEdit}><Pencil className="size-4" />{t("editNote")}</Button>
        ) : (
          <span className="text-xs text-muted-foreground">{t("detailReadOnly")}</span>
        )}
        <Button variant="outline" size="sm" className="ml-auto gap-2" onClick={onCopy}><Copy className="size-4" />{t("copyContent")}</Button>
        <Button variant="ghost" size="icon-sm" onClick={onDelete}><Trash2 className="size-4" /></Button>
      </div>
      {detail.editable && (
        <div className="flex flex-wrap items-center gap-2">
          <Input
            aria-label={t("tag.addLabel")}
            placeholder={t("tag.addPlaceholder")}
            value={tagDraft}
            onChange={(event) => onTagDraftChange(event.target.value)}
            className="h-8 w-40 text-xs"
          />
          <Button
            variant="outline"
            size="sm"
            className="gap-1"
            onClick={onAddTag}
            disabled={!tagDraft.trim()}
          >
            <Plus className="size-3" />
            {t("tag.add")}
          </Button>
        </div>
      )}
      <div className="space-y-1">
        <h3 className="text-lg font-semibold">
          <HighlightedText text={detail.key} query={queryHighlight} />
        </h3>
        <p className="whitespace-pre-wrap text-sm">
          <HighlightedText text={detail.content} query={queryHighlight} />
        </p>
      </div>
      <Separator />
      <dl className="grid gap-3 text-sm sm:grid-cols-2">
        <div><dt className="text-muted-foreground">{t("detailCreatedAt")}</dt><dd>{formatTime(detail.createdAt)}</dd></div>
        <div><dt className="text-muted-foreground">{t("detailUpdatedAt")}</dt><dd>{formatTime(detail.updatedAt)}</dd></div>
        <div><dt className="text-muted-foreground">{t("detailLastAccessed")}</dt><dd>{formatTime(detail.lastAccessedAt)}</dd></div>
        <div><dt className="text-muted-foreground">{t("detailAccessCount")}</dt><dd>{t("accessed", { count: detail.accessCount })}</dd></div>
      </dl>
      <Separator />
      <div className="space-y-2">
        <h4 className="text-sm font-semibold">{t("detailMetadata")}</h4>
        {detail.metadataObject && Object.keys(detail.metadataObject).length > 0 ? (
          <pre className="overflow-auto rounded-lg border bg-muted/30 p-3 text-xs">{JSON.stringify(detail.metadataObject, null, 2)}</pre>
        ) : (
          <p className="text-sm text-muted-foreground">{t("detailNoMetadata")}</p>
        )}
      </div>
      <div className="space-y-2">
        <h4 className="text-sm font-semibold">{t("detailRelatedContext")}</h4>
        {detail.relatedContext && detail.relatedContext.length > 0 ? (
          detail.relatedContext.map((item) => (
            <div key={`${item.type}-${item.id}`} className="rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-sm">
              <div className="font-medium">{item.label ?? item.type}</div>
              <div className="text-xs text-muted-foreground">{item.id}</div>
            </div>
          ))
        ) : (
          <p className="text-sm text-muted-foreground">{t("detailNoRelatedContext")}</p>
        )}
      </div>
    </div>
  );
}
