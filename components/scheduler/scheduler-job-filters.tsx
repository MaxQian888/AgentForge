"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import { FilterBar } from "@/components/shared/filter-bar";
import type { SchedulerJob, SchedulerJobListFilters } from "@/lib/stores/scheduler-store";

interface SchedulerJobFiltersProps {
  jobs: SchedulerJob[];
  filters: SchedulerJobListFilters;
  onFiltersChange: (filters: Partial<SchedulerJobListFilters>) => void;
  onReset: () => void;
}

const STATUS_OPTIONS = [
  { value: "scheduled", labelKey: "jobFilters.statusScheduled" },
  { value: "running", labelKey: "jobFilters.statusRunning" },
  { value: "succeeded", labelKey: "jobFilters.statusSucceeded" },
  { value: "failed", labelKey: "jobFilters.statusFailed" },
  { value: "paused", labelKey: "jobFilters.statusPaused" },
];

export function SchedulerJobFilters({
  jobs,
  filters,
  onFiltersChange,
  onReset,
}: SchedulerJobFiltersProps) {
  const t = useTranslations("scheduler");

  const scopeOptions = useMemo(() => {
    const scopes = Array.from(new Set(jobs.map((job) => job.scope).filter(Boolean)));
    scopes.sort();
    return scopes.map((scope) => ({ value: scope, label: scope }));
  }, [jobs]);

  return (
    <FilterBar
      filters={[
        {
          key: "status",
          label: t("jobFilters.allStatus"),
          value: filters.status,
          onChange: (value) => onFiltersChange({ status: value }),
          options: STATUS_OPTIONS.map((option) => ({
            value: option.value,
            label: t(option.labelKey),
          })),
        },
        {
          key: "scope",
          label: t("jobFilters.allScopes"),
          value: filters.scope,
          onChange: (value) => onFiltersChange({ scope: value }),
          options: scopeOptions,
        },
      ]}
      onReset={onReset}
    />
  );
}
