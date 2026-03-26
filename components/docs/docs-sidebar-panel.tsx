"use client";

import { useMemo } from "react";
import Link from "next/link";
import { Clock3, Search, Star, Pin } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  flattenDocsTree,
  type DocsFavorite,
  type DocsPageTreeNode,
  type DocsRecentAccess,
} from "@/lib/stores/docs-store";
import { PageTree } from "./page-tree";

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
  tree: DocsPageTreeNode[];
  currentPageId?: string | null;
  favorites: DocsFavorite[];
  recentAccess: DocsRecentAccess[];
  onMovePage?: (pageId: string, parentId: string | null, sortOrder: number) => void;
  onToggleFavorite?: (pageId: string, favorite: boolean) => void;
  onTogglePinned?: (pageId: string, pinned: boolean) => void;
  onDeletePage?: (pageId: string) => void;
}) {
  const flattened = useMemo(() => flattenDocsTree(tree), [tree]);
  const favoritePages = flattened.filter((page) =>
    favorites.some((favorite) => favorite.pageId === page.id)
  );
  const recentPages = recentAccess
    .map((access) => flattened.find((page) => page.id === access.pageId))
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
          Search docs
        </div>
        <Input
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Find a page, template, or runbook"
        />
      </div>

      <section className="space-y-2">
        <h2 className="text-sm font-semibold">Page Tree</h2>
        <PageTree
          nodes={filteredTree as DocsPageTreeNode[]}
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
          Favorites
        </div>
        {favoritePages.map((page) => (
          <Link key={page.id} href={`/docs/${page.id}`} className="flex items-center gap-2 text-sm hover:text-primary">
            <Star className="size-4 text-muted-foreground" />
            <span className="truncate">{page.title}</span>
          </Link>
        ))}
        {favoritePages.length === 0 ? (
          <p className="text-xs text-muted-foreground">No favorites yet.</p>
        ) : null}
      </section>

      <section className="space-y-2">
        <div className="flex items-center gap-2 text-sm font-semibold">
          <Clock3 className="size-4 text-muted-foreground" />
          Recent
        </div>
        {recentPages.map((page) => (
          <Link key={page.id} href={`/docs/${page.id}`} className="truncate text-sm hover:text-primary">
            {page.title}
          </Link>
        ))}
        {recentPages.length === 0 ? (
          <p className="text-xs text-muted-foreground">No recent docs yet.</p>
        ) : null}
      </section>
    </aside>
  );
}
