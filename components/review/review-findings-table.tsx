"use client";

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
  if (findings.length === 0) {
    return (
      <p className="py-4 text-center text-sm text-muted-foreground">
        No findings reported.
      </p>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Severity</TableHead>
          <TableHead>Category</TableHead>
          <TableHead>File:Line</TableHead>
          <TableHead>Message</TableHead>
          <TableHead>Suggestion</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {findings.map((f, i) => (
          <TableRow key={i}>
            <TableCell>
              <Badge
                variant="secondary"
                className={cn("text-xs", severityColors[f.severity] ?? "")}
              >
                {f.severity}
              </Badge>
            </TableCell>
            <TableCell className="text-xs">
              {f.category}
              {f.subcategory ? ` / ${f.subcategory}` : ""}
            </TableCell>
            <TableCell className="font-mono text-xs">
              {f.file ? `${f.file}${f.line ? `:${f.line}` : ""}` : "-"}
            </TableCell>
            <TableCell className="max-w-xs whitespace-normal text-xs">
              {f.message}
            </TableCell>
            <TableCell className="max-w-xs whitespace-normal text-xs text-muted-foreground">
              {f.suggestion ?? "-"}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
