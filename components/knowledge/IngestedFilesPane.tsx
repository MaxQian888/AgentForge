"use client";

import React, { useRef } from "react";
import { FileUp, RotateCcw, Loader2, ExternalLink } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useKnowledgeStore, type KnowledgeAsset, type IngestStatus } from "@/lib/stores/knowledge-store";

function formatBytes(bytes: number | null | undefined): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function IngestStatusBadge({ status }: { status: IngestStatus | null | undefined }) {
  if (!status) return null;
  const variants: Record<IngestStatus, React.ReactElement> = {
    pending: <Badge variant="outline">Pending</Badge>,
    processing: (
      <Badge variant="secondary" className="gap-1">
        <Loader2 className="size-3 animate-spin" />
        Processing
      </Badge>
    ),
    ready: (
      <Badge className="bg-green-500/15 text-green-700 hover:bg-green-500/25 dark:text-green-300">
        Ready
      </Badge>
    ),
    failed: <Badge variant="destructive">Failed</Badge>,
  };
  return variants[status] ?? <Badge variant="outline">{status}</Badge>;
}

export function IngestedFilesPane({
  projectId,
  onMaterializeAsWiki,
}: {
  projectId: string;
  onMaterializeAsWiki?: (assetId: string) => void;
}) {
  const { ingestedFiles, uploading, saving, uploadFile, reuploadFile } = useKnowledgeStore();
  const uploadRef = useRef<HTMLInputElement>(null);
  const reuploadRefs = useRef<Map<string, HTMLInputElement>>(new Map());

  const handleUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    void uploadFile(projectId, file);
    e.target.value = "";
  };

  const handleReupload = (assetId: string, e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    void reuploadFile(projectId, assetId, file);
    e.target.value = "";
  };

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border/60 bg-card/70 p-4">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold">Uploads</h2>
        <Button
          size="sm"
          variant="outline"
          disabled={uploading}
          onClick={() => uploadRef.current?.click()}
        >
          {uploading ? (
            <Loader2 className="mr-1 size-4 animate-spin" />
          ) : (
            <FileUp className="mr-1 size-4" />
          )}
          Upload file
        </Button>
        <input
          ref={uploadRef}
          type="file"
          className="hidden"
          onChange={handleUpload}
        />
      </div>

      {ingestedFiles.length === 0 ? (
        <p className="text-sm text-muted-foreground">No uploaded files yet.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {ingestedFiles.map((asset) => (
            <IngestedFileRow
              key={asset.id}
              asset={asset}
              saving={saving}
              onMaterializeAsWiki={onMaterializeAsWiki}
              onReupload={(e) => handleReupload(asset.id, e)}
              reuploadRefs={reuploadRefs}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

function IngestedFileRow({
  asset,
  saving,
  onMaterializeAsWiki,
  onReupload,
  reuploadRefs,
}: {
  asset: KnowledgeAsset;
  saving: boolean;
  onMaterializeAsWiki?: (assetId: string) => void;
  onReupload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  reuploadRefs: React.MutableRefObject<Map<string, HTMLInputElement>>;
}) {
  return (
    <li className="flex items-start justify-between gap-2 rounded-lg border border-border/60 bg-background px-3 py-2">
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">{asset.title}</div>
        <div className="mt-1 flex flex-wrap items-center gap-2">
          <IngestStatusBadge status={asset.ingestStatus} />
          <span className="text-xs text-muted-foreground">{formatBytes(asset.fileSize)}</span>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-1">
        <Button
          size="icon-sm"
          variant="ghost"
          title="Re-upload file"
          onClick={() => {
            const ref = reuploadRefs.current.get(asset.id);
            ref?.click();
          }}
        >
          <RotateCcw className="size-4" />
        </Button>
        <input
          type="file"
          className="hidden"
          ref={(el) => {
            if (el) reuploadRefs.current.set(asset.id, el);
            else reuploadRefs.current.delete(asset.id);
          }}
          onChange={onReupload}
        />
        {onMaterializeAsWiki && (
          <Button
            size="icon-sm"
            variant="ghost"
            title="Open as wiki page"
            disabled={saving}
            onClick={() => onMaterializeAsWiki(asset.id)}
          >
            {saving ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <ExternalLink className="size-4" />
            )}
          </Button>
        )}
      </div>
    </li>
  );
}
