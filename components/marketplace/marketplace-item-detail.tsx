"use client";

import { useMemo, useState } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { ArrowUpCircle, ExternalLink, ShieldCheck, Sparkles, Star, Trash2, Upload } from "lucide-react";
import {
  useMarketplaceStore,
  typeDisplayLabel,
  type MarketplaceConsumptionRecord,
  type MarketplaceItem,
  type MarketplaceUpdateInfo,
} from "@/lib/stores/marketplace-store";
import { MarketplaceVersionList } from "./marketplace-version-list";
import { MarketplaceReviewDialog } from "./marketplace-review-dialog";
import { SkillPackagePreviewPane } from "./skill-package-preview";
import { toast } from "sonner";
import { useTranslations } from "next-intl";

interface Props {
  item: MarketplaceItem;
  consumption?: MarketplaceConsumptionRecord | null;
  currentUserId?: string | null;
  updateInfo?: MarketplaceUpdateInfo | null;
  onInstall?: (item: MarketplaceItem) => void;
  onSideLoad?: (item: MarketplaceItem) => void;
  onUninstall?: (item: MarketplaceItem) => void;
  onTagClick?: (tag: string) => void;
}

function downstreamHref(item: MarketplaceItem): string {
  switch (item.type) {
    case "plugin":
      return "/plugins";
    case "workflow_template":
      return "/workflow";
    default:
      return "/roles";
  }
}

function sideloadRootFile(type: string): string {
  switch (type) {
    case "role":
      return "role.yaml";
    case "workflow_template":
      return "workflow.json";
    default:
      return "SKILL.md";
  }
}

function getStatusState(
  consumption: MarketplaceConsumptionRecord | null,
  t: (key: string, values?: Record<string, string | number>) => string,
) {
  if (!consumption) {
    return {
      badge: null,
      detail: t("detail.status.notInstalled"),
    };
  }

  if (consumption.status === "installed" && consumption.provenance?.sourceType === "builtin") {
    return {
      badge: consumption.used ? t("detail.status.manage") : t("detail.status.available"),
      detail: consumption.used
        ? t("detail.status.detailManaged", { surface: consumption.consumerSurface })
        : t("detail.status.detailAvailable", { surface: consumption.consumerSurface }),
    };
  }

  switch (consumption.status) {
    case "installed":
      return {
        badge: consumption.used ? t("detail.status.manage") : t("detail.status.installed"),
        detail: consumption.used
          ? t("detail.status.detailManaged", { surface: consumption.consumerSurface })
          : t("detail.status.detailReady"),
      };
    case "warning":
      return {
        badge: t("detail.status.needsAttention"),
        detail: consumption.warning ?? consumption.failureReason ?? t("detail.status.detailWarning", { warning: "" }),
      };
    default:
      return {
        badge: t("detail.status.blocked"),
        detail: consumption.failureReason ?? t("detail.status.detailBlocked"),
      };
  }
}

export function MarketplaceItemDetail({
  item,
  consumption = null,
  currentUserId = null,
  updateInfo = null,
  onInstall,
  onSideLoad,
  onUninstall,
  onTagClick,
}: Props) {
  const t = useTranslations("marketplace");
  const reviews = useMarketplaceStore((s) => s.selectedItemReviews);
  const uploadVersion = useMarketplaceStore((s) => s.uploadVersion);
  const deleteItem = useMarketplaceStore((s) => s.deleteItem);
  const verifyItem = useMarketplaceStore((s) => s.verifyItem);
  const featureItem = useMarketplaceStore((s) => s.featureItem);
  const sideloadItem = useMarketplaceStore((s) => s.sideloadItem);
  const uninstallLoading = useMarketplaceStore((s) => s.uninstallLoading);
  const sideloadLoading = useMarketplaceStore((s) => s.sideloadLoading);

  const [version, setVersion] = useState("");
  const [changelog, setChangelog] = useState("");
  const [artifact, setArtifact] = useState<File | null>(null);
  const [submittingVersion, setSubmittingVersion] = useState(false);
  const [runningAction, setRunningAction] = useState<"verify" | "feature" | "delete" | null>(null);
  const [sideloadFile, setSideloadFile] = useState<File | null>(null);

  const state = useMemo(() => getStatusState(consumption, t), [consumption, t]);
  const isInstalled = consumption?.status === "installed" && consumption.installed;
  const canManageVersions = currentUserId === item.author_id;

  const downstreamLabelText = useMemo(() => {
    switch (item.type) {
      case "plugin":
        return t("detail.downstream.plugin");
      case "role":
        return t("detail.downstream.role");
      case "workflow_template":
        return t("detail.downstream.workflow");
      default:
        return t("detail.downstream.default");
    }
  }, [item.type, t]);

  const handleUploadVersion = async () => {
    if (!artifact || !version.trim()) {
      toast.error(t("detail.toast.failedUpload"));
      return;
    }
    setSubmittingVersion(true);
    try {
      await uploadVersion(item.id, {
        version: version.trim(),
        changelog,
        artifact,
      });
      toast.success(t("detail.toast.uploaded", { version: version.trim(), name: item.name }));
      setVersion("");
      setChangelog("");
      setArtifact(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("detail.toast.failedUpload"));
    } finally {
      setSubmittingVersion(false);
    }
  };

  const handleVerify = async () => {
    setRunningAction("verify");
    try {
      await verifyItem(item.id);
      toast.success(t("detail.toast.verify", { name: item.name }));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("detail.toast.failedVerify"));
    } finally {
      setRunningAction(null);
    }
  };

  const handleFeature = async () => {
    setRunningAction("feature");
    try {
      await featureItem(item.id);
      toast.success(t("detail.toast.feature", { name: item.name }));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("detail.toast.failedFeature"));
    } finally {
      setRunningAction(null);
    }
  };

  const handleDelete = async () => {
    setRunningAction("delete");
    try {
      await deleteItem(item.id);
      toast.success(t("detail.toast.delete", { name: item.name }));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("detail.toast.failedDelete"));
    } finally {
      setRunningAction(null);
    }
  };

  const handleSideload = async () => {
    if (!sideloadFile) return;
    try {
      await sideloadItem(item.type, sideloadFile);
      toast.success(t("detail.toast.sideload", {
        type: typeDisplayLabel(item.type).toLowerCase(),
        filename: sideloadFile.name,
      }));
      setSideloadFile(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("detail.toast.failedSideload"));
    }
  };

  const hasUpdate = updateInfo?.hasUpdate === true;

  return (
    <div className="flex h-full flex-col">
      {hasUpdate ? (
        <div className="flex items-center gap-2 border-b bg-blue-50 px-4 py-2 dark:bg-blue-950">
          <ArrowUpCircle className="size-4 text-blue-500" />
          <span className="flex-1 text-xs text-blue-700 dark:text-blue-300">
            {t("update.banner", { latest: updateInfo!.latestVersion, installed: updateInfo!.installedVersion })}
          </span>
          <Button
            size="sm"
            variant="default"
            onClick={() => onInstall?.(item)}
          >
            {t("update.button")}
          </Button>
        </div>
      ) : null}

      <div className="border-b p-4">
        <div className="mb-2 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 className="text-sm font-semibold">{item.name}</h2>
            <p className="text-xs text-muted-foreground">
              {t("item.byAuthor", { author: item.author_name })}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {isInstalled ? (
              <>
                <Button asChild size="sm" variant="secondary">
                  <a href={downstreamHref(item)}>{downstreamLabelText}</a>
                </Button>
                {consumption?.provenance?.sourceType !== "builtin" ? (
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button
                        size="sm"
                        variant="outline"
                        className="text-destructive"
                        disabled={uninstallLoading}
                      >
                        {uninstallLoading ? t("uninstall.removing") : t("uninstall.button")}
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>{t("uninstall.confirm", { name: item.name })}</AlertDialogTitle>
                        <AlertDialogDescription>
                          {t("uninstall.confirmDesc")}
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>{t("install.cancel")}</AlertDialogCancel>
                        <AlertDialogAction onClick={() => onUninstall?.(item)}>
                          {t("uninstall.button")}
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                ) : null}
              </>
            ) : (
              <Button
                size="sm"
                variant={consumption?.status === "blocked" ? "secondary" : "default"}
                disabled={consumption?.status === "blocked"}
                onClick={() => onInstall?.(item)}
              >
                {consumption?.status === "blocked" ? t("item.blocked") : t("item.install")}
              </Button>
            )}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline" className="text-xs">
            {typeDisplayLabel(item.type)}
          </Badge>
          {item.sourceType ? (
            <Badge variant="secondary" className="text-xs">
              {item.sourceType}
            </Badge>
          ) : null}
          {item.category ? (
            <Badge variant="outline" className="text-xs">
              {item.category}
            </Badge>
          ) : null}
          {item.is_verified ? (
            <Badge className="bg-blue-500 text-xs">
              <ShieldCheck className="mr-1 size-3" />
              {t("item.verified")}
            </Badge>
          ) : null}
          {item.is_featured ? (
            <Badge className="bg-amber-500 text-xs">
              <Sparkles className="mr-1 size-3" />
              {t("item.featured")}
            </Badge>
          ) : null}
          {state.badge ? <Badge variant="secondary" className="text-xs">{state.badge}</Badge> : null}
          <span className="flex items-center gap-1 text-xs text-muted-foreground">
            <Star className="size-3" />
            {item.avg_rating.toFixed(1)} ({item.rating_count})
          </span>
        </div>
      </div>

      <Tabs defaultValue="overview" className="flex flex-1 flex-col">
        <TabsList className="mx-4 mt-2 w-auto">
          <TabsTrigger value="overview" className="text-xs">
            {t("detail.tab.overview")}
          </TabsTrigger>
          <TabsTrigger value="versions" className="text-xs">
            {t("detail.tab.versions")}
          </TabsTrigger>
          <TabsTrigger value="reviews" className="text-xs">
            {t("detail.tab.reviews")}
          </TabsTrigger>
        </TabsList>
        <ScrollArea className="flex-1">
          <TabsContent value="overview" className="space-y-4 p-4">
            <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground">
              {state.detail}
            </div>

            {item.description ? (
              <p className="text-sm text-muted-foreground">{item.description}</p>
            ) : (
              <p className="text-sm text-muted-foreground">{t("item.noDescription")}</p>
            )}

            {item.tags.length > 0 ? (
              <div className="flex flex-wrap gap-1">
                {item.tags.map((tag) => (
                  <button
                    key={tag}
                    type="button"
                    className="rounded-full bg-muted px-2 py-0.5 text-xs hover:bg-muted/80 hover:ring-1 hover:ring-ring"
                    onClick={() => onTagClick?.(tag)}
                  >
                    {tag}
                  </button>
                ))}
              </div>
            ) : null}

            <div className="space-y-1 text-xs text-muted-foreground">
              <div>
                {t("detail.license", { license: item.license })}
              </div>
              {item.sourceType === "builtin" ? (
                <div>{t("detail.sourceBuiltin")}</div>
              ) : item.latest_version ? (
                <div>
                  {t("detail.latest", { version: item.latest_version })}
                </div>
              ) : (
                <div>{t("detail.noVersion")}</div>
              )}
              {consumption?.provenance?.selectedVersion ? (
                <div>
                  {t("detail.installedVersion", { version: consumption.provenance.selectedVersion })}
                </div>
              ) : null}
              {consumption?.provenance?.localPath ? (
                <div className="break-all">
                  {t("detail.localPath", { path: consumption.provenance.localPath })}
                </div>
              ) : item.localPath ? (
                <div className="break-all">
                  {t("detail.localPath", { path: item.localPath })}
                </div>
              ) : null}
            </div>

            {item.repository_url ? (
              <a
                href={item.repository_url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 text-xs text-blue-500 hover:underline"
              >
                <ExternalLink className="size-3" />
                {t("item.repository")}
              </a>
            ) : null}

            {typeof item.extra_metadata.docsRef === "string" &&
            item.extra_metadata.docsRef.trim() ? (
              <div className="text-xs text-muted-foreground">
                {t("detail.docsRef", { ref: item.extra_metadata.docsRef })}
              </div>
            ) : null}

            {item.type === "skill" && item.skillPreview ? (
              <SkillPackagePreviewPane preview={item.skillPreview} />
            ) : null}

            {item.type === "skill" && item.previewError ? (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
                {t("detail.skillPreviewUnavailable", { error: item.previewError })}
              </div>
            ) : null}

            <div className="space-y-2 rounded-lg border border-border/60 p-3">
              <p className="text-xs font-medium">{t("detail.sideload.title")}</p>
              {item.type === "plugin" ? (
                <>
                  <p className="text-xs text-muted-foreground">
                    {t("detail.sideload.pluginDesc")}
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => onSideLoad?.(item)}
                  >
                    {t("detail.sideload.pluginButton")}
                  </Button>
                </>
              ) : (
                <>
                  <p className="text-xs text-muted-foreground">
                    {t("detail.sideload.uploadDesc", { file: sideloadRootFile(item.type) })}
                  </p>
                  <div className="flex items-center gap-2">
                    <Input
                      type="file"
                      accept=".zip"
                      className="flex-1 text-xs"
                      onChange={(e) => setSideloadFile(e.target.files?.[0] ?? null)}
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={!sideloadFile || sideloadLoading}
                      onClick={() => void handleSideload()}
                    >
                      <Upload className="mr-1 size-3" />
                      {sideloadLoading
                        ? t("detail.sideload.installing")
                        : t("detail.sideload.uploadButton", { type: typeDisplayLabel(item.type).toLowerCase() })}
                    </Button>
                  </div>
                </>
              )}
            </div>

            <div className="space-y-2 rounded-lg border border-border/60 p-3">
              <p className="text-xs font-medium">{t("detail.moderation.title")}</p>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={item.is_verified || runningAction !== null}
                  onClick={() => void handleVerify()}
                >
                  {t("detail.moderation.verify")}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={item.is_featured || runningAction !== null}
                  onClick={() => void handleFeature()}
                >
                  {t("detail.moderation.feature")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                {t("detail.moderation.hint")}
              </p>
            </div>
          </TabsContent>

          <TabsContent value="versions" className="space-y-4 p-4">
            {canManageVersions ? (
              <div className="space-y-3 rounded-lg border border-border/60 p-3">
                <p className="text-xs font-medium">{t("detail.versionUpload.title")}</p>
                <div className="grid gap-2">
                  <Label htmlFor="version-input">{t("detail.versionUpload.versionLabel")}</Label>
                  <Input
                    id="version-input"
                    value={version}
                    onChange={(event) => setVersion(event.target.value)}
                    placeholder={t("detail.versionUpload.versionPlaceholder")}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="changelog-input">{t("detail.versionUpload.changelogLabel")}</Label>
                  <Textarea
                    id="changelog-input"
                    value={changelog}
                    onChange={(event) => setChangelog(event.target.value)}
                    rows={3}
                    placeholder={t("detail.versionUpload.changelogPlaceholder")}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="artifact-input">{t("detail.versionUpload.artifactLabel")}</Label>
                  <Input
                    id="artifact-input"
                    type="file"
                    onChange={(event) =>
                      setArtifact(event.target.files?.[0] ?? null)
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    {t("detail.versionUpload.artifactHint")}
                  </p>
                </div>
                <Button
                  type="button"
                  size="sm"
                  disabled={submittingVersion}
                  onClick={() => void handleUploadVersion()}
                >
                  <Upload className="mr-1 size-3" />
                  {submittingVersion ? t("detail.versionUpload.uploading") : t("detail.versionUpload.uploadButton")}
                </Button>
              </div>
            ) : null}

            <MarketplaceVersionList
              itemId={item.id}
              canManage={canManageVersions}
            />
          </TabsContent>

          <TabsContent value="reviews" className="space-y-3 p-4">
            <MarketplaceReviewDialog itemId={item.id} />
            {reviews.length === 0 ? (
              <p className="text-xs text-muted-foreground">{t("detail.noReviews")}</p>
            ) : (
              reviews.map((review) => (
                <div key={review.id} className="space-y-1 rounded border p-3">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-medium">{review.user_name}</span>
                    <div className="flex">
                      {Array.from({ length: 5 }).map((_, index) => (
                        <Star
                          key={index}
                          className={`size-3 ${
                            index < review.rating
                              ? "fill-yellow-400 text-yellow-400"
                              : "text-muted"
                          }`}
                        />
                      ))}
                    </div>
                  </div>
                  {review.comment ? (
                    <p className="text-xs text-muted-foreground">{review.comment}</p>
                  ) : null}
                </div>
              ))
            )}
          </TabsContent>
        </ScrollArea>
      </Tabs>

      {canManageVersions ? (
        <div className="border-t p-4">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-full text-destructive"
            disabled={runningAction !== null}
            onClick={() => void handleDelete()}
          >
            <Trash2 className="mr-1 size-3" />
            {t("detail.deleteItem")}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
