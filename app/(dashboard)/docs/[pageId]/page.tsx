"use client";

import { use, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useDocsStore } from "@/lib/stores/docs-store";
import { DocsSidebarPanel } from "@/components/docs/docs-sidebar-panel";
import { BlockEditor } from "@/components/docs/block-editor";
import { CommentsPanel } from "@/components/docs/comments-panel";
import { EditorToolbar } from "@/components/docs/editor-toolbar";
import { TemplatePicker } from "@/components/docs/template-picker";
import { VersionHistoryPanel } from "@/components/docs/version-history-panel";
import { VersionViewer } from "@/components/docs/version-viewer";

export default function DocsPageDetail({
  params,
}: {
  params: Promise<{ pageId: string }>;
}) {
  const { pageId } = use(params);
  const router = useRouter();
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const {
    projectId,
    tree,
    currentPage,
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
  } = useDocsStore();
  const [query, setQuery] = useState("");
  const [selectedVersionId, setSelectedVersionId] = useState<string | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [readonly, setReadonly] = useState(false);

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
    };

    void hydratePage();

    return () => {
      cancelled = true;
    };
  }, [fetchPageWorkspace, fetchTree, pageId, resolvePageContext, selectedProjectId, setProjectId]);

  const selectedVersion = useMemo(
    () => versions.find((version) => version.id === selectedVersionId) ?? versions[0] ?? null,
    [selectedVersionId, versions]
  );
  const displayContent = readonly && selectedVersion ? selectedVersion.content : currentPage?.content ?? "[]";
  const commentedBlockIds = useMemo(
    () =>
      comments
        .map((comment) => comment.anchorBlockId)
        .filter((value): value is string => Boolean(value)),
    [comments]
  );

  if (loading && !currentPage) {
    return <p className="text-sm text-muted-foreground">Loading document…</p>;
  }

  if (!currentPage) {
    return (
      <div className="flex flex-col gap-3">
        <h1 className="text-2xl font-bold">Document not found</h1>
        <Button variant="outline" onClick={() => router.push("/docs")}>
          Back to Docs
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
        currentPageId={currentPage.id}
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
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-3xl font-semibold">{currentPage.title}</h1>
            <p className="text-sm text-muted-foreground">
              {currentPage.path} · last updated{" "}
              {new Date(
                readonly && selectedVersion ? selectedVersion.createdAt : currentPage.updatedAt
              ).toLocaleString()}
            </p>
          </div>
          <div className="flex gap-2">
            {!readonly ? (
              <Button variant="outline" onClick={() => setPickerOpen(true)}>
                New from Template
              </Button>
            ) : null}
            <Button
              variant="outline"
              onClick={() =>
                void navigator.clipboard.writeText(`${window.location.origin}/docs/${currentPage.id}`)
              }
            >
              <Copy className="mr-1 size-4" />
              Copy Link
            </Button>
          </div>
        </div>

        <EditorToolbar
          readonly={readonly}
          saving={saving}
          onSaveVersion={() =>
            void createVersion({
              projectId: projectId ?? selectedProjectId ?? "",
              pageId: currentPage.id,
              name: `Snapshot ${new Date().toLocaleTimeString()}`,
            })
          }
          onSaveTemplate={() =>
            void createTemplateFromPage({
              projectId: projectId ?? selectedProjectId ?? "",
              pageId: currentPage.id,
              name: `${currentPage.title} Template`,
              category: currentPage.templateCategory || "custom",
            })
          }
          onShareVersion={() => {
            const versionId = selectedVersion?.id;
            if (!versionId) return;
            void navigator.clipboard.writeText(
              `${window.location.origin}/docs/${currentPage.id}?version=${versionId}&readonly=1`
            );
          }}
        />

        <BlockEditor
          value={displayContent}
          editable={!readonly}
          commentedBlockIds={commentedBlockIds}
          onChange={(content, contentText) =>
            readonly
              ? undefined
              : void updatePage({
                  projectId: projectId ?? selectedProjectId ?? "",
                  pageId: currentPage.id,
                  title: currentPage.title,
                  content,
                  contentText,
                  expectedUpdatedAt: currentPage.updatedAt,
                })
          }
        />
      </div>

      <div className="flex flex-col gap-4">
        <VersionHistoryPanel
          readonly={readonly}
          versions={versions}
          selectedVersionId={selectedVersionId}
          onSelect={setSelectedVersionId}
          onRestore={(versionId) =>
            void restoreVersion({
              projectId: projectId ?? selectedProjectId ?? "",
              pageId: currentPage.id,
              versionId,
            })
          }
          onShare={(versionId) =>
            void navigator.clipboard.writeText(
              `${window.location.origin}/docs/${currentPage.id}?version=${versionId}&readonly=1`
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
              pageId: currentPage.id,
              body,
              mentions: "[]",
            })
          }
          onResolve={(commentId) =>
            void setCommentResolved({
              projectId: projectId ?? selectedProjectId ?? "",
              pageId: currentPage.id,
              commentId,
              resolved: true,
            })
          }
          onReopen={(commentId) =>
            void setCommentResolved({
              projectId: projectId ?? selectedProjectId ?? "",
              pageId: currentPage.id,
              commentId,
              resolved: false,
            })
          }
          onCopyLink={(commentId) =>
            void navigator.clipboard.writeText(
              `${window.location.origin}/docs/${currentPage.id}#comment-${commentId}`
            )
          }
          mentionSuggestions={["alice", "bob", "carol"]}
        />
      </div>

      <TemplatePicker
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        templates={templates}
        onPick={(templateId) => {
          void createPageFromTemplate({
            projectId: projectId ?? selectedProjectId ?? "",
            templateId,
            title: "New document from template",
          });
          setPickerOpen(false);
        }}
      />
    </div>
  );
}
