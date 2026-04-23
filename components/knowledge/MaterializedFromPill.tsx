"use client";

import Link from "next/link";
import { FileSymlink } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useTranslations } from "next-intl";

/**
 * Shows a pill on a wiki page that was materialized from an ingested file.
 * Pass the source file's asset ID and optionally a label.
 */
export function MaterializedFromPill({
  sourceAssetId,
  label,
}: {
  sourceAssetId: string;
  label?: string;
}) {
  const t = useTranslations("knowledge");
  const resolvedLabel = label ?? t("materializedFrom");
  return (
    <Badge
      variant="secondary"
      className="inline-flex items-center gap-1.5 font-normal"
      asChild
    >
      <Link href={`/docs?assetId=${sourceAssetId}`}>
        <FileSymlink className="size-3.5 shrink-0" />
        {resolvedLabel}
      </Link>
    </Badge>
  );
}
