"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { ChevronLeft, ChevronRight, CalendarDays } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { SchedulerJob } from "@/lib/stores/scheduler-store";

interface SchedulerUpcomingCalendarProps {
  jobs: SchedulerJob[];
}

interface CalendarEntry {
  jobKey: string;
  name: string;
  runAt: string;
}

interface DayCell {
  date: Date;
  isCurrentMonth: boolean;
  entries: CalendarEntry[];
}

const MAX_ENTRIES_PER_DAY = 3;

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date: Date, months: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function buildMonthGrid(cursor: Date, entries: CalendarEntry[]): DayCell[] {
  const firstOfMonth = startOfMonth(cursor);
  const leadingDays = firstOfMonth.getDay();
  const gridStart = new Date(firstOfMonth);
  gridStart.setDate(firstOfMonth.getDate() - leadingDays);

  const cells: DayCell[] = [];
  for (let i = 0; i < 42; i += 1) {
    const date = new Date(gridStart);
    date.setDate(gridStart.getDate() + i);
    const dayEntries = entries
      .filter((entry) => sameDay(new Date(entry.runAt), date))
      .sort((a, b) => new Date(a.runAt).getTime() - new Date(b.runAt).getTime());
    cells.push({
      date,
      isCurrentMonth: date.getMonth() === cursor.getMonth(),
      entries: dayEntries,
    });
  }
  return cells;
}

function formatTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

export function SchedulerUpcomingCalendar({ jobs }: SchedulerUpcomingCalendarProps) {
  const t = useTranslations("scheduler");
  const [cursor, setCursor] = useState(() => startOfMonth(new Date()));
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);

  const entries = useMemo<CalendarEntry[]>(() => {
    const collected: CalendarEntry[] = [];
    for (const job of jobs) {
      for (const run of job.upcomingRuns ?? []) {
        if (!run?.runAt) continue;
        collected.push({
          jobKey: job.jobKey,
          name: job.name,
          runAt: run.runAt,
        });
      }
    }
    return collected;
  }, [jobs]);

  const grid = useMemo(() => buildMonthGrid(cursor, entries), [cursor, entries]);
  const selectedEntries = useMemo(() => {
    if (!selectedDate) return [];
    return entries
      .filter((entry) => sameDay(new Date(entry.runAt), selectedDate))
      .sort((a, b) => new Date(a.runAt).getTime() - new Date(b.runAt).getTime());
  }, [selectedDate, entries]);

  const monthLabel = cursor.toLocaleDateString(undefined, {
    month: "long",
    year: "numeric",
  });
  const today = new Date();

  const weekdayLabels = useMemo(() => {
    // Generate localized short weekday labels (Sunday first).
    const base = new Date(2024, 0, 7); // a Sunday
    return Array.from({ length: 7 }, (_, index) => {
      const day = new Date(base);
      day.setDate(base.getDate() + index);
      return day.toLocaleDateString(undefined, { weekday: "short" });
    });
  }, []);

  if (entries.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <CalendarDays className="mx-auto mb-3 size-10 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">{t("calendar.noUpcoming")}</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <CalendarDays className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium">{monthLabel}</span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            className="h-7 w-7 p-0"
            onClick={() => setCursor((prev) => addMonths(prev, -1))}
            aria-label={t("calendar.prevMonth")}
          >
            <ChevronLeft className="size-3.5" />
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={() => {
              setCursor(startOfMonth(new Date()));
              setSelectedDate(null);
            }}
          >
            {t("calendar.today")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-7 w-7 p-0"
            onClick={() => setCursor((prev) => addMonths(prev, 1))}
            aria-label={t("calendar.nextMonth")}
          >
            <ChevronRight className="size-3.5" />
          </Button>
        </div>
      </div>

      <div
        role="grid"
        aria-label={t("calendar.gridLabel")}
        className="grid grid-cols-7 gap-px overflow-hidden rounded-md border bg-border text-xs"
      >
        {weekdayLabels.map((label) => (
          <div
            key={label}
            role="columnheader"
            className="bg-muted px-2 py-1 text-center font-medium uppercase tracking-wide text-muted-foreground"
          >
            {label}
          </div>
        ))}
        {grid.map((cell) => {
          const dateKey = cell.date.toISOString().slice(0, 10);
          const isToday = sameDay(cell.date, today);
          const isSelected = selectedDate && sameDay(cell.date, selectedDate);
          const overflow = Math.max(0, cell.entries.length - MAX_ENTRIES_PER_DAY);
          return (
            <button
              key={dateKey}
              type="button"
              role="gridcell"
              aria-label={`${cell.date.toLocaleDateString()} ${t("calendar.jobsCount", { count: cell.entries.length })}`}
              onClick={() => setSelectedDate(cell.date)}
              className={cn(
                "flex min-h-[76px] flex-col items-stretch gap-1 bg-background p-1 text-left transition-colors hover:bg-accent/50",
                !cell.isCurrentMonth && "text-muted-foreground/60",
                isSelected && "bg-accent",
              )}
            >
              <span
                className={cn(
                  "inline-flex size-5 items-center justify-center self-end rounded-full text-[11px]",
                  isToday && "bg-primary text-primary-foreground",
                )}
              >
                {cell.date.getDate()}
              </span>
              {cell.entries.slice(0, MAX_ENTRIES_PER_DAY).map((entry) => (
                <span
                  key={`${entry.jobKey}-${entry.runAt}`}
                  className="truncate rounded bg-primary/10 px-1 py-0.5 text-[10px] font-medium text-primary"
                  title={`${entry.name} — ${new Date(entry.runAt).toLocaleString()}`}
                >
                  {formatTime(entry.runAt)} · {entry.name}
                </span>
              ))}
              {overflow > 0 && (
                <span className="text-[10px] text-muted-foreground">
                  {t("calendar.moreItems", { count: overflow })}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {selectedDate && selectedEntries.length > 0 && (
        <div className="rounded-md border bg-muted/40 p-3">
          <div className="mb-2 text-xs font-medium text-muted-foreground">
            {t("calendar.dayDetail", { date: selectedDate.toLocaleDateString() })}
          </div>
          <ul className="flex flex-col gap-1">
            {selectedEntries.map((entry) => (
              <li
                key={`${entry.jobKey}-${entry.runAt}`}
                className="flex items-center justify-between gap-2 text-xs"
              >
                <span className="font-medium">{entry.name}</span>
                <span className="text-muted-foreground">
                  {new Date(entry.runAt).toLocaleString()}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
