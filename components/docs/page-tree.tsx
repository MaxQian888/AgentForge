"use client";

import { useTranslations } from "next-intl";
import type { KnowledgeAssetTreeNode } from "@/lib/stores/knowledge-store";
import { PageTreeItem } from "./page-tree-item";

export function PageTree({
  nodes,
  currentPageId,
  onMovePage,
  onToggleFavorite,
  onTogglePinned,
  onDeletePage,
}: {
  nodes: KnowledgeAssetTreeNode[];
  currentPageId?: string | null;
  onMovePage?: (pageId: string, parentId: string | null, sortOrder: number) => void;
  onToggleFavorite?: (pageId: string, favorite: boolean) => void;
  onTogglePinned?: (pageId: string, pinned: boolean) => void;
  onDeletePage?: (pageId: string) => void;
}) {
  const t = useTranslations("docs");

  return (
    <div className="flex flex-col gap-1">
      {nodes.map((node) => (
        <PageTreeItem
          key={node.id}
          node={node}
          currentPageId={currentPageId}
          onMove={onMovePage}
          onToggleFavorite={onToggleFavorite}
          onTogglePinned={onTogglePinned}
          onDelete={onDeletePage}
        />
      ))}
      {nodes.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border/60 p-3 text-sm text-muted-foreground">
          {t("pageTree.empty")}
        </div>
      ) : null}
    </div>
  );
}
