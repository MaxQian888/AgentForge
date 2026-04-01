"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useBreakpoint } from "@/hooks/use-breakpoint";
import { cn } from "@/lib/utils";

export interface ResponsiveTableColumn<T> {
  key: string;
  header: React.ReactNode;
  renderCell: (row: T) => React.ReactNode;
  cardLabel?: React.ReactNode;
  headClassName?: string;
  cellClassName?: string;
  hideOnTablet?: boolean;
  hideOnCard?: boolean;
}

interface ResponsiveTableProps<T> {
  columns: ResponsiveTableColumn<T>[];
  data: T[];
  getRowId: (row: T) => string;
  className?: string;
  emptyState?: React.ReactNode;
  mobileCardTitle?: (row: T) => React.ReactNode;
  mobileCardDescription?: (row: T) => React.ReactNode;
}

export function ResponsiveTable<T>({
  columns,
  data,
  getRowId,
  className,
  emptyState = null,
  mobileCardTitle,
  mobileCardDescription,
}: ResponsiveTableProps<T>) {
  const { isMobile, isTablet } = useBreakpoint();
  const [expandedRowIds, setExpandedRowIds] = useState<Set<string>>(new Set());

  if (data.length === 0) {
    return <>{emptyState}</>;
  }

  if (isMobile) {
    const cardColumns = columns.filter((column) => !column.hideOnCard);

    return (
      <div className={cn("space-y-[var(--space-grid-gap)]", className)}>
        {data.map((row) => {
          const rowId = getRowId(row);

          return (
            <article
              key={rowId}
              className="rounded-lg border bg-card p-[var(--space-card-padding)] shadow-sm"
            >
              {mobileCardTitle ? (
                <div className="text-sm font-semibold text-foreground">
                  {mobileCardTitle(row)}
                </div>
              ) : null}
              {mobileCardDescription ? (
                <div className="mt-[var(--space-stack-xs)] text-fluid-body text-muted-foreground">
                  {mobileCardDescription(row)}
                </div>
              ) : null}
              <dl className="mt-[var(--space-stack-sm)] space-y-[var(--space-stack-sm)]">
                {cardColumns.map((column) => (
                  <div
                    key={column.key}
                    className="grid grid-cols-[minmax(0,120px)_minmax(0,1fr)] gap-3"
                  >
                    <dt className="text-fluid-caption font-medium text-muted-foreground">
                      {column.cardLabel ?? column.header}
                    </dt>
                    <dd className="min-w-0 text-sm text-foreground">
                      {column.renderCell(row)}
                    </dd>
                  </div>
                ))}
              </dl>
            </article>
          );
        })}
      </div>
    );
  }

  const visibleColumns = isTablet
    ? columns.filter((column) => !column.hideOnTablet)
    : columns;
  const hiddenColumns = isTablet
    ? columns.filter((column) => column.hideOnTablet)
    : [];

  const toggleExpanded = (rowId: string) => {
    setExpandedRowIds((current) => {
      const next = new Set(current);

      if (next.has(rowId)) {
        next.delete(rowId);
      } else {
        next.add(rowId);
      }

      return next;
    });
  };

  return (
    <div className={className}>
      <Table>
        <TableHeader>
          <TableRow>
            {visibleColumns.map((column) => (
              <TableHead key={column.key} className={column.headClassName}>
                {column.header}
              </TableHead>
            ))}
            {hiddenColumns.length > 0 ? (
              <TableHead className="w-[120px] text-right">Details</TableHead>
            ) : null}
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.map((row) => {
            const rowId = getRowId(row);
            const expanded = expandedRowIds.has(rowId);

            return (
              <FragmentRow
                key={rowId}
                mainRow={
                  <TableRow>
                    {visibleColumns.map((column) => (
                      <TableCell key={column.key} className={column.cellClassName}>
                        {column.renderCell(row)}
                      </TableCell>
                    ))}
                    {hiddenColumns.length > 0 ? (
                      <TableCell className="text-right">
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          onClick={() => toggleExpanded(rowId)}
                        >
                          {expanded ? "Show less" : "Show more"}
                        </Button>
                      </TableCell>
                    ) : null}
                  </TableRow>
                }
                detailRow={
                  hiddenColumns.length > 0 && expanded ? (
                    <TableRow>
                      <TableCell
                        colSpan={visibleColumns.length + 1}
                        className="bg-muted/30"
                      >
                        <dl className="grid gap-3 py-1 sm:grid-cols-2">
                          {hiddenColumns.map((column) => (
                            <div key={column.key} className="space-y-1">
                              <dt className="text-fluid-caption font-medium text-muted-foreground">
                                {column.cardLabel ?? column.header}
                              </dt>
                              <dd className="text-sm text-foreground">
                                {column.renderCell(row)}
                              </dd>
                            </div>
                          ))}
                        </dl>
                      </TableCell>
                    </TableRow>
                  ) : null
                }
              />
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}

function FragmentRow({
  mainRow,
  detailRow,
}: {
  mainRow: React.ReactNode;
  detailRow: React.ReactNode;
}) {
  return (
    <>
      {mainRow}
      {detailRow}
    </>
  );
}
