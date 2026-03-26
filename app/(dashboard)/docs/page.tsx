"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { FileText, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { flattenDocsTree, useDocsStore } from "@/lib/stores/docs-store";
import { DocsSidebarPanel } from "@/components/docs/docs-sidebar-panel";
import { TemplateCenter } from "@/components/docs/template-center";
import { TemplatePicker } from "@/components/docs/template-picker";

export default function DocsLandingPage() {
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

  if (!selectedProjectId) {
    return (
      <div className="flex flex-col gap-4">
        <h1 className="text-2xl font-bold">Docs Workspace</h1>
        <p className="text-sm text-muted-foreground">
          Select a project from the dashboard to open its wiki space.
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
        onMovePage={(pageId, parentId, sortOrder) =>
          void movePage({ projectId: selectedProjectId, pageId, parentId, sortOrder })
        }
        onToggleFavorite={(pageId, favorite) =>
          void toggleFavorite({ projectId: selectedProjectId, pageId, favorite })
        }
        onTogglePinned={(pageId, pinned) =>
          void togglePinned({ projectId: selectedProjectId, pageId, pinned })
        }
      />

      <div className="flex flex-col gap-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-3xl font-semibold">Docs Workspace</h1>
            <p className="text-sm text-muted-foreground">
              Capture PRDs, ADRs, runbooks, onboarding notes, and execution briefs next to the work.
            </p>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setPickerOpen(true)}>
              Use Template
            </Button>
            <Button
              onClick={() =>
                void createPage({
                  projectId: selectedProjectId,
                  title: "Untitled doc",
                })
              }
            >
              <Plus className="mr-1 size-4" />
              New Page
            </Button>
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-3">
          <section className="rounded-2xl border border-border/60 bg-card/70 p-4">
            <h2 className="text-base font-semibold">Pinned</h2>
            <div className="mt-3 flex flex-col gap-2">
              {pinnedPages.map((page) => (
                <Link key={page.id} href={`/docs/${page.id}`} className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40">
                  {page.title}
                </Link>
              ))}
              {pinnedPages.length === 0 ? (
                <p className="text-sm text-muted-foreground">No pinned pages yet.</p>
              ) : null}
            </div>
          </section>

          <section className="rounded-2xl border border-border/60 bg-card/70 p-4">
            <h2 className="text-base font-semibold">Favorites</h2>
            <div className="mt-3 flex flex-col gap-2">
              {favorites.map((favorite) => {
                const page = allPages.find((item) => item.id === favorite.pageId);
                if (!page) return null;
                return (
                  <Link key={favorite.pageId} href={`/docs/${favorite.pageId}`} className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40">
                    {page.title}
                  </Link>
                );
              })}
              {favorites.length === 0 ? (
                <p className="text-sm text-muted-foreground">Mark a page as favorite to keep it here.</p>
              ) : null}
            </div>
          </section>

          <section className="rounded-2xl border border-border/60 bg-card/70 p-4">
            <h2 className="text-base font-semibold">Recent</h2>
            <div className="mt-3 flex flex-col gap-2">
              {recentAccess.map((access) => {
                const page = allPages.find((item) => item.id === access.pageId);
                if (!page) return null;
                return (
                  <Link key={`${access.pageId}-${access.accessedAt}`} href={`/docs/${access.pageId}`} className="rounded-lg border border-border/60 px-3 py-2 hover:bg-accent/40">
                    {page.title}
                  </Link>
                );
              })}
              {recentAccess.length === 0 ? (
                <p className="text-sm text-muted-foreground">Recently opened pages show up here.</p>
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
              title: "New document from template",
            })
          }
        />

        <section className="rounded-2xl border border-border/60 bg-card/70 p-4">
          <div className="flex items-center gap-2">
            <FileText className="size-4 text-muted-foreground" />
            <h2 className="text-base font-semibold">All Pages</h2>
          </div>
          <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {allPages.map((page) => (
              <Link
                key={page.id}
                href={`/docs/${page.id}`}
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
            title: "New document from template",
          });
          setPickerOpen(false);
        }}
      />
    </div>
  );
}
