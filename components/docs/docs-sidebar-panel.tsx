"use client";

import { useMemo } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { Clock3, Search, Star, Pin } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  flattenKnowledgeTree,
  type KnowledgeAssetTreeNode,
} from "@/lib/stores/knowledge-store";
import { buildDocsHref } from "@/lib/route-hrefs";
import { PageTree } from "./page-tree";

type FavoriteEntry = { assetId?: string; pageId?: string; userId: string; createdAt: string };
type RecentEntry = { assetId?: string; pageId?: string; userId: string; accessedAt: string };

function resolveId(entry: FavoriteEntry | RecentEntry): string {
  return ("assetId" in entry && entry.assetId) ? entry.assetId : (entry.pageId ?? "");
}

export function DocsSidebarPanel({
  query,
  onQueryChange,
  tree,
  currentPageId,
  favorites,
  recentAccess,
  onMovePage,
  onToggleFavorite,
  onTogglePinned,
  onDeletePage,
}: {
  query: string;
  onQueryChange: (query: string) => void;
  tree: KnowledgeAssetTreeNode[];
  currentPageId?: string | null;
  favorites: FavoriteEntry[];
  recentAccess: RecentEntry[];
  onMovePage?: (pageId: string, parentId: string | null, sortOrder: number) => void;
  onToggleFavorite?: (pageId: string, favorite: boolean) => void;
  onTogglePinned?: (pageId: string, pinned: boolean) => void;
  onDeletePage?: (pageId: string) => void;
}) {
  const t = useTranslations("docs");
  const flattened = useMemo(() => flattenKnowledgeTree(tree), [tree]);
  const favoritePages = flattened.filter((page) =>
    favorites.some((favorite) => resolveId(favorite) === page.id)
  );
  const recentPages = recentAccess
    .map((access) => flattened.find((page) => page.id === resolveId(access)))
    .filter((page): page is NonNullable<typeof page> => Boolean(page));
  const filteredTree = query.trim()
    ? flattened
        .filter((page) => page.title.toLowerCase().includes(query.toLowerCase()))
        .map((page) => ({ ...page, children: [] }))
    : tree;

  return (
    <aside className="flex flex-col gap-4 rounded-2xl border border-border/60 bg-card/70 p-4">
      <div className="space-y-2">
        <div className="flex items-center gap-2 text-sm font-medium">
          <Search className="size-4 text-muted-foreground" />
          {t("sidebar.searchDocs")}
        </div>
        <Input
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={t("sidebar.searchPlaceholder")}
        />
      </div>

      <section className="space-y-2">
        <h2 className="text-sm font-semibold">{t("sidebar.pageTree")}</h2>
        <PageTree
          nodes={filteredTree as KnowledgeAssetTreeNode[]}
          currentPageId={currentPageId}
          onMovePage={onMovePage}
          onToggleFavorite={onToggleFavorite}
          onTogglePinned={onTogglePinned}
          onDeletePage={onDeletePage}
        />
      </section>

      <section className="space-y-2">
        <div className="flex items-center gap-2 text-sm font-semibold">
          <Pin className="size-4 text-muted-foreground" />
          {t("sidebar.favorites")}
        </div>
        {favoritePages.map((page) => (
          <Link key={page.id} href={buildDocsHref(page.id)} className="flex items-center gap-2 text-sm hover:text-primary">
            <Star className="size-4 text-muted-foreground" />
            <span className="truncate">{page.title}</span>
          </Link>
        ))}
        {favoritePages.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t("sidebar.noFavorites")}</p>
        ) : null}
      </section>

      <section className="space-y-2">
        <div className="flex items-center gap-2 text-sm font-semibold">
          <Clock3 className="size-4 text-muted-foreground" />
          {t("sidebar.recent")}
        </div>
        {recentPages.map((page) => (
          <Link key={page.id} href={buildDocsHref(page.id)} className="truncate text-sm hover:text-primary">
            {page.title}
          </Link>
        ))}
        {recentPages.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t("sidebar.noRecent")}</p>
        ) : null}
      </section>
    </aside>
  );
}
