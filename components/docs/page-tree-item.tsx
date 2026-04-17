"use client";

import { Star, Pin, Trash2, GripVertical, ChevronRight } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import type { KnowledgeAssetTreeNode } from "@/lib/stores/knowledge-store";
import { buildDocsHref } from "@/lib/route-hrefs";

export function PageTreeItem({
  node,
  currentPageId,
  onMove,
  onToggleFavorite,
  onTogglePinned,
  onDelete,
}: {
  node: KnowledgeAssetTreeNode;
  currentPageId?: string | null;
  onMove?: (pageId: string, parentId: string | null, sortOrder: number) => void;
  onToggleFavorite?: (pageId: string, favorite: boolean) => void;
  onTogglePinned?: (pageId: string, pinned: boolean) => void;
  onDelete?: (pageId: string) => void;
}) {
  const [open, setOpen] = useState(true);

  return (
    <div
      className="flex flex-col gap-1"
      draggable
      onDragStart={(event) => event.dataTransfer.setData("text/page-id", node.id)}
      onDrop={(event) => {
        const draggedId = event.dataTransfer.getData("text/page-id");
        if (draggedId && draggedId !== node.id) {
          onMove?.(draggedId, node.parentId ?? null, node.sortOrder ?? 0);
        }
        event.preventDefault();
      }}
      onDragOver={(event) => event.preventDefault()}
    >
      <div
        className={`group flex items-center gap-2 rounded-lg px-2 py-1.5 ${
          currentPageId === node.id ? "bg-accent text-accent-foreground" : "hover:bg-accent/40"
        }`}
      >
        <Button size="icon-sm" variant="ghost" onClick={() => setOpen((value) => !value)}>
          <ChevronRight className={`size-4 transition ${open ? "rotate-90" : ""}`} />
        </Button>
        <GripVertical className="size-4 text-muted-foreground" />
        <Link href={buildDocsHref(node.id)} className="flex-1 truncate text-sm font-medium">
          {node.title}
        </Link>
        <div className="flex opacity-0 transition group-hover:opacity-100">
          <Button size="icon-sm" variant="ghost" onClick={() => onToggleFavorite?.(node.id, true)}>
            <Star className="size-4" />
          </Button>
          <Button size="icon-sm" variant="ghost" onClick={() => onTogglePinned?.(node.id, !node.isPinned)}>
            <Pin className={`size-4 ${node.isPinned ? "fill-current" : ""}`} />
          </Button>
          <Button size="icon-sm" variant="ghost" onClick={() => onDelete?.(node.id)}>
            <Trash2 className="size-4" />
          </Button>
        </div>
      </div>
      {open && node.children.length > 0 ? (
        <div className="ml-5 flex flex-col gap-1 border-l border-border/60 pl-3">
          {node.children.map((child) => (
            <PageTreeItem
              key={child.id}
              node={child}
              currentPageId={currentPageId}
              onMove={onMove}
              onToggleFavorite={onToggleFavorite}
              onTogglePinned={onTogglePinned}
              onDelete={onDelete}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
