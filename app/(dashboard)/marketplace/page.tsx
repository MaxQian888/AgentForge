"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, PackageSearch, Plus, Store } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { useMarketplaceStore, resolveMarketplaceConsumptionRecord, typeDisplayLabel } from "@/lib/stores/marketplace-store";
import { MarketplaceFilterPanel } from "@/components/marketplace/marketplace-filter-panel";
import { MarketplaceSearchBar } from "@/components/marketplace/marketplace-search-bar";
import { MarketplaceItemCard } from "@/components/marketplace/marketplace-item-card";
import { MarketplaceItemDetail } from "@/components/marketplace/marketplace-item-detail";
import { MarketplacePublishDialog } from "@/components/marketplace/marketplace-publish-dialog";
import { MarketplaceInstallConfirm } from "@/components/marketplace/marketplace-install-confirm";
import { EmptyState } from "@/components/shared/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuthStore } from "@/lib/stores/auth-store";
import { usePlatformCapability } from "@/hooks/use-platform-capability";
import { toast } from "sonner";

export default function MarketplacePage() {
  const t = useTranslations("marketplace");
  const {
    items,
    builtInItems,
    featuredItems,
    selectedItem,
    filters,
    total,
    loading,
    builtInLoading,
    consumptionItems,
    updates,
    serviceStatus,
    serviceMessage,
    builtInMessage,
    publishDialogOpen,
    installConfirmItem,
    fetchItems,
    fetchBuiltInItems,
    fetchFeatured,
    fetchConsumption,
    checkUpdates,
    uninstallItem,
    selectItem,
    search,
    setFilters,
    setPublishDialogOpen,
    setInstallConfirmItem,
    installLocalPlugin,
  } = useMarketplaceStore();
  const currentUserId = useAuthStore((state) => state.user?.id ?? null);
  const { isDesktop, selectFiles } = usePlatformCapability();
  const hydratedDeepLinkItemIdRef = useRef<string | null>(null);

  const [searchQuery, setSearchQuery] = useState(filters.query);
  const locationSearch = typeof window === "undefined" ? "" : window.location.search;

  useEffect(() => {
    void Promise.all([fetchItems(), fetchBuiltInItems(), fetchFeatured(), fetchConsumption(), checkUpdates()]);
  }, [fetchBuiltInItems, fetchConsumption, fetchFeatured, fetchItems, checkUpdates]);

  useEffect(() => {
    if (!items.length && !builtInItems.length) {
      return;
    }
    const params = new URLSearchParams(locationSearch);
    const requestedItemId = params.get("item");
    if (!requestedItemId) {
      hydratedDeepLinkItemIdRef.current = null;
      return;
    }
    if (hydratedDeepLinkItemIdRef.current === requestedItemId) {
      return;
    }
    const match = [...builtInItems, ...items].find((item) => item.id === requestedItemId);
    if (match) {
      selectItem(match);
      hydratedDeepLinkItemIdRef.current = requestedItemId;
    }
  }, [builtInItems, items, locationSearch, selectItem]);

  const handleSearch = useCallback(
    (query: string) => {
      if (query.trim()) {
        void search(query);
      } else {
        void fetchItems();
      }
    },
    [fetchItems, search],
  );

  const handleSideLoad = useCallback(
    async () => {
      const result = await selectFiles({
        directory: true,
        multiple: false,
        title: t("selectFilesTitle"),
      });

      if (result.ok && result.mode === "desktop" && result.paths[0]) {
        try {
          await installLocalPlugin(result.paths[0]);
          toast.success(t("sideLoadSuccess"));
        } catch (error) {
          toast.error(error instanceof Error ? error.message : t("sideLoadFailed"));
        }
        return;
      }

      if (result.ok && result.mode !== "desktop") {
        toast.info(t("sideLoadUnavailable"));
        return;
      }

      if (!isDesktop) {
        toast.info(t("sideLoadDesktopOnly"));
      }
    },
    [installLocalPlugin, isDesktop, selectFiles, t],
  );

  const handleTagClick = useCallback(
    (tag: string) => {
      const existing = filters.tags;
      if (!existing.includes(tag)) {
        setFilters({ tags: [...existing, tag], page: 1 });
      }
    },
    [filters.tags, setFilters],
  );

  const handleUninstall = useCallback(
    async (item: { id: string; type: string }) => {
      try {
        await uninstallItem(item.id, item.type);
        toast.success(t("uninstallSuccess", { type: typeDisplayLabel(item.type) }));
        selectItem(null);
      } catch (error) {
        toast.error(error instanceof Error ? error.message : t("uninstallFailed"));
      }
    },
    [uninstallItem, selectItem, t],
  );

  const updatesByItemId = useMemo(() => {
    const map = new Map<string, (typeof updates)[number]>();
    for (const u of updates) {
      map.set(u.itemId, u);
    }
    return map;
  }, [updates]);

  const totalPages = Math.max(1, Math.ceil(total / 20));
  const selectedConsumption = selectedItem
    ? resolveMarketplaceConsumptionRecord(consumptionItems, selectedItem.id)
    : null;
  const selectedUpdateInfo = selectedItem
    ? updatesByItemId.get(selectedItem.id) ?? null
    : null;

  const filteredBuiltInItems = useMemo(
    () =>
      builtInItems.filter((item) => {
        if (filters.type !== "all" && item.type !== filters.type) {
          return false;
        }
        if (filters.category && item.category !== filters.category) {
          return false;
        }
        if (!searchQuery.trim()) {
          return true;
        }
        const needle = searchQuery.trim().toLowerCase();
        return (
          item.name.toLowerCase().includes(needle) ||
          item.description.toLowerCase().includes(needle) ||
          item.tags.some((tag) => tag.toLowerCase().includes(needle))
        );
      }),
    [builtInItems, filters.category, filters.type, searchQuery],
  );

  const featuredWithoutSelectionNoise = useMemo(
    () => featuredItems.filter((item) => item.id !== selectedItem?.id),
    [featuredItems, selectedItem?.id],
  );

  return (
    <div className="flex h-full">
      <div className="w-52 shrink-0 overflow-y-auto border-r">
        <MarketplaceFilterPanel filters={filters} onChange={setFilters} />
      </div>

      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <div className="flex items-center justify-between gap-3 border-b p-4">
          <div className="flex min-w-0 items-center gap-2">
            <Store className="size-5 shrink-0 text-muted-foreground" />
            <div className="min-w-0">
              <h1 className="text-sm font-semibold">{t("pageTitle")}</h1>
              <p className="text-xs text-muted-foreground">
                {t("pageDescription")}
              </p>
            </div>
          </div>
          <div className="flex flex-1 items-center gap-2">
            <MarketplaceSearchBar
              value={searchQuery}
              onChange={setSearchQuery}
              onSearch={handleSearch}
            />
          </div>
          <Button size="sm" onClick={() => setPublishDialogOpen(true)}>
            <Plus className="mr-1 size-4" />
            {t("publish.submitLabel")}
          </Button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          {serviceStatus === "unavailable" && filteredBuiltInItems.length === 0 && !builtInLoading ? (
            <div className="flex h-40 flex-col items-center justify-center gap-2 rounded-xl border border-dashed text-center">
              <AlertTriangle className="size-5 text-amber-500" />
              <p className="text-sm font-medium">{t("serviceUnavailable")}</p>
              <p className="max-w-md text-xs text-muted-foreground">
                {serviceMessage ?? t("serviceUnavailableDesc")}
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {serviceStatus === "unavailable" ? (
                <div className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-left">
                  <div className="flex items-center gap-2 text-sm font-medium text-amber-900">
                    <AlertTriangle className="size-4" />
                    {t("remoteUnavailable")}
                  </div>
                  <p className="mt-1 text-xs text-amber-800">
                    {serviceMessage ?? t("remoteUnavailableDesc")}
                  </p>
                </div>
              ) : null}

              {builtInMessage ? (
                <div className="rounded-xl border border-dashed p-3 text-xs text-muted-foreground">
                  {builtInMessage}
                </div>
              ) : null}

              {builtInLoading ? (
                <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <Skeleton key={i} className="h-40 rounded-lg" />
                  ))}
                </div>
              ) : filteredBuiltInItems.length > 0 ? (
                <div>
                  <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {filters.type !== "all" ? (filters.type === "workflow_template" ? t("builtInTypeWorkflows") : t("builtInItems")) : t("builtInItems")}
                  </h2>
                  <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
                    {filteredBuiltInItems.map((item) => (
                      <MarketplaceItemCard
                        key={item.id}
                        item={item}
                        consumption={resolveMarketplaceConsumptionRecord(consumptionItems, item.id)}
                        updateInfo={updatesByItemId.get(item.id)}
                        selected={selectedItem?.id === item.id}
                        onSelect={selectItem}
                        onInstall={setInstallConfirmItem}
                        onTagClick={handleTagClick}
                      />
                    ))}
                  </div>
                </div>
              ) : null}

              {featuredWithoutSelectionNoise.length > 0 && !searchQuery ? (
                <div>
                  <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    {t("featured")}
                  </h2>
                  <div className="flex gap-3 overflow-x-auto pb-1">
                    {featuredWithoutSelectionNoise.slice(0, 5).map((item) => (
                      <div key={item.id} className="w-64 shrink-0">
                        <MarketplaceItemCard
                          item={item}
                          consumption={resolveMarketplaceConsumptionRecord(consumptionItems, item.id)}
                          updateInfo={updatesByItemId.get(item.id)}
                          selected={selectedItem?.id === item.id}
                          onSelect={selectItem}
                          onInstall={setInstallConfirmItem}
                          onTagClick={handleTagClick}
                        />
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}

              {serviceStatus === "unavailable" ? (
                <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                  {t("remoteUnavailableDesc")}
                </div>
              ) : loading ? (
                <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
                  {Array.from({ length: 6 }).map((_, i) => (
                    <Skeleton key={i} className="h-40 rounded-lg" />
                  ))}
                </div>
              ) : items.length === 0 ? (
                <EmptyState
                  icon={PackageSearch}
                  title={t("noItems")}
                  description={t("noItemsDesc")}
                  action={{ label: t("publish.submitLabel"), onClick: () => setPublishDialogOpen(true) }}
                />
              ) : (
                <div className="grid grid-cols-1 gap-[var(--space-grid-gap)] sm:grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-3">
                  {items.map((item) => (
                    <MarketplaceItemCard
                      key={item.id}
                      item={item}
                      consumption={resolveMarketplaceConsumptionRecord(consumptionItems, item.id)}
                      updateInfo={updatesByItemId.get(item.id)}
                      selected={selectedItem?.id === item.id}
                      onSelect={selectItem}
                      onInstall={setInstallConfirmItem}
                      onTagClick={handleTagClick}
                    />
                  ))}
                </div>
              )}

              {totalPages > 1 ? (
                <div className="flex items-center justify-center gap-2 pt-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={filters.page <= 1}
                    onClick={() => setFilters({ page: filters.page - 1 })}
                  >
                    ← {t("prev")}
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
                    {t("next")} →
                  </Button>
                </div>
              ) : null}
            </div>
          )}
        </div>
      </div>

      {selectedItem ? (
        <div className="w-96 shrink-0 overflow-hidden border-l">
          <MarketplaceItemDetail
            item={selectedItem}
            consumption={selectedConsumption}
            currentUserId={currentUserId}
            updateInfo={selectedUpdateInfo}
            onInstall={setInstallConfirmItem}
            onSideLoad={selectedItem.type === "plugin" ? handleSideLoad : undefined}
            onUninstall={(item) => void handleUninstall(item)}
            onTagClick={handleTagClick}
          />
        </div>
      ) : null}

      <MarketplacePublishDialog
        open={publishDialogOpen}
        onClose={() => setPublishDialogOpen(false)}
      />
      <MarketplaceInstallConfirm
        item={installConfirmItem}
        consumption={
          installConfirmItem
            ? resolveMarketplaceConsumptionRecord(consumptionItems, installConfirmItem.id)
            : null
        }
        onClose={() => setInstallConfirmItem(null)}
      />
    </div>
  );
}
