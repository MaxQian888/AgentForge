"use client";

import Link from "next/link";
import { FileSymlink } from "lucide-react";
import { Badge } from "@/components/ui/badge";

/**
 * Shows a pill on a wiki page that was materialized from an ingested file.
 * Pass the source file's asset ID and optionally a label.
 */
export function MaterializedFromPill({
  sourceAssetId,
  label = "Imported from file",
}: {
  sourceAssetId: string;
  label?: string;
}) {
  return (
    <Badge
      variant="secondary"
      className="inline-flex items-center gap-1.5 font-normal"
      asChild
    >
      <Link href={`/docs?assetId=${sourceAssetId}`}>
        <FileSymlink className="size-3.5 shrink-0" />
        {label}
      </Link>
    </Badge>
  );
}
