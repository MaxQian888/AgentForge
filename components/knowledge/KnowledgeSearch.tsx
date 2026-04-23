"use client";

import { useEffect, useRef, useState } from "react";
import { Search, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  useKnowledgeStore,
  type KnowledgeAssetKind,
  type KnowledgeSearchResult,
} from "@/lib/stores/knowledge-store";
import { buildDocsHref } from "@/lib/route-hrefs";
import Link from "next/link";
import { useTranslations } from "next-intl";

function groupByKind(
  items: KnowledgeSearchResult[],
): Map<KnowledgeAssetKind, KnowledgeSearchResult[]> {
  const map = new Map<KnowledgeAssetKind, KnowledgeSearchResult[]>();
  for (const item of items) {
    const group = map.get(item.kind) ?? [];
    group.push(item);
    map.set(item.kind, group);
  }
  return map;
}

function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return debounced;
}

export function KnowledgeSearch({
  projectId,
  placeholder,
  onNavigate,
}: {
  projectId: string;
  placeholder?: string;
  onNavigate?: (id: string, kind: KnowledgeAssetKind) => void;
}) {
  const t = useTranslations("knowledge");
  const [query, setQuery] = useState("");
  const debouncedQuery = useDebounce(query, 300);
  const { searchResults, loading, searchKnowledge, clearSearch } = useKnowledgeStore();
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!debouncedQuery.trim()) {
      clearSearch();
      return;
    }
    void searchKnowledge(projectId, debouncedQuery.trim());
  }, [debouncedQuery, projectId, searchKnowledge, clearSearch]);

  const handleClear = () => {
    setQuery("");
    clearSearch();
    inputRef.current?.focus();
  };

  const grouped = searchResults ? groupByKind(searchResults.items) : null;
  const kindOrder: KnowledgeAssetKind[] = ["wiki_page", "ingested_file", "template"];

  const kindLabels: Record<KnowledgeAssetKind, string> = {
    wiki_page: t("kind.wiki_page"),
    ingested_file: t("kind.ingested_file"),
    template: t("kind.template"),
  };

  const resolvedPlaceholder = placeholder ?? t("search.placeholder");

  return (
    <div className="relative flex flex-col gap-0">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          ref={inputRef}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={resolvedPlaceholder}
          className="pl-9 pr-8"
        />
        {query && (
          <button
            type="button"
            onClick={handleClear}
            className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          >
            <X className="size-4" />
          </button>
        )}
      </div>

      {grouped && query.trim() && (
        <div className="absolute left-0 right-0 top-full z-50 mt-1 max-h-96 overflow-y-auto rounded-xl border border-border/70 bg-popover p-2 shadow-lg">
          {loading && (
            <p className="px-2 py-1 text-xs text-muted-foreground">{t("search.searching")}</p>
          )}
          {!loading && grouped.size === 0 && (
            <p className="px-2 py-1 text-sm text-muted-foreground">{t("search.noResults")}</p>
          )}
          {kindOrder.map((kind) => {
            const items = grouped.get(kind);
            if (!items || items.length === 0) return null;
            return (
              <div key={kind} className="mb-2">
                <p className="px-2 pb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {kindLabels[kind]}
                </p>
                <ul>
                  {items.map((item) => (
                    <li key={item.id}>
                      {kind === "wiki_page" || kind === "template" ? (
                        <Link
                          href={buildDocsHref(item.id)}
                          onClick={() => {
                            handleClear();
                            onNavigate?.(item.id, kind);
                          }}
                          className="flex items-start gap-2 rounded-lg px-2 py-2 hover:bg-accent/50"
                        >
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-sm font-medium">{item.title}</div>
                            {item.snippet && (
                              <div className="truncate text-xs text-muted-foreground">
                                {item.snippet}
                              </div>
                            )}
                          </div>
                          <Badge variant="outline" className="shrink-0 text-xs">
                            {kindLabels[kind]}
                          </Badge>
                        </Link>
                      ) : (
                        <button
                          type="button"
                          onClick={() => {
                            handleClear();
                            onNavigate?.(item.id, kind);
                          }}
                          className="flex w-full items-start gap-2 rounded-lg px-2 py-2 text-left hover:bg-accent/50"
                        >
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-sm font-medium">{item.title}</div>
                            {item.snippet && (
                              <div className="truncate text-xs text-muted-foreground">
                                {item.snippet}
                              </div>
                            )}
                          </div>
                          <Badge variant="outline" className="shrink-0 text-xs">
                            {kindLabels[kind]}
                          </Badge>
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
