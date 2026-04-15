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

interface Props {
  item: MarketplaceItem | null;
  consumption?: MarketplaceConsumptionRecord | null;
  onClose: () => void;
}

export function MarketplaceInstallConfirm({ item, consumption, onClose }: Props) {
  const { installItem, checkUpdates } = useMarketplaceStore();
  const [loading, setLoading] = useState(false);

  if (!item) return null;

  const isUpdate =
    consumption?.status === "installed" &&
    consumption.installed &&
    consumption.provenance?.selectedVersion !== item.latest_version;

  const handleInstall = async () => {
    if (!item.latest_version) {
      toast.error("No version available");
      return;
    }
    setLoading(true);
    try {
      await installItem(item.id, item.latest_version);
      toast.success(
        isUpdate
          ? `${item.name} updated to v${item.latest_version}`
          : `${item.name} installed successfully`,
      );
      void checkUpdates();
      onClose();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Installation failed",
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
            {isUpdate ? `Update ${item.name}?` : `Install ${item.name}?`}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {isUpdate ? (
              <>
                This will update {item.name} from v
                {consumption?.provenance?.selectedVersion ?? "unknown"} to v
                {item.latest_version ?? "latest"}.
              </>
            ) : (
              <>
                This will download and install version{" "}
                {item.latest_version ?? "latest"} of {item.name} ({typeDisplayLabel(item.type)}) into
                AgentForge.
              </>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={onClose}>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={handleInstall} disabled={loading}>
            {loading
              ? isUpdate
                ? "Updating..."
                : "Installing..."
              : isUpdate
                ? `Update to v${item.latest_version}`
                : "Install"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
