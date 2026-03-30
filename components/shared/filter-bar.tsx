"use client";

import { Search, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
  children?: React.ReactNode;
  className?: string;
}

export function FilterBar({
  filters,
  searchValue,
  searchPlaceholder = "Search...",
  onSearch,
  onReset,
  children,
  className,
}: FilterBarProps) {
  const hasActiveFilters = filters?.some((f) => f.value && f.value !== "all");
  const hasSearch = searchValue && searchValue.length > 0;
  const showReset = hasActiveFilters || hasSearch;

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-2 pb-3",
        className
      )}
    >
      {onSearch && (
        <div className="relative w-full max-w-xs">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={searchValue}
            onChange={(e) => onSearch(e.target.value)}
            placeholder={searchPlaceholder}
            className="h-8 pl-8 text-sm"
          />
        </div>
      )}
      {filters?.map((filter) => (
        <Select
          key={filter.key}
          value={filter.value}
          onValueChange={filter.onChange}
        >
          <SelectTrigger className="h-8 w-auto min-w-[120px] text-sm">
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
          Reset
        </Button>
      )}
    </div>
  );
}
