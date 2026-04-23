"use client";

import { useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  useMarketplaceStore,
  type MarketplaceConsumptionRecord,
  type MarketplaceItem,
  typeDisplayLabel,
} from "@/lib/stores/marketplace-store";
import { toast } from "sonner";
import { useTranslations } from "next-intl";

interface Props {
  item: MarketplaceItem | null;
  consumption?: MarketplaceConsumptionRecord | null;
  onClose: () => void;
}

export function MarketplaceInstallConfirm({ item, consumption, onClose }: Props) {
  const t = useTranslations("marketplace");
  const { installItem, checkUpdates } = useMarketplaceStore();
  const [loading, setLoading] = useState(false);

  if (!item) return null;

  const isUpdate =
    consumption?.status === "installed" &&
    consumption.installed &&
    consumption.provenance?.selectedVersion !== item.latest_version;

  const handleInstall = async () => {
    if (!item.latest_version) {
      toast.error(t("install.noVersion"));
      return;
    }
    setLoading(true);
    try {
      await installItem(item.id, item.latest_version);
      toast.success(
        isUpdate
          ? t("install.toastUpdated", { name: item.name, version: item.latest_version })
          : t("install.toastInstalled", { name: item.name }),
      );
      void checkUpdates();
      onClose();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("install.toastFailed"),
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <AlertDialog open={!!item} onOpenChange={(o) => !o && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {isUpdate
              ? t("install.updateTitle", { name: item.name })
              : t("install.installTitle", { name: item.name })}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {isUpdate ? (
              <>
                {t("install.updateDesc", {
                  name: item.name,
                  fromVersion: consumption?.provenance?.selectedVersion ?? "unknown",
                  toVersion: item.latest_version ?? "latest",
                })}
              </>
            ) : (
              <>
                {t("install.installDesc", {
                  name: item.name,
                  version: item.latest_version ?? "latest",
                  type: typeDisplayLabel(item.type),
                })}
              </>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={onClose}>{t("install.cancel")}</AlertDialogCancel>
          <AlertDialogAction onClick={handleInstall} disabled={loading}>
            {loading
              ? isUpdate
                ? t("install.updating")
                : t("install.installing")
              : isUpdate
                ? t("install.updateTo", { version: item.latest_version ?? "" })
                : t("install.confirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
