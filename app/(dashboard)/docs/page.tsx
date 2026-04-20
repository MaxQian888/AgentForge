"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { FileText, FolderOpen, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { SectionCard } from "@/components/shared/section-card";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { flattenKnowledgeTree, useKnowledgeStore } from "@/lib/stores/knowledge-store";
import { buildDocsHref } from "@/lib/route-hrefs";
import { DocsSidebarPanel } from "@/components/docs/docs-sidebar-panel";
import { TemplateCenter } from "@/components/docs/template-center";
import { TemplatePicker } from "@/components/docs/template-picker";
import { IngestedFilesPane } from "@/components/knowledge/IngestedFilesPane";
import { KnowledgeSearch } from "@/components/knowledge/KnowledgeSearch";
import { DocsPageDetailClient } from "./[pageId]/page-client";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function DocsLandingPage() {
  useBreadcrumbs([{ label: "Configuration", href: "/" }, { label: "Docs" }]);
  const t = useTranslations("docs");
  const router = useRouter();
  const searchParams = useSearchParams();
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
  const requestedProjectId = searchParams.get("project");
  const requestedAction = searchParams.get("action");
  const activeProjectId = requestedProjectId ?? selectedProjectId;
  const {
    tree,
    templates,
    favorites,
    recentAccess,
    fetchTree,
    fetchTemplates,
    fetchFavorites,
    fetchRecentAccess,
    fetchIngestedFiles,
    materializeAsWiki,
    createPage,
    createTemplate,
    createPageFromTemplate,
    duplicateTemplate,
    deleteTemplate,
    movePage,
    toggleFavorite,
    togglePinned,
  } = useKnowledgeStore();
  const [query, setQuery] = useState("");
  const [pickerOpen, setPickerOpen] = useState(
    () => requestedAction === "use-template" && Boolean(activeProjectId),
  );
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const pageId = searchParams.get("pageId");

  useEffect(() => {
    if (!activeProjectId) return;
    useKnowledgeStore.getState().setProjectId(activeProjectId);
    void fetchTree(activeProjectId);
    void fetchTemplates(activeProjectId);
    void fetchFavorites(activeProjectId);
    void fetchRecentAccess(activeProjectId);
    void fetchIngestedFiles(activeProjectId);
  }, [activeProjectId, fetchFavorites, fetchIngestedFiles, fetchRecentAccess, fetchTemplates, fetchTree]);

  const allPages = useMemo(() => flattenKnowledgeTree(tree), [tree]);
  const pinnedPages = useMemo(() => allPages.filter((page) => page.isPinned), [allPages]);
  const templateDestinations = useMemo(
    () =>
      allPages
        .filter((page) => page.kind !== "template")
        .map((page) => ({
          id: page.id,
          title: page.title,
        })),
    [allPages],
  );

  if (pageId) {
    return <DocsPageDetailClient pageId={pageId} />;
  }

  if (!activeProjectId) {
    return (
      <div className="flex flex-col gap-[var(--space-stack-md)]">
        <PageHeader title={t("title")} />
        <EmptyState icon={FolderOpen} title={t("selectProject")} />
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] xl:grid-cols-[320px_minmax(0,1fr)]">
      <div className="flex flex-col gap-[var(--space-stack-md)]">
        <KnowledgeSearch projectId={activeProjectId} />
        <DocsSidebarPanel
          query={query}
          onQueryChange={setQuery}
          tree={tree}
          favorites={favorites}
          recentAccess={recentAccess}
          onMovePage={(targetPageId, parentId, sortOrder) =>
            void movePage({ projectId: activeProjectId, pageId: targetPageId, parentId, sortOrder })
          }
          onToggleFavorite={(targetPageId, favorite) =>
            void toggleFavorite({ projectId: activeProjectId, pageId: targetPageId, favorite })
          }
          onTogglePinned={(targetPageId, pinned) =>
            void togglePinned({ projectId: activeProjectId, pageId: targetPageId, pinned })
          }
        />
        <IngestedFilesPane
          projectId={activeProjectId}
          onMaterializeAsWiki={async (assetId) => {
            const page = await materializeAsWiki(activeProjectId, assetId);
            if (page) router.push(buildDocsHref(page.id));
          }}
        />
      </div>

      <div className="flex flex-col gap-[var(--space-section-gap)]">
        <PageHeader
          title={t("title")}
          description={t("subtitle")}
          actions={
            <>
              <Button variant="outline" onClick={() => setPickerOpen(true)}>
                {t("useTemplate")}
              </Button>
              <Button
                onClick={() =>
                  void createPage({
                    projectId: activeProjectId,
                    title: t("untitledDoc"),
                  })
                }
              >
                <Plus className="mr-1 size-4" />
                {t("newPage")}
              </Button>
            </>
          }
        />

        <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-3 xl:grid-cols-3">
          <SectionCard title={t("pinned")}>
            <div className="flex flex-col gap-[var(--space-stack-sm)]">
              {pinnedPages.map((page) => (
                <Link
                  key={page.id}
                  href={buildDocsHref(page.id)}
                  className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40"
                >
                  {page.title}
                </Link>
              ))}
              {pinnedPages.length === 0 ? (
                <EmptyState icon={FileText} title={t("noPinned")} className="py-[var(--space-card-padding)]" />
              ) : null}
            </div>
          </SectionCard>

          <SectionCard title={t("favorites")}>
            <div className="flex flex-col gap-[var(--space-stack-sm)]">
              {favorites.map((favorite) => {
                const page = allPages.find((item) => item.id === favorite.assetId);
                if (!page) return null;
                return (
                  <Link
                    key={favorite.assetId}
                    href={buildDocsHref(favorite.assetId)}
                    className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40"
                  >
                    {page.title}
                  </Link>
                );
              })}
              {favorites.length === 0 ? (
                <EmptyState icon={FileText} title={t("noFavorites")} className="py-[var(--space-card-padding)]" />
              ) : null}
            </div>
          </SectionCard>

          <SectionCard title={t("recent")}>
            <div className="flex flex-col gap-[var(--space-stack-sm)]">
              {recentAccess.map((access) => {
                const page = allPages.find((item) => item.id === access.assetId);
                if (!page) return null;
                return (
                  <Link
                    key={`${access.assetId}-${access.accessedAt}`}
                    href={buildDocsHref(access.assetId)}
                    className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40"
                  >
                    {page.title}
                  </Link>
                );
              })}
              {recentAccess.length === 0 ? (
                <EmptyState icon={FileText} title={t("noRecent")} className="py-[var(--space-card-padding)]" />
              ) : null}
            </div>
          </SectionCard>
        </div>

        <TemplateCenter
          templates={templates}
          onCreateFromTemplate={(templateId) => {
            setSelectedTemplateId(templateId);
            setPickerOpen(true);
          }}
          onCreateTemplate={async ({ title, category }) => {
            const template = await createTemplate({
              projectId: activeProjectId,
              title,
              category,
            });
            if (template) {
              router.push(buildDocsHref(template.id));
            }
          }}
          onEditTemplate={(templateId) => router.push(buildDocsHref(templateId))}
          onDuplicateTemplate={async ({ templateId, name, category }) => {
            const template = await duplicateTemplate({
              projectId: activeProjectId,
              templateId,
              name,
              category,
            });
            if (template) {
              router.push(buildDocsHref(template.id));
            }
          }}
          onDeleteTemplate={(templateId) =>
            deleteTemplate({ projectId: activeProjectId, templateId })
          }
        />

        <SectionCard
          title={
            <span className="flex items-center gap-2">
              <FileText className="size-4 text-muted-foreground" />
              {t("allPages")}
            </span>
          }
        >
          <div className="grid grid-cols-1 gap-[var(--space-stack-sm)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
            {allPages.map((page) => (
              <Link
                key={page.id}
                href={buildDocsHref(page.id)}
                className="rounded-xl border border-border/60 px-4 py-3 hover:bg-accent/40"
              >
                <div className="font-medium">{page.title}</div>
                <div className="text-xs text-muted-foreground">{page.path}</div>
              </Link>
            ))}
          </div>
        </SectionCard>
      </div>

      <TemplatePicker
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        templates={templates}
        destinations={templateDestinations}
        initialTemplateId={selectedTemplateId}
        defaultTitle={t("newFromTemplate")}
        onPick={({ templateId, title, parentId }) => {
          void createPageFromTemplate({
            projectId: activeProjectId,
            templateId,
            title,
            parentId,
          });
          setPickerOpen(false);
          setSelectedTemplateId(null);
        }}
      />
    </div>
  );
}
