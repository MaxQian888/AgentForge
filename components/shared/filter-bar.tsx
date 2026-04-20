"use client";

import { useState } from "react";
import { Filter, Search, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";

export interface FilterOption {
  value: string;
  label: string;
}

export interface FilterConfig {
  key: string;
  label: string;
  placeholder?: string;
  options: FilterOption[];
  value: string;
  onChange: (value: string) => void;
}

interface FilterBarProps {
  filters?: FilterConfig[];
  searchValue?: string;
  searchPlaceholder?: string;
  onSearch?: (query: string) => void;
  onReset?: () => void;
  moreFiltersLabel?: string;
  moreFiltersDescription?: string;
  resetLabel?: string;
  children?: React.ReactNode;
  className?: string;
}

function FilterSelect({ filter }: { filter: FilterConfig }) {
  return (
    <Select value={filter.value} onValueChange={filter.onChange}>
      <SelectTrigger className="h-9 w-full min-w-[140px] text-sm sm:h-8 sm:w-auto">
        <SelectValue placeholder={filter.placeholder ?? filter.label} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="all">{filter.label}</SelectItem>
        {filter.options.map((opt) => (
          <SelectItem key={opt.value} value={opt.value}>
            {opt.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

export function FilterBar({
  filters,
  searchValue,
  searchPlaceholder = "Search...",
  onSearch,
  onReset,
  moreFiltersLabel = "More filters",
  moreFiltersDescription,
  resetLabel = "Reset",
  children,
  className,
}: FilterBarProps) {
  const [sheetOpen, setSheetOpen] = useState(false);
  const hasActiveFilters = filters?.some((f) => f.value && f.value !== "all");
  const hasSearch = searchValue && searchValue.length > 0;
  const showReset = hasActiveFilters || hasSearch;
  const overflowFilters = filters ?? [];
  const hasOverflow = overflowFilters.length > 0;

  const handleReset = () => {
    if (onReset) {
      onReset();
    }
    setSheetOpen(false);
  };

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-[var(--space-stack-sm)] pb-[var(--space-stack-sm)]",
        className,
      )}
    >
      {onSearch && (
        <div className="relative w-full sm:max-w-xs">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={searchValue}
            onChange={(e) => onSearch(e.target.value)}
            placeholder={searchPlaceholder}
            className="h-9 pl-8 text-sm sm:h-8"
          />
        </div>
      )}

      {/* Inline filters on md+ viewports. */}
      <div className="hidden flex-wrap items-center gap-[var(--space-stack-sm)] md:flex">
        {filters?.map((filter) => (
          <FilterSelect key={filter.key} filter={filter} />
        ))}
        {children}
        {showReset && onReset && (
          <Button
            variant="ghost"
            size="sm"
            className="h-8 gap-1 text-xs text-muted-foreground"
            onClick={onReset}
          >
            <X className="size-3" />
            {resetLabel}
          </Button>
        )}
      </div>

      {/* Overflow sheet on sub-md viewports. */}
      {hasOverflow && (
        <div className="flex flex-1 flex-wrap items-center gap-[var(--space-stack-sm)] md:hidden">
          <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
            <SheetTrigger asChild>
              <Button
                variant="outline"
                size="sm"
                className="h-9 gap-1.5 text-sm"
                aria-label={moreFiltersLabel}
              >
                <Filter className="size-3.5" />
                {moreFiltersLabel}
                {hasActiveFilters ? (
                  <span className="ml-1 inline-flex size-4 items-center justify-center rounded-full bg-primary text-[10px] font-medium text-primary-foreground">
                    •
                  </span>
                ) : null}
              </Button>
            </SheetTrigger>
            <SheetContent side="bottom" className="max-h-[85vh] overflow-y-auto">
              <SheetHeader>
                <SheetTitle>{moreFiltersLabel}</SheetTitle>
                {moreFiltersDescription && (
                  <SheetDescription>{moreFiltersDescription}</SheetDescription>
                )}
              </SheetHeader>
              <div className="flex flex-col gap-[var(--space-stack-md)] px-4 pb-6">
                {overflowFilters.map((filter) => (
                  <div
                    key={filter.key}
                    className="flex flex-col gap-[var(--space-stack-xs)]"
                  >
                    <span className="text-xs font-medium text-muted-foreground">
                      {filter.label}
                    </span>
                    <FilterSelect filter={filter} />
                  </div>
                ))}
                {children && <div className="flex flex-col gap-2">{children}</div>}
                {showReset && onReset && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="mt-[var(--space-stack-sm)] justify-start gap-1 text-sm text-muted-foreground"
                    onClick={handleReset}
                  >
                    <X className="size-3.5" />
                    {resetLabel}
                  </Button>
                )}
              </div>
            </SheetContent>
          </Sheet>
          {showReset && onReset && (
            <Button
              variant="ghost"
              size="sm"
              className="h-9 gap-1 text-xs text-muted-foreground"
              onClick={onReset}
            >
              <X className="size-3" />
              {resetLabel}
            </Button>
          )}
        </div>
      )}

      {/* Children without filters shown inline on mobile too. */}
      {!hasOverflow && (
        <div className="flex flex-wrap items-center gap-[var(--space-stack-sm)] md:hidden">
          {children}
          {showReset && onReset && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 gap-1 text-xs text-muted-foreground"
              onClick={onReset}
            >
              <X className="size-3" />
              {resetLabel}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
