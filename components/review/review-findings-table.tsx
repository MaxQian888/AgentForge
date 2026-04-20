"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Check, X, Clock, Eye } from "lucide-react";
import type { ReviewFinding } from "@/lib/stores/review-store";
import { useReviewStore } from "@/lib/stores/review-store";
import { getReviewRiskLabel } from "./review-copy";
import { FindingPatchModal } from "./finding-patch-modal";

const severityColors: Record<string, string> = {
  critical: "bg-red-500/15 text-red-700 dark:text-red-400",
  high: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  medium: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  low: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  info: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
};

const decisionColors: Record<string, string> = {
  pending: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  approved: "bg-green-500/15 text-green-700 dark:text-green-400",
  dismissed: "bg-zinc-500/15 text-zinc-500 dark:text-zinc-500",
  deferred: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  needs_manual_fix: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
};

interface ReviewFindingsTableProps {
  findings: ReviewFinding[];
}

export function ReviewFindingsTable({ findings }: ReviewFindingsTableProps) {
  const t = useTranslations("reviews");
  const decideFinding = useReviewStore((s) => s.decideFinding);
  const [patchModal, setPatchModal] = useState<{ open: boolean; patch: string | null }>({
    open: false,
    patch: null,
  });

  if (findings.length === 0) {
    return (
      <p className="py-4 text-center text-sm text-muted-foreground">
        {t("noFindingsReported")}
      </p>
    );
  }

  return (
    <>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("findingSeverity")}</TableHead>
            <TableHead>{t("findingCategory")}</TableHead>
            <TableHead>{t("findingSource")}</TableHead>
            <TableHead>{t("findingFileLine")}</TableHead>
            <TableHead>{t("findingMessage")}</TableHead>
            <TableHead>{t("findingSuggestion")}</TableHead>
            <TableHead>Decision</TableHead>
            <TableHead>Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {findings.map((finding, index) => (
            <TableRow key={index}>
              <TableCell>
                <Badge
                  variant="secondary"
                  className={cn("text-xs", severityColors[finding.severity] ?? "")}
                >
                  {getReviewRiskLabel(t, finding.severity)}
                </Badge>
              </TableCell>
              <TableCell className="text-xs">
                {finding.category}
                {finding.subcategory ? ` / ${finding.subcategory}` : ""}
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {finding.sources && finding.sources.length > 1
                  ? `${finding.sources[0]} +${finding.sources.length - 1}`
                  : finding.sources?.[0] ?? "-"}
              </TableCell>
              <TableCell className="font-mono text-xs">
                {finding.file ? `${finding.file}${finding.line ? `:${finding.line}` : ""}` : "-"}
              </TableCell>
              <TableCell className="max-w-xs whitespace-normal text-xs">
                {finding.message}
              </TableCell>
              <TableCell className="max-w-xs whitespace-normal text-xs text-muted-foreground">
                {finding.suggestion ?? "-"}
              </TableCell>
              <TableCell>
                <Badge
                  variant="secondary"
                  className={cn("text-xs", decisionColors[finding.decision ?? "pending"] ?? "")}
                >
                  {finding.decision ?? "pending"}
                </Badge>
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    title="Approve"
                    data-testid={`approve-${finding.id ?? index}`}
                    onClick={() => finding.id && decideFinding(finding.id, "approve")}
                  >
                    <Check className="h-3.5 w-3.5 text-green-600" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    title="Dismiss"
                    data-testid={`dismiss-${finding.id ?? index}`}
                    onClick={() => finding.id && decideFinding(finding.id, "dismiss")}
                  >
                    <X className="h-3.5 w-3.5 text-red-600" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    title="Defer"
                    data-testid={`defer-${finding.id ?? index}`}
                    onClick={() => finding.id && decideFinding(finding.id, "defer")}
                  >
                    <Clock className="h-3.5 w-3.5 text-blue-600" />
                  </Button>
                  {finding.suggestedPatch && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      title="Show patch"
                      data-testid={`show-patch-${finding.id ?? index}`}
                      onClick={() =>
                        setPatchModal({ open: true, patch: finding.suggestedPatch ?? null })
                      }
                    >
                      <Eye className="h-3.5 w-3.5" />
                    </Button>
                  )}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <FindingPatchModal
        patch={patchModal.patch}
        open={patchModal.open}
        onClose={() => setPatchModal({ open: false, patch: null })}
      />
    </>
  );
}
