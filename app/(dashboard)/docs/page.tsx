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
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { flattenDocsTree, useDocsStore } from "@/lib/stores/docs-store";
import { buildDocsHref } from "@/lib/route-hrefs";
import { DocsSidebarPanel } from "@/components/docs/docs-sidebar-panel";
import { TemplateCenter } from "@/components/docs/template-center";
import { TemplatePicker } from "@/components/docs/template-picker";
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
    createPage,
    createTemplate,
    createPageFromTemplate,
    duplicateTemplate,
    deleteTemplate,
    movePage,
    toggleFavorite,
    togglePinned,
  } = useDocsStore();
  const [query, setQuery] = useState("");
  const [pickerOpen, setPickerOpen] = useState(
    () => requestedAction === "use-template" && Boolean(activeProjectId),
  );
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const pageId = searchParams.get("pageId");

  useEffect(() => {
    if (!activeProjectId) return;
    useDocsStore.getState().setProjectId(activeProjectId);
    void fetchTree(activeProjectId);
    void fetchTemplates(activeProjectId);
    void fetchFavorites(activeProjectId);
    void fetchRecentAccess(activeProjectId);
  }, [activeProjectId, fetchFavorites, fetchRecentAccess, fetchTemplates, fetchTree]);

  const allPages = useMemo(() => flattenDocsTree(tree), [tree]);
  const pinnedPages = useMemo(() => allPages.filter((page) => page.isPinned), [allPages]);
  const templateDestinations = useMemo(
    () =>
      allPages
        .filter((page) => !page.isTemplate)
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
      <div className="flex flex-col gap-4">
        <PageHeader title={t("title")} />
        <EmptyState
          icon={FolderOpen}
          title={t("selectProject")}
        />
      </div>
    );
  }

  return (
    <div className="grid gap-6 xl:grid-cols-[320px_minmax(0,1fr)]">
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

      <div className="flex flex-col gap-6">
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

        <div className="grid gap-4 lg:grid-cols-3">
          <section className="rounded-2xl border border-border/60 bg-card/70 p-4">
            <h2 className="text-base font-semibold">{t("pinned")}</h2>
            <div className="mt-3 flex flex-col gap-2">
              {pinnedPages.map((page) => (
                <Link key={page.id} href={buildDocsHref(page.id)} className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40">
                  {page.title}
                </Link>
              ))}
              {pinnedPages.length === 0 ? (
                <EmptyState icon={FileText} title={t("noPinned")} className="py-6" />
              ) : null}
            </div>
          </section>

          <section className="rounded-2xl border border-border/60 bg-card/70 p-4">
            <h2 className="text-base font-semibold">{t("favorites")}</h2>
            <div className="mt-3 flex flex-col gap-2">
              {favorites.map((favorite) => {
                const page = allPages.find((item) => item.id === favorite.pageId);
                if (!page) return null;
                return (
                  <Link key={favorite.pageId} href={buildDocsHref(favorite.pageId)} className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40">
                    {page.title}
                  </Link>
                );
              })}
              {favorites.length === 0 ? (
                <EmptyState icon={FileText} title={t("noFavorites")} className="py-6" />
              ) : null}
            </div>
          </section>

          <section className="rounded-2xl border border-border/60 bg-card/70 p-4">
            <h2 className="text-base font-semibold">{t("recent")}</h2>
            <div className="mt-3 flex flex-col gap-2">
              {recentAccess.map((access) => {
                const page = allPages.find((item) => item.id === access.pageId);
                if (!page) return null;
                return (
                  <Link key={`${access.pageId}-${access.accessedAt}`} href={buildDocsHref(access.pageId)} className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40">
                    {page.title}
                  </Link>
                );
              })}
              {recentAccess.length === 0 ? (
                <EmptyState icon={FileText} title={t("noRecent")} className="py-6" />
              ) : null}
            </div>
          </section>
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

        <section className="rounded-2xl border border-border/60 bg-card/70 p-4">
          <div className="flex items-center gap-2">
            <FileText className="size-4 text-muted-foreground" />
            <h2 className="text-base font-semibold">{t("allPages")}</h2>
          </div>
          <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
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
        </section>
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
