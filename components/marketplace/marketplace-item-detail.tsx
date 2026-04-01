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
  type MarketplaceConsumptionRecord,
  type MarketplaceItem,
  type MarketplaceUpdateInfo,
} from "@/lib/stores/marketplace-store";
import { MarketplaceVersionList } from "./marketplace-version-list";
import { MarketplaceReviewDialog } from "./marketplace-review-dialog";
import { SkillPackagePreviewPane } from "./skill-package-preview";
import { toast } from "sonner";

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

function statusCopy(consumption: MarketplaceConsumptionRecord | null) {
  if (!consumption) {
    return {
      badge: null,
      detail: "Not installed yet. Install it from the marketplace or use a supported local side-load flow.",
    };
  }

  if (consumption.status === "installed" && consumption.provenance?.sourceType === "builtin") {
    return {
      badge: consumption.used ? "Manage in workspace" : "Available locally",
      detail: consumption.used
        ? `Managed through ${consumption.consumerSurface}.`
        : `Already available through ${consumption.consumerSurface} in this checkout.`,
    };
  }

  switch (consumption.status) {
    case "installed":
      return {
        badge: consumption.used ? "Manage in workspace" : "Installed",
        detail: consumption.used
          ? `Managed through ${consumption.consumerSurface}.`
          : "Installed successfully and ready for downstream handoff.",
      };
    case "warning":
      return {
        badge: "Needs attention",
        detail: consumption.warning ?? consumption.failureReason ?? "Installation completed with warnings.",
      };
    default:
      return {
        badge: "Blocked",
        detail: consumption.failureReason ?? "This item cannot be installed in the current checkout.",
      };
  }
}

function downstreamHref(item: MarketplaceItem): string {
  switch (item.type) {
    case "plugin":
      return "/plugins";
    default:
      return "/roles";
  }
}

function downstreamLabel(item: MarketplaceItem): string {
  switch (item.type) {
    case "plugin":
      return "Open plugin console";
    case "role":
      return "Open roles workspace";
    default:
      return "Open role authoring";
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

  const state = useMemo(() => statusCopy(consumption), [consumption]);
  const isInstalled = consumption?.status === "installed" && consumption.installed;
  const canManageVersions = currentUserId === item.author_id;

  const handleUploadVersion = async () => {
    if (!artifact || !version.trim()) {
      toast.error("Version and artifact are required.");
      return;
    }
    setSubmittingVersion(true);
    try {
      await uploadVersion(item.id, {
        version: version.trim(),
        changelog,
        artifact,
      });
      toast.success(`Uploaded ${version.trim()} for ${item.name}.`);
      setVersion("");
      setChangelog("");
      setArtifact(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to upload version");
    } finally {
      setSubmittingVersion(false);
    }
  };

  const handleVerify = async () => {
    setRunningAction("verify");
    try {
      await verifyItem(item.id);
      toast.success(`${item.name} marked as verified.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to verify item");
    } finally {
      setRunningAction(null);
    }
  };

  const handleFeature = async () => {
    setRunningAction("feature");
    try {
      await featureItem(item.id);
      toast.success(`${item.name} added to featured items.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to feature item");
    } finally {
      setRunningAction(null);
    }
  };

  const handleDelete = async () => {
    setRunningAction("delete");
    try {
      await deleteItem(item.id);
      toast.success(`${item.name} deleted.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to delete item");
    } finally {
      setRunningAction(null);
    }
  };

  const handleSideload = async () => {
    if (!sideloadFile) return;
    try {
      await sideloadItem(item.type, sideloadFile);
      toast.success(`Side-loaded ${item.type} from ${sideloadFile.name}.`);
      setSideloadFile(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Sideload failed");
    }
  };

  const hasUpdate = updateInfo?.hasUpdate === true;

  return (
    <div className="flex h-full flex-col">
      {hasUpdate ? (
        <div className="flex items-center gap-2 border-b bg-blue-50 px-4 py-2 dark:bg-blue-950">
          <ArrowUpCircle className="size-4 text-blue-500" />
          <span className="flex-1 text-xs text-blue-700 dark:text-blue-300">
            Update available: v{updateInfo!.latestVersion} (installed: v{updateInfo!.installedVersion})
          </span>
          <Button
            size="sm"
            variant="default"
            onClick={() => onInstall?.(item)}
          >
            Update
          </Button>
        </div>
      ) : null}

      <div className="border-b p-4">
        <div className="mb-2 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 className="text-sm font-semibold">{item.name}</h2>
            <p className="text-xs text-muted-foreground">by {item.author_name}</p>
          </div>
          <div className="flex items-center gap-2">
            {isInstalled ? (
              <>
                <Button asChild size="sm" variant="secondary">
                  <a href={downstreamHref(item)}>{downstreamLabel(item)}</a>
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
                        {uninstallLoading ? "Removing..." : "Uninstall"}
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>Uninstall {item.name}?</AlertDialogTitle>
                        <AlertDialogDescription>
                          This will remove the installed files and consumption state for this {item.type}. This action cannot be undone.
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction onClick={() => onUninstall?.(item)}>
                          Uninstall
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
                {consumption?.status === "blocked" ? "Blocked" : "Install"}
              </Button>
            )}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline" className="text-xs">
            {item.type}
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
              Verified
            </Badge>
          ) : null}
          {item.is_featured ? (
            <Badge className="bg-amber-500 text-xs">
              <Sparkles className="mr-1 size-3" />
              Featured
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
          <TabsContent value="overview" className="space-y-4 p-4">
            <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground">
              {state.detail}
            </div>

            {item.description ? (
              <p className="text-sm text-muted-foreground">{item.description}</p>
            ) : (
              <p className="text-sm text-muted-foreground">No description provided.</p>
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
                License: <span className="text-foreground">{item.license}</span>
              </div>
              {item.sourceType === "builtin" ? (
                <div>Source: <span className="text-foreground">Repo-owned built-in skill</span></div>
              ) : item.latest_version ? (
                <div>
                  Latest: <span className="text-foreground">{item.latest_version}</span>
                </div>
              ) : (
                <div>No published version yet.</div>
              )}
              {consumption?.provenance?.selectedVersion ? (
                <div>
                  Installed version:{" "}
                  <span className="text-foreground">
                    {consumption.provenance.selectedVersion}
                  </span>
                </div>
              ) : null}
              {consumption?.provenance?.localPath ? (
                <div className="break-all">
                  Local path:{" "}
                  <span className="text-foreground">
                    {consumption.provenance.localPath}
                  </span>
                </div>
              ) : item.localPath ? (
                <div className="break-all">
                  Local path: <span className="text-foreground">{item.localPath}</span>
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
                Repository
              </a>
            ) : null}

            {typeof item.extra_metadata.docsRef === "string" &&
            item.extra_metadata.docsRef.trim() ? (
              <div className="text-xs text-muted-foreground">
                Docs reference:{" "}
                <span className="text-foreground">{item.extra_metadata.docsRef}</span>
              </div>
            ) : null}

            {item.type === "skill" && item.skillPreview ? (
              <SkillPackagePreviewPane preview={item.skillPreview} />
            ) : null}

            {item.type === "skill" && item.previewError ? (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
                Skill preview unavailable: {item.previewError}
              </div>
            ) : null}

            <div className="space-y-2 rounded-lg border border-border/60 p-3">
              <p className="text-xs font-medium">Local side-load</p>
              {item.type === "plugin" ? (
                <>
                  <p className="text-xs text-muted-foreground">
                    Reuse the existing local plugin install seam from within the marketplace workspace.
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => onSideLoad?.(item)}
                  >
                    Side-load local plugin
                  </Button>
                </>
              ) : (
                <>
                  <p className="text-xs text-muted-foreground">
                    Upload a zip package containing a valid {item.type === "role" ? "role.yaml" : "SKILL.md"} at its root.
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
                      {sideloadLoading ? "Installing..." : `Side-load ${item.type}`}
                    </Button>
                  </div>
                </>
              )}
            </div>

            <div className="space-y-2 rounded-lg border border-border/60 p-3">
              <p className="text-xs font-medium">Moderation</p>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={item.is_verified || runningAction !== null}
                  onClick={() => void handleVerify()}
                >
                  Verify
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={item.is_featured || runningAction !== null}
                  onClick={() => void handleFeature()}
                >
                  Feature
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                Admin-only actions fail with an explicit permission error when the current operator is not allowed to moderate this item.
              </p>
            </div>
          </TabsContent>

          <TabsContent value="versions" className="space-y-4 p-4">
            {canManageVersions ? (
              <div className="space-y-3 rounded-lg border border-border/60 p-3">
                <p className="text-xs font-medium">Upload a new version</p>
                <div className="grid gap-2">
                  <Label htmlFor="version-input">Version</Label>
                  <Input
                    id="version-input"
                    value={version}
                    onChange={(event) => setVersion(event.target.value)}
                    placeholder="1.2.0"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="changelog-input">Changelog</Label>
                  <Textarea
                    id="changelog-input"
                    value={changelog}
                    onChange={(event) => setChangelog(event.target.value)}
                    rows={3}
                    placeholder="What changed in this release?"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="artifact-input">Artifact</Label>
                  <Input
                    id="artifact-input"
                    type="file"
                    onChange={(event) =>
                      setArtifact(event.target.files?.[0] ?? null)
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    Upload a zip package whose root matches the current marketplace artifact contract for this item type.
                  </p>
                </div>
                <Button
                  type="button"
                  size="sm"
                  disabled={submittingVersion}
                  onClick={() => void handleUploadVersion()}
                >
                  <Upload className="mr-1 size-3" />
                  {submittingVersion ? "Uploading..." : "Upload version"}
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
              <p className="text-xs text-muted-foreground">No reviews yet.</p>
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
            Delete item
          </Button>
        </div>
      ) : null}
    </div>
  );
}
