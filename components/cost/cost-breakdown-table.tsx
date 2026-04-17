"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export interface CostBreakdownEntry {
  id: string;
  date: string;
  category: string;
  agent: string;
  amountUsd: number;
}

interface CostBreakdownTableProps {
  data: CostBreakdownEntry[];
  pageSize?: number;
}

export function CostBreakdownTable({
  data,
  pageSize = 10,
}: CostBreakdownTableProps) {
  const t = useTranslations("cost");
  const [page, setPage] = useState(0);

  const sorted = useMemo(
    () =>
      [...data].sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0)),
    [data],
  );

  const totalPages = Math.max(1, Math.ceil(sorted.length / pageSize));
  const safePage = Math.min(page, totalPages - 1);
  const pageItems = sorted.slice(
    safePage * pageSize,
    safePage * pageSize + pageSize,
  );

  if (data.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">{t("noBreakdownData")}</p>
    );
  }

  return (
    <div className="space-y-3">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("colDate")}</TableHead>
            <TableHead>{t("colCategory")}</TableHead>
            <TableHead>{t("colAgent")}</TableHead>
            <TableHead className="text-right">{t("colAmount")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageItems.map((entry) => (
            <TableRow key={entry.id}>
              <TableCell className="font-mono text-xs">{entry.date}</TableCell>
              <TableCell>{entry.category}</TableCell>
              <TableCell>{entry.agent}</TableCell>
              <TableCell className="text-right">
                ${entry.amountUsd.toFixed(2)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>
          {t("paginationStatus", {
            page: (safePage + 1).toString(),
            total: totalPages.toString(),
            count: sorted.length.toString(),
          })}
        </span>
        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            className="h-7 px-2"
            aria-label={t("paginationPrev")}
            disabled={safePage === 0}
            onClick={() => setPage((p) => Math.max(0, p - 1))}
          >
            <ChevronLeft className="size-3.5" />
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-7 px-2"
            aria-label={t("paginationNext")}
            disabled={safePage >= totalPages - 1}
            onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
          >
            <ChevronRight className="size-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}
