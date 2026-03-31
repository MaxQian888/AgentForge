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
  type MarketplaceItem,
} from "@/lib/stores/marketplace-store";
import { toast } from "sonner";

interface Props {
  item: MarketplaceItem | null;
  onClose: () => void;
}

export function MarketplaceInstallConfirm({ item, onClose }: Props) {
  const { installItem } = useMarketplaceStore();
  const [loading, setLoading] = useState(false);

  if (!item) return null;

  const handleInstall = async () => {
    if (!item.latest_version) {
      toast.error("No version available");
      return;
    }
    setLoading(true);
    try {
      await installItem(item.id, item.latest_version);
      toast.success(`${item.name} installed successfully`);
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
          <AlertDialogTitle>Install {item.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            This will download and install version{" "}
            {item.latest_version ?? "latest"} of {item.name} ({item.type}) into
            AgentForge.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={onClose}>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={handleInstall} disabled={loading}>
            {loading ? "Installing..." : "Install"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
