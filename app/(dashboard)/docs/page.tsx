"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { FileText, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { flattenDocsTree, useDocsStore } from "@/lib/stores/docs-store";
import { buildDocsHref } from "@/lib/route-hrefs";
import { DocsSidebarPanel } from "@/components/docs/docs-sidebar-panel";
import { TemplateCenter } from "@/components/docs/template-center";
import { TemplatePicker } from "@/components/docs/template-picker";
import { DocsPageDetailClient } from "./[pageId]/page-client";

export default function DocsLandingPage() {
  const t = useTranslations("docs");
  const searchParams = useSearchParams();
  const selectedProjectId = useDashboardStore((state) => state.selectedProjectId);
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
    createPageFromTemplate,
    movePage,
    toggleFavorite,
    togglePinned,
  } = useDocsStore();
  const [query, setQuery] = useState("");
  const [pickerOpen, setPickerOpen] = useState(false);
  const pageId = searchParams.get("pageId");

  useEffect(() => {
    if (!selectedProjectId) return;
    useDocsStore.getState().setProjectId(selectedProjectId);
    void fetchTree(selectedProjectId);
    void fetchTemplates(selectedProjectId);
    void fetchFavorites(selectedProjectId);
    void fetchRecentAccess(selectedProjectId);
  }, [fetchFavorites, fetchRecentAccess, fetchTemplates, fetchTree, selectedProjectId]);

  const allPages = useMemo(() => flattenDocsTree(tree), [tree]);
  const pinnedPages = useMemo(() => allPages.filter((page) => page.isPinned), [allPages]);

  if (pageId) {
    return <DocsPageDetailClient pageId={pageId} />;
  }

  if (!selectedProjectId) {
    return (
      <div className="flex flex-col gap-4">
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">
          {t("selectProject")}
        </p>
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
          void movePage({ projectId: selectedProjectId, pageId: targetPageId, parentId, sortOrder })
        }
        onToggleFavorite={(targetPageId, favorite) =>
          void toggleFavorite({ projectId: selectedProjectId, pageId: targetPageId, favorite })
        }
        onTogglePinned={(targetPageId, pinned) =>
          void togglePinned({ projectId: selectedProjectId, pageId: targetPageId, pinned })
        }
      />

      <div className="flex flex-col gap-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-3xl font-semibold">{t("title")}</h1>
            <p className="text-sm text-muted-foreground">
              {t("subtitle")}
            </p>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setPickerOpen(true)}>
              {t("useTemplate")}
            </Button>
            <Button
              onClick={() =>
                void createPage({
                  projectId: selectedProjectId,
                  title: t("untitledDoc"),
                })
              }
            >
              <Plus className="mr-1 size-4" />
              {t("newPage")}
            </Button>
          </div>
        </div>

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
                <p className="text-sm text-muted-foreground">{t("noPinned")}</p>
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
                <p className="text-sm text-muted-foreground">{t("noFavorites")}</p>
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
                <p className="text-sm text-muted-foreground">{t("noRecent")}</p>
              ) : null}
            </div>
          </section>
        </div>

        <TemplateCenter
          templates={templates}
          onCreateFromTemplate={(templateId) =>
            void createPageFromTemplate({
              projectId: selectedProjectId,
              templateId,
              title: t("newFromTemplate"),
            })
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
        onPick={(templateId) => {
          void createPageFromTemplate({
            projectId: selectedProjectId,
            templateId,
            title: t("newFromTemplate"),
          });
          setPickerOpen(false);
        }}
      />
    </div>
  );
}