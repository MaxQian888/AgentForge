"use client";

import { useEffect, useState, useCallback } from "react";
import { Store, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useMarketplaceStore } from "@/lib/stores/marketplace-store";
import { MarketplaceFilterPanel } from "@/components/marketplace/marketplace-filter-panel";
import { MarketplaceSearchBar } from "@/components/marketplace/marketplace-search-bar";
import { MarketplaceItemCard } from "@/components/marketplace/marketplace-item-card";
import { MarketplaceItemDetail } from "@/components/marketplace/marketplace-item-detail";
import { MarketplacePublishDialog } from "@/components/marketplace/marketplace-publish-dialog";
import { MarketplaceInstallConfirm } from "@/components/marketplace/marketplace-install-confirm";

export default function MarketplacePage() {
  const {
    items,
    featuredItems,
    selectedItem,
    filters,
    total,
    loading,
    installedItemIds,
    publishDialogOpen,
    installConfirmItem,
    fetchItems,
    fetchFeatured,
    fetchInstalled,
    selectItem,
    search,
    setFilters,
    setPublishDialogOpen,
    setInstallConfirmItem,
  } = useMarketplaceStore();

  const [searchQuery, setSearchQuery] = useState("");

  useEffect(() => {
    void fetchItems();
    void fetchFeatured();
    void fetchInstalled();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSearch = useCallback(
    (q: string) => {
      if (q.trim()) {
        void search(q);
      } else {
        void fetchItems();
      }
    },
    [search, fetchItems],
  );

  const totalPages = Math.ceil(total / 20);

  return (
    <div className="flex h-full">
      {/* Left: Filters */}
      <div className="w-52 shrink-0 border-r overflow-y-auto">
        <MarketplaceFilterPanel filters={filters} onChange={setFilters} />
      </div>

      {/* Center: Main content */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <Store className="w-5 h-5 shrink-0 text-muted-foreground" />
            <h1 className="font-semibold text-sm">Marketplace</h1>
          </div>
          <div className="flex items-center gap-2 flex-1 max-w-sm">
            <MarketplaceSearchBar
              value={searchQuery}
              onChange={setSearchQuery}
              onSearch={handleSearch}
            />
          </div>
          <Button size="sm" onClick={() => setPublishDialogOpen(true)}>
            <Plus className="w-4 h-4 mr-1" /> Publish
          </Button>
        </div>

        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          {/* Featured strip */}
          {featuredItems.length > 0 && !searchQuery && (
            <div>
              <h2 className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2">
                Featured
              </h2>
              <div className="flex gap-3 overflow-x-auto pb-1">
                {featuredItems.slice(0, 5).map((item) => (
                  <div key={item.id} className="w-64 shrink-0">
                    <MarketplaceItemCard
                      item={item}
                      installed={installedItemIds.has(item.id)}
                      selected={selectedItem?.id === item.id}
                      onSelect={selectItem}
                      onInstall={setInstallConfirmItem}
                    />
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Main grid */}
          {loading ? (
            <div className="flex items-center justify-center h-32 text-sm text-muted-foreground">
              Loading...
            </div>
          ) : items.length === 0 ? (
            <div className="flex items-center justify-center h-32 text-sm text-muted-foreground">
              No items found. Try adjusting your filters.
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
              {items.map((item) => (
                <MarketplaceItemCard
                  key={item.id}
                  item={item}
                  installed={installedItemIds.has(item.id)}
                  selected={selectedItem?.id === item.id}
                  onSelect={selectItem}
                  onInstall={setInstallConfirmItem}
                />
              ))}
            </div>
          )}

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 pt-2">
              <Button
                variant="outline"
                size="sm"
                disabled={filters.page <= 1}
                onClick={() => setFilters({ page: filters.page - 1 })}
              >
                ← Prev
              </Button>
              <span className="text-xs text-muted-foreground">
                {filters.page} / {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={filters.page >= totalPages}
                onClick={() => setFilters({ page: filters.page + 1 })}
              >
                Next →
              </Button>
            </div>
          )}
        </div>
      </div>

      {/* Right: Detail panel */}
      {selectedItem && (
        <div className="w-80 shrink-0 border-l overflow-hidden">
          <MarketplaceItemDetail
            item={selectedItem}
            onInstall={setInstallConfirmItem}
            installedIds={installedItemIds}
          />
        </div>
      )}

      {/* Dialogs */}
      <MarketplacePublishDialog
        open={publishDialogOpen}
        onClose={() => setPublishDialogOpen(false)}
      />
      <MarketplaceInstallConfirm
        item={installConfirmItem}
        onClose={() => setInstallConfirmItem(null)}
      />
    </div>
  );
}
