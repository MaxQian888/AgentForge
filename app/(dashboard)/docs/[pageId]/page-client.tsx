"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Copy } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { createApiClient } from "@/lib/api-client";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { flattenKnowledgeTree, useKnowledgeStore } from "@/lib/stores/knowledge-store";
import { useAuthStore } from "@/lib/stores/auth-store";
import { buildDocsHref } from "@/lib/route-hrefs";
import { DocsSidebarPanel } from "@/components/docs/docs-sidebar-panel";
import { BlockEditor } from "@/components/docs/block-editor";
import { CommentsPanel } from "@/components/docs/comments-panel";
import { DecomposeTasksDialog } from "@/components/docs/decompose-tasks-dialog";
import { EditorToolbar } from "@/components/docs/editor-toolbar";
import { RelatedTasksPanel, type RelatedTaskItem } from "@/components/docs/related-tasks-panel";
import { TaskLinkPicker } from "@/components/docs/task-link-picker";
import { TemplatePicker } from "@/components/docs/template-picker";
import { VersionHistoryPanel } from "@/components/docs/version-history-panel";
import { VersionViewer } from "@/components/docs/version-viewer";
import { BacklinksPanel, type BacklinkItem } from "@/components/shared/backlinks-panel";
import { MaterializedFromPill } from "@/components/knowledge/MaterializedFromPill";
import { SourceUpdatedBanner } from "@/components/knowledge/SourceUpdatedBanner";
import { useEntityLinkStore } from "@/lib/stores/entity-link-store";
import { useTaskStore } from "@/lib/stores/task-store";

export function DocsPageDetailClient({ pageId }: { pageId: string }) {
  const t = useTranslations("docs");
  const router = useRouter();
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const {
    projectId,
    tree,
    currentAsset,
    comments,
    versions,
    templates,
    favorites,
    recentAccess,
    loading,
    saving,
    resolvePageContext,
    setProjectId,
    fetchTree,
    fetchPageWorkspace,
    createPageFromTemplate,
    createTemplateFromPage,
    createVersion,
    restoreVersion,
    createComment,
    setCommentResolved,
    movePage,
    toggleFavorite,
    togglePinned,
    updatePage,
  } = useKnowledgeStore();
  const [query, setQuery] = useState("");
  const [selectedVersionId, setSelectedVersionId] = useState<string | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [taskPickerOpen, setTaskPickerOpen] = useState(false);
  const [decomposeOpen, setDecomposeOpen] = useState(false);
  const [preselectedBlockIds, setPreselectedBlockIds] = useState<string[]>([]);
  const [readonly, setReadonly] = useState(false);
  const entityLinks = useEntityLinkStore(
    (state) => (currentAsset ? state.linksByEntity[`wiki_page:${currentAsset.id}`] ?? [] : []),
  );
  const fetchEntityLinks = useEntityLinkStore((state) => state.fetchLinks);
  const createEntityLink = useEntityLinkStore((state) => state.createLink);
  const deleteEntityLink = useEntityLinkStore((state) => state.deleteLink);
  const tasks = useTaskStore((state) => state.tasks);
  const fetchTasks = useTaskStore((state) => state.fetchTasks);

  useEffect(() => {
    let cancelled = false;

    const hydratePage = async () => {
      const urlParams =
        typeof window === "undefined"
          ? new URLSearchParams()
          : new URLSearchParams(window.location.search);
      const requestedVersionId = urlParams.get("version");
      const isReadonly = urlParams.get("readonly") === "1";

      if (!cancelled) {
        setSelectedVersionId(requestedVersionId);
        setReadonly(isReadonly);
      }

      const resolvedProjectId = await resolvePageContext(pageId);
      if (!resolvedProjectId || cancelled) {
        return;
      }
      setProjectId(resolvedProjectId);
      void fetchTree(resolvedProjectId);
      void fetchPageWorkspace(resolvedProjectId, pageId);
      void fetchTasks(resolvedProjectId);
    };

    void hydratePage();

    return () => {
      cancelled = true;
    };
  }, [fetchPageWorkspace, fetchTasks, fetchTree, pageId, resolvePageContext, selectedProjectId, setProjectId]);

  useEffect(() => {
    if (!(projectId ?? selectedProjectId) || !currentAsset) {
      return;
    }
    void fetchEntityLinks(projectId ?? selectedProjectId ?? "", "wiki_page", currentAsset.id);
  }, [currentAsset, fetchEntityLinks, projectId, selectedProjectId]);

  const selectedVersion = useMemo(
    () => versions.find((version) => version.id === selectedVersionId) ?? versions[0] ?? null,
    [selectedVersionId, versions]
  );
  const templateReadonly = currentAsset?.kind === "template" && currentAsset.canEdit === false;
  const effectiveReadonly = readonly || Boolean(templateReadonly);
  const displayContent = effectiveReadonly && selectedVersion ? selectedVersion.contentJson ?? "[]" : currentAsset?.contentJson ?? "[]";
  const commentedBlockIds = useMemo(
    () =>
      comments
        .map((comment) => comment.anchorBlockId)
        .filter((value): value is string => Boolean(value)),
    [comments]
  );
  const relatedTasks = useMemo<RelatedTaskItem[]>(() => {
    return entityLinks
      .filter((link) => link.targetType === "task" || link.sourceType === "task")
        .map((link) => {
          const taskId = link.sourceType === "task" ? link.sourceId : link.targetId;
          const task = tasks.find((item) => item.id === taskId);
          return {
            linkId: link.id,
            taskId,
          title: task?.title ?? taskId,
          status: task?.status ?? "unknown",
          assigneeName: task?.assigneeName ?? null,
          dueDate: task?.plannedEndAt ?? null,
        };
      });
  }, [entityLinks, tasks]);
  const currentAssetId = currentAsset?.id ?? null;
  const backlinks = useMemo<BacklinkItem[]>(
    () =>
      currentAssetId
        ? entityLinks
            .filter(
              (link) =>
                link.linkType === "mention" &&
                link.targetType === "wiki_page" &&
                link.targetId === currentAssetId,
            )
            .map((link) => ({
              linkId: link.id,
              entityId: link.sourceId,
              entityType: link.sourceType,
              title:
                link.sourceType === "task"
                  ? tasks.find((task) => task.id === link.sourceId)?.title ?? link.sourceId
                  : link.sourceId,
            }))
        : [],
    [currentAssetId, entityLinks, tasks],
  );
  const availableBlocks = useMemo(() => {
    try {
      const parsed = JSON.parse(currentAsset?.contentJson ?? "[]") as Array<Record<string, unknown>>;
      return parsed
        .map((block) => {
          const content = block.content;
          const text =
            typeof content === "string"
              ? content
              : Array.isArray(content)
                ? content
                    .map((item) =>
                      typeof item === "object" && item && "text" in item
                        ? String((item as { text?: unknown }).text ?? "")
                        : "",
                    )
                    .join(" ")
                : "";
          return {
            id: String(block.id ?? ""),
            text,
          };
        })
        .filter((block) => block.id);
    } catch {
      return [];
    }
  }, [currentAsset]);
  const blockTaskCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const link of entityLinks) {
      if (link.linkType !== "requirement" || !link.anchorBlockId) {
        continue;
      }
      counts.set(link.anchorBlockId, (counts.get(link.anchorBlockId) ?? 0) + 1);
    }
    return counts;
  }, [entityLinks]);
  const templateDestinations = useMemo(
    () =>
      flattenKnowledgeTree(tree)
        .filter((page) => page.kind !== "template")
        .map((page) => ({
          id: page.id,
          title: page.title,
        })),
    [tree],
  );

  if (loading && !currentAsset) {
    return <p className="text-sm text-muted-foreground">{t("pageDetail.loading")}</p>;
  }

  if (!currentAsset) {
    return (
      <div className="flex flex-col gap-3">
        <h1 className="text-2xl font-bold">{t("pageDetail.notFound")}</h1>
        <Button variant="outline" onClick={() => router.push("/docs")}>
          {t("pageDetail.backToDocs")}
        </Button>
      </div>
    );
  }

  return (
    <div className="grid gap-6 xl:grid-cols-[320px_minmax(0,1fr)_360px]">
      <DocsSidebarPanel
        query={query}
        onQueryChange={setQuery}
        tree={tree}
        currentPageId={currentAsset.id}
        favorites={favorites}
        recentAccess={recentAccess}
        onMovePage={(movedPageId, parentId, sortOrder) =>
          void movePage({
            projectId: projectId ?? selectedProjectId ?? "",
            pageId: movedPageId,
            parentId,
            sortOrder,
          })
        }
        onToggleFavorite={(targetPageId, favorite) =>
          void toggleFavorite({
            projectId: projectId ?? selectedProjectId ?? "",
            pageId: targetPageId,
            favorite,
          })
        }
        onTogglePinned={(targetPageId, pinned) =>
          void togglePinned({
            projectId: projectId ?? selectedProjectId ?? "",
            pageId: targetPageId,
            pinned,
          })
        }
      />

      <div className="flex flex-col gap-4">
        {currentAsset.kind === "template" ? (
          <div className="rounded-xl border border-border/60 bg-card/70 p-4">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="secondary">
                {currentAsset.templateSource === "system"
                  ? t("templateMode.systemBadge")
                  : t("templateMode.customBadge")}
              </Badge>
              <span className="text-sm font-medium">{t("templateMode.title")}</span>
            </div>
            <p className="mt-2 text-sm text-muted-foreground">
              {templateReadonly
                ? t("templateMode.systemDesc")
                : t("templateMode.customDesc")}
            </p>
          </div>
        ) : null}

        {currentAsset.sourceUpdatedSinceMaterialize ? <SourceUpdatedBanner /> : null}

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-3xl font-semibold">{currentAsset.title}</h1>
              {currentAsset.materializedFromId ? (
                <MaterializedFromPill sourceAssetId={currentAsset.materializedFromId} />
              ) : null}
            </div>
            <p className="text-sm text-muted-foreground">
              {currentAsset.path ?? ""} · {t("pageDetail.lastUpdated")}{" "}
              {new Date(
                readonly && selectedVersion ? selectedVersion.createdAt : currentAsset.updatedAt
              ).toLocaleString()}
            </p>
          </div>
          <div className="flex gap-2">
            {!readonly ? (
              <Button variant="outline" onClick={() => setPickerOpen(true)}>
                {t("pageDetail.newFromTemplate")}
              </Button>
            ) : null}
            {!effectiveReadonly ? (
              <Button variant="outline" onClick={() => setDecomposeOpen(true)}>
                {t("pageDetail.createTasks")}
              </Button>
            ) : null}
            <Button
              variant="outline"
              onClick={() =>
                void navigator.clipboard.writeText(`${window.location.origin}${buildDocsHref(currentAsset.id)}`)
              }
            >
              <Copy className="mr-1 size-4" />
              {t("pageDetail.copyLink")}
            </Button>
          </div>
        </div>

        <EditorToolbar
          readonly={effectiveReadonly}
          saving={saving}
          onSaveVersion={() =>
            void createVersion({
              projectId: projectId ?? selectedProjectId ?? "",
              assetId: currentAsset.id,
              name: `Snapshot ${new Date().toLocaleTimeString()}`,
            })
          }
          onSaveTemplate={() =>
            void createTemplateFromPage({
              projectId: projectId ?? selectedProjectId ?? "",
              pageId: currentAsset.id,
              name: currentAsset.kind === "template" ? `${currentAsset.title} Copy` : `${currentAsset.title} Template`,
              category: currentAsset.templateCategory || "custom",
            })
          }
          templateActionLabel={
            currentAsset.kind === "template" ? t("editor.duplicateTemplate") : undefined
          }
          templateActionDisabled={currentAsset.kind === "template" ? false : effectiveReadonly}
          onShareVersion={() => {
            const versionId = selectedVersion?.id;
            if (!versionId) return;
            void navigator.clipboard.writeText(
              `${window.location.origin}${buildDocsHref(currentAsset.id)}?version=${versionId}&readonly=1`
            );
          }}
        />

        <BlockEditor
          value={displayContent}
          editable={!effectiveReadonly}
          commentedBlockIds={commentedBlockIds}
          taskCountsByBlock={Object.fromEntries(blockTaskCounts.entries())}
          onCreateTasksFromSelection={(blockIds) => {
            setPreselectedBlockIds(blockIds);
            setDecomposeOpen(true);
          }}
          onChange={(content, contentText) =>
            effectiveReadonly
              ? undefined
              : void updatePage({
                  projectId: projectId ?? selectedProjectId ?? "",
                  pageId: currentAsset.id,
                  title: currentAsset.title,
                  content,
                  contentText,
                  expectedUpdatedAt: currentAsset.updatedAt,
                  templateCategory: currentAsset.kind === "template"
                    ? currentAsset.templateCategory || "custom"
                    : undefined,
                })
          }
        />

        {availableBlocks.length > 0 ? (
          <div className="rounded-xl border border-border/60 bg-card/70 p-4">
            <h2 className="text-base font-semibold">{t("pageDetail.blockTaskCounts")}</h2>
            <div className="mt-3 space-y-2">
              {availableBlocks.map((block) => (
                <div
                  key={block.id}
                  className="flex items-center justify-between rounded-lg border border-border/60 bg-background px-3 py-2"
                >
                  <div className="min-w-0">
                    <div className="font-medium">{block.id}</div>
                    <div className="text-xs text-muted-foreground">{block.text}</div>
                  </div>
                  <Button type="button" size="sm" variant="outline">
                    {t("pageDetail.tasks", { count: blockTaskCounts.get(block.id) ?? 0 })}
                  </Button>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </div>

      <div className="flex flex-col gap-4">
        <RelatedTasksPanel
          tasks={relatedTasks}
          onAddTask={() => setTaskPickerOpen(true)}
          onRemoveTask={(linkId) =>
            void deleteEntityLink(projectId ?? selectedProjectId ?? "", "wiki_page", currentAsset.id, linkId)
          }
        />
        <BacklinksPanel items={backlinks} />
        <VersionHistoryPanel
          readonly={effectiveReadonly}
          versions={versions}
          selectedVersionId={selectedVersionId}
          onSelect={setSelectedVersionId}
          onRestore={(versionId) =>
            void restoreVersion({
              projectId: projectId ?? selectedProjectId ?? "",
              assetId: currentAsset.id,
              versionId,
            })
          }
          onShare={(versionId) =>
            void navigator.clipboard.writeText(
              `${window.location.origin}${buildDocsHref(currentAsset.id)}?version=${versionId}&readonly=1`
            )
          }
        />
        <VersionViewer version={selectedVersion} />
        <CommentsPanel
          readonly={readonly}
          comments={comments}
          onCreateComment={(body) =>
            createComment({
              projectId: projectId ?? selectedProjectId ?? "",
              assetId: currentAsset.id,
              body,
              mentions: "[]",
            })
          }
          onResolve={(commentId) =>
            void setCommentResolved({
              projectId: projectId ?? selectedProjectId ?? "",
              assetId: currentAsset.id,
              commentId,
              resolved: true,
            })
          }
          onReopen={(commentId) =>
            void setCommentResolved({
              projectId: projectId ?? selectedProjectId ?? "",
              assetId: currentAsset.id,
              commentId,
              resolved: false,
            })
          }
          onCopyLink={(commentId) =>
            void navigator.clipboard.writeText(
              `${window.location.origin}${buildDocsHref(currentAsset.id)}#comment-${commentId}`
            )
          }
          mentionSuggestions={["alice", "bob", "carol"]}
        />
      </div>

      <TemplatePicker
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        templates={templates}
        destinations={templateDestinations}
        initialTemplateId={currentAsset.kind === "template" ? currentAsset.id : undefined}
        defaultTitle={t("newFromTemplate")}
        onPick={({ templateId, title, parentId }) => {
          void createPageFromTemplate({
            projectId: projectId ?? selectedProjectId ?? "",
            templateId,
            title,
            parentId,
          });
          setPickerOpen(false);
        }}
      />
      <TaskLinkPicker
        open={taskPickerOpen}
        onOpenChange={setTaskPickerOpen}
        tasks={tasks.map((task) => ({
          id: task.id,
          title: task.title,
          status: task.status,
        }))}
        onPick={(taskId) => {
          void createEntityLink({
            projectId: projectId ?? selectedProjectId ?? "",
            sourceType: "wiki_page",
            sourceId: currentAsset.id,
            targetType: "task",
            targetId: taskId,
            linkType: "design",
          });
          setTaskPickerOpen(false);
        }}
      />
      <DecomposeTasksDialog
        open={decomposeOpen}
        onOpenChange={setDecomposeOpen}
        blocks={availableBlocks}
        tasks={tasks.map((task) => ({ id: task.id, title: task.title }))}
        initialBlockIds={preselectedBlockIds}
        onConfirm={({ blockIds, parentTaskId }) => {
          const token = useAuthStore.getState().accessToken;
          if (!token) {
            return;
          }
          const api = createApiClient(process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777");
          void api.post(
            `/api/v1/projects/${projectId ?? selectedProjectId ?? ""}/knowledge/assets/${currentAsset.id}/decompose-tasks`,
            {
              blockIds,
              parentTaskId: parentTaskId ?? undefined,
            },
            { token },
          );
          setDecomposeOpen(false);
        }}
      />
    </div>
  );
}
