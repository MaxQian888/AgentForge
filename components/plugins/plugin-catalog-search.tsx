"use client";

import { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useTranslations } from "next-intl";
import { usePluginStore, type MarketplacePluginEntry } from "@/lib/stores/plugin-store";
import { Search } from "lucide-react";

interface PluginCatalogSearchProps {
  onSelect: (entry: MarketplacePluginEntry) => void;
}

export function PluginCatalogSearch({ onSelect }: PluginCatalogSearchProps) {
  const t = useTranslations("plugins");
  const searchCatalog = usePluginStore((s) => s.searchCatalog);
  const catalogResults = usePluginStore((s) => s.catalogResults);
  const catalogQuery = usePluginStore((s) => s.catalogQuery);
  const setCatalogQuery = usePluginStore((s) => s.setCatalogQuery);

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [localQuery, setLocalQuery] = useState(catalogQuery);

  useEffect(() => {
    if (!localQuery.trim()) {
      setCatalogQuery("");
      return;
    }

    const timer = setTimeout(() => {
      void searchCatalog(localQuery.trim());
    }, 300);

    return () => clearTimeout(timer);
  }, [localQuery, searchCatalog, setCatalogQuery]);

  const handleSelect = (entry: MarketplacePluginEntry) => {
    setSelectedId(entry.id);
    onSelect(entry);
  };

  return (
    <div className="grid gap-3">
      <div className="relative">
        <Search className="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
        <Input
          placeholder={t("catalogSearch.placeholder")}
          value={localQuery}
          onChange={(e) => setLocalQuery(e.target.value)}
          className="pl-9"
        />
      </div>

      <div className="max-h-56 overflow-y-auto rounded-md border">
        {!catalogQuery.trim() ? (
          <p className="p-4 text-center text-sm text-muted-foreground">
            {t("catalogSearch.emptyPrompt")}
          </p>
        ) : catalogResults.length === 0 ? (
          <p className="p-4 text-center text-sm text-muted-foreground">
            {t("catalogSearch.noResults")}
          </p>
        ) : (
          <div className="grid gap-1 p-1">
            {catalogResults.map((entry) => (
              <button
                key={entry.id}
                type="button"
                onClick={() => handleSelect(entry)}
                className={cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-accent",
                  selectedId === entry.id &&
                    "border border-primary bg-accent"
                )}
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium truncate">
                      {entry.name}
                    </span>
                    <Badge variant="secondary" className="text-[10px] shrink-0">
                      {entry.kind}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span>v{entry.version}</span>
                    <span className="truncate">{entry.author}</span>
                  </div>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
