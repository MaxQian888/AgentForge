"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ExternalLink, Star } from "lucide-react";
import type { MarketplaceItem } from "@/lib/stores/marketplace-store";
import { useMarketplaceStore } from "@/lib/stores/marketplace-store";
import { MarketplaceVersionList } from "./marketplace-version-list";
import { MarketplaceReviewDialog } from "./marketplace-review-dialog";

interface Props {
  item: MarketplaceItem;
  onInstall?: (item: MarketplaceItem) => void;
  installedIds: Set<string>;
}

export function MarketplaceItemDetail({
  item,
  onInstall,
  installedIds,
}: Props) {
  const reviews = useMarketplaceStore((s) => s.selectedItemReviews);
  const installed = installedIds.has(item.id);

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="p-4 border-b">
        <div className="flex items-start justify-between mb-2">
          <div>
            <h2 className="font-semibold text-sm">{item.name}</h2>
            <p className="text-xs text-muted-foreground">
              by {item.author_name}
            </p>
          </div>
          <Button
            size="sm"
            variant={installed ? "secondary" : "default"}
            disabled={installed}
            onClick={() => onInstall?.(item)}
          >
            {installed ? "Installed" : "Install"}
          </Button>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Badge variant="outline" className="text-xs">
            {item.type}
          </Badge>
          {item.category && (
            <Badge variant="outline" className="text-xs">
              {item.category}
            </Badge>
          )}
          {item.is_verified && (
            <Badge className="text-xs bg-blue-500">Verified</Badge>
          )}
          <span className="text-xs flex items-center gap-1 text-muted-foreground">
            <Star className="w-3 h-3" /> {item.avg_rating.toFixed(1)} (
            {item.rating_count})
          </span>
        </div>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="overview" className="flex-1 flex flex-col">
        <TabsList className="mx-4 mt-2 w-auto">
          <TabsTrigger value="overview" className="text-xs">
            Overview
          </TabsTrigger>
          <TabsTrigger value="versions" className="text-xs">
            Versions
          </TabsTrigger>
          <TabsTrigger value="reviews" className="text-xs">
            Reviews
          </TabsTrigger>
        </TabsList>
        <ScrollArea className="flex-1">
          <TabsContent value="overview" className="p-4 space-y-3">
            <p className="text-sm text-muted-foreground">
              {item.description || "No description."}
            </p>
            {item.tags.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {item.tags.map((t) => (
                  <span
                    key={t}
                    className="text-xs bg-muted px-2 py-0.5 rounded-full"
                  >
                    {t}
                  </span>
                ))}
              </div>
            )}
            <div className="text-xs text-muted-foreground space-y-1">
              <div>
                License:{" "}
                <span className="text-foreground">{item.license}</span>
              </div>
              {item.latest_version && (
                <div>
                  Latest:{" "}
                  <span className="text-foreground">{item.latest_version}</span>
                </div>
              )}
            </div>
            {item.repository_url && (
              <a
                href={item.repository_url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 text-xs text-blue-500 hover:underline"
              >
                <ExternalLink className="w-3 h-3" /> Repository
              </a>
            )}
          </TabsContent>
          <TabsContent value="versions" className="p-0">
            <MarketplaceVersionList itemId={item.id} />
          </TabsContent>
          <TabsContent value="reviews" className="p-4 space-y-3">
            <MarketplaceReviewDialog itemId={item.id} />
            {reviews.length === 0 ? (
              <p className="text-xs text-muted-foreground">No reviews yet.</p>
            ) : (
              reviews.map((r) => (
                <div key={r.id} className="border rounded p-3 space-y-1">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-medium">{r.user_name}</span>
                    <div className="flex">
                      {Array.from({ length: 5 }).map((_, i) => (
                        <Star
                          key={i}
                          className={`w-3 h-3 ${
                            i < r.rating
                              ? "text-yellow-400 fill-yellow-400"
                              : "text-muted"
                          }`}
                        />
                      ))}
                    </div>
                  </div>
                  {r.comment && (
                    <p className="text-xs text-muted-foreground">{r.comment}</p>
                  )}
                </div>
              ))
            )}
          </TabsContent>
        </ScrollArea>
      </Tabs>
    </div>
  );
}
