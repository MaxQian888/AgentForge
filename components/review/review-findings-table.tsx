"use client";

import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ReviewFinding } from "@/lib/stores/review-store";
import { getReviewRiskLabel } from "./review-copy";

const severityColors: Record<string, string> = {
  critical: "bg-red-500/15 text-red-700 dark:text-red-400",
  high: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  medium: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  low: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  info: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
};

interface ReviewFindingsTableProps {
  findings: ReviewFinding[];
}

export function ReviewFindingsTable({ findings }: ReviewFindingsTableProps) {
  const t = useTranslations("reviews");

  if (findings.length === 0) {
    return (
      <p className="py-4 text-center text-sm text-muted-foreground">
        {t("noFindingsReported")}
      </p>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("findingSeverity")}</TableHead>
          <TableHead>{t("findingCategory")}</TableHead>
          <TableHead>{t("findingSource")}</TableHead>
          <TableHead>{t("findingFileLine")}</TableHead>
          <TableHead>{t("findingMessage")}</TableHead>
          <TableHead>{t("findingSuggestion")}</TableHead>
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
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
