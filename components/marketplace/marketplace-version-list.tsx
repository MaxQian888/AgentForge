"use client";

import { useEffect } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Download } from "lucide-react";
import { useMarketplaceStore } from "@/lib/stores/marketplace-store";
import { useTranslations } from "next-intl";
import { formatDistanceToNow } from "date-fns";

const MARKETPLACE_URL =
  process.env.NEXT_PUBLIC_MARKETPLACE_URL ?? "http://localhost:7781";

interface Props {
  itemId: string;
  canManage?: boolean;
}

export function MarketplaceVersionList({ itemId, canManage = false }: Props) {
  const t = useTranslations("marketplace");
  const { fetchItemVersions, selectedItemVersions, yankVersion } = useMarketplaceStore();

  useEffect(() => {
    void fetchItemVersions(itemId);
  }, [itemId, fetchItemVersions]);

  if (selectedItemVersions.length === 0) {
    return (
      <p className="text-xs text-muted-foreground p-4">
        {t("versions.noVersions")}
      </p>
    );
  }

  return (
    <div className="divide-y">
      {selectedItemVersions.map((v) => (
        <div key={v.id} className="flex items-center justify-between px-4 py-3">
          <div className="space-y-0.5">
            <div className="flex items-center gap-2">
              <Badge variant="outline" className="text-xs font-mono">
                {v.version}
              </Badge>
              {v.is_latest && <Badge className="text-xs">{t("versions.latest")}</Badge>}
              {v.is_yanked && (
                <Badge variant="destructive" className="text-xs">
                  {t("versions.yanked")}
                </Badge>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {formatDistanceToNow(new Date(v.created_at), {
                addSuffix: true,
              })}{" "}
              · {(v.artifact_size_bytes / 1024).toFixed(1)} KB
            </p>
          </div>
          {!v.is_yanked && (
            <div className="flex items-center gap-2">
              <a
                href={`${MARKETPLACE_URL}/api/v1/items/${itemId}/versions/${v.version}/download`}
                className="text-xs text-blue-500 hover:underline flex items-center gap-1"
                download
              >
                <Download className="w-3 h-3" />
                {t("versions.download")}
              </a>
              {canManage ? (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => void yankVersion(itemId, v.version)}
                >
                  {t("versions.yank")}
                </Button>
              ) : null}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
