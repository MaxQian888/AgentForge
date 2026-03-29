"use client";

import { useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { usePlatformCapability } from "@/hooks/use-platform-capability";
import {
  usePluginStore,
  type MarketplacePluginEntry,
} from "@/lib/stores/plugin-store";
import { PluginCatalogSearch } from "./plugin-catalog-search";
import { PluginInstallConfirmation } from "./plugin-install-confirmation";
import { Loader2 } from "lucide-react";

interface PluginInstallDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type InstallStep = "source" | "confirm" | "installing";
type SourceTab = "local" | "catalog";

type InstallSource =
  | { type: "local"; path: string }
  | { type: "catalog"; entry: string; catalogEntry: MarketplacePluginEntry };

export function PluginInstallDialog({
  open,
  onOpenChange,
}: PluginInstallDialogProps) {
  const t = useTranslations("plugins");
  const installLocal = usePluginStore((s) => s.installLocal);
  const installFromCatalog = usePluginStore((s) => s.installFromCatalog);
  const fetchPlugins = usePluginStore((s) => s.fetchPlugins);
  const { isDesktop, selectFiles } = usePlatformCapability();

  const [step, setStep] = useState<InstallStep>("source");
  const [activeTab, setActiveTab] = useState<SourceTab>("local");
  const [installError, setInstallError] = useState<string | null>(null);

  // Form fields
  const [localPath, setLocalPath] = useState("");
  const [catalogEntry, setCatalogEntry] =
    useState<MarketplacePluginEntry | null>(null);

  const resetForm = useCallback(() => {
    setStep("source");
    setActiveTab("local");
    setInstallError(null);
    setLocalPath("");
    setCatalogEntry(null);
  }, []);

  const handleOpenChange = (next: boolean) => {
    if (!next) resetForm();
    onOpenChange(next);
  };

  const handleBrowse = async () => {
    const result = await selectFiles({
      directory: true,
      multiple: false,
      title: "Select a local plugin directory",
    });

    if (result.ok && result.mode === "desktop" && result.paths[0]) {
      setLocalPath(result.paths[0]);
    }
  };

  const isNextValid = (): boolean => {
    switch (activeTab) {
      case "local":
        return localPath.trim().length > 0;
      case "catalog":
        return catalogEntry !== null;
    }
  };

  const buildSource = (): InstallSource => {
    switch (activeTab) {
      case "local":
        return { type: "local", path: localPath.trim() };
      case "catalog":
        return {
          type: "catalog",
          entry: catalogEntry!.id,
          catalogEntry: catalogEntry!,
        };
    }
  };

  const getSourceLabel = (source: InstallSource): string => {
    switch (source.type) {
      case "local":
        return `Local path: ${source.path}`;
      case "catalog":
        return `Catalog: ${source.catalogEntry.name} v${source.catalogEntry.version}`;
    }
  };

  const [source, setSource] = useState<InstallSource | null>(null);

  const handleNext = () => {
    const s = buildSource();
    setSource(s);
    setStep("confirm");
  };

  const handleInstall = async () => {
    if (!source) return;
    setStep("installing");
    setInstallError(null);

    try {
      switch (source.type) {
        case "local":
          await installLocal(source.path);
          break;
        case "catalog":
          await installFromCatalog(source.entry);
          break;
      }
      await fetchPlugins();
      handleOpenChange(false);
    } catch (err) {
      setInstallError(
        err instanceof Error ? err.message : t("installDialog.installationFailed")
      );
    }
  };

  const dialogTitle = (): string => {
    switch (step) {
      case "source":
        return t("installDialog.titleInstall");
      case "confirm":
        return t("installDialog.titleConfirm");
      case "installing":
        return t("installDialog.titleInstalling");
    }
  };

  const dialogDescription = (): string => {
    switch (step) {
      case "source":
        return t("installDialog.descSource");
      case "confirm":
        return t("installDialog.descConfirm");
      case "installing":
        return t("installDialog.descInstalling");
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{dialogTitle()}</DialogTitle>
          <DialogDescription>{dialogDescription()}</DialogDescription>
        </DialogHeader>

        {/* Step 1: Source Selection */}
        {step === "source" && (
          <div className="grid gap-4 py-2">
            <Tabs
              value={activeTab}
              onValueChange={(v) => setActiveTab(v as SourceTab)}
            >
              <TabsList className="w-full">
                <TabsTrigger value="local">{t("installDialog.tabLocal")}</TabsTrigger>
                <TabsTrigger value="catalog">{t("installDialog.tabCatalog")}</TabsTrigger>
              </TabsList>

              <TabsContent value="local" className="mt-4">
                <div className="grid gap-2">
                  <Label htmlFor="plugin-path">{t("installDialog.pluginPath")}</Label>
                  <div className="flex gap-2">
                    <Input
                      id="plugin-path"
                      placeholder="/path/to/plugin"
                      value={localPath}
                      onChange={(e) => setLocalPath(e.target.value)}
                    />
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => void handleBrowse()}
                    >
                      {t("installDialog.browse")}
                    </Button>
                  </div>
                  {!isDesktop && (
                    <p className="text-xs text-muted-foreground">
                      {t("installDialog.nativePathHint")}
                    </p>
                  )}
                </div>
              </TabsContent>

              <TabsContent value="catalog" className="mt-4">
                <PluginCatalogSearch onSelect={setCatalogEntry} />
              </TabsContent>
            </Tabs>

            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => handleOpenChange(false)}
              >
                {t("installDialog.cancel")}
              </Button>
              <Button
                type="button"
                disabled={!isNextValid()}
                onClick={handleNext}
              >
                {t("installDialog.next")}
              </Button>
            </div>
          </div>
        )}

        {/* Step 2: Confirm */}
        {step === "confirm" && source && (
          <div className="py-2">
            <PluginInstallConfirmation
              sourceType={source.type === "catalog" ? "catalog" : source.type}
              sourceLabel={getSourceLabel(source)}
              unsigned={
                source.type === "catalog"
                  ? source.catalogEntry.trustStatus === "unknown"
                  : true
              }
              onConfirm={() => void handleInstall()}
              onBack={() => setStep("source")}
            />
          </div>
        )}

        {/* Step 3: Installing */}
        {step === "installing" && (
          <div className="flex flex-col items-center gap-4 py-8">
            {installError ? (
              <>
                <p className="text-sm text-destructive">{installError}</p>
                <div className="flex gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setStep("confirm")}
                  >
                    {t("installDialog.back")}
                  </Button>
                  <Button
                    type="button"
                    onClick={() => void handleInstall()}
                  >
                    {t("installDialog.retry")}
                  </Button>
                </div>
              </>
            ) : (
              <>
                <Loader2 className="size-8 animate-spin text-muted-foreground" />
                <p className="text-sm text-muted-foreground">
                  {t("installDialog.installingPlugin")}
                </p>
              </>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
