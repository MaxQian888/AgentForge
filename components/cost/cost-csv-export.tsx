"use client";

import { useTranslations } from "next-intl";
import { Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { CostBreakdownEntry } from "./cost-breakdown-table";

interface CostCsvExportProps {
  data: CostBreakdownEntry[];
  fileName?: string;
  className?: string;
  disabled?: boolean;
}

function escapeCsvCell(value: string | number): string {
  const str = String(value);
  if (/[",\n\r]/.test(str)) {
    return `"${str.replace(/"/g, '""')}"`;
  }
  return str;
}

export function buildCostCsv(rows: CostBreakdownEntry[]): string {
  const header = ["Date", "Category", "Agent", "Amount (USD)"];
  const lines = [header.join(",")];
  for (const row of rows) {
    lines.push(
      [
        escapeCsvCell(row.date),
        escapeCsvCell(row.category),
        escapeCsvCell(row.agent),
        escapeCsvCell(row.amountUsd.toFixed(2)),
      ].join(","),
    );
  }
  return lines.join("\n");
}

export function downloadCostCsv(
  rows: CostBreakdownEntry[],
  fileName = "cost-breakdown.csv",
): void {
  const csv = buildCostCsv(rows);
  if (typeof window === "undefined") return;
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

export function CostCsvExport({
  data,
  fileName,
  className,
  disabled,
}: CostCsvExportProps) {
  const t = useTranslations("cost");
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className={className}
      disabled={disabled || data.length === 0}
      onClick={() => downloadCostCsv(data, fileName)}
    >
      <Download className="size-3.5" aria-hidden />
      {t("exportCsv")}
    </Button>
  );
}
