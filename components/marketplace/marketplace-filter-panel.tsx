"use client";

import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { X } from "lucide-react";
import type {
  MarketplaceFilters,
  MarketplaceItemType,
} from "@/lib/stores/marketplace-store";

interface Props {
  filters: MarketplaceFilters;
  onChange: (f: Partial<MarketplaceFilters>) => void;
}

const TYPES: { value: MarketplaceItemType; label: string }[] = [
  { value: "all", label: "All Types" },
  { value: "plugin", label: "Plugins" },
  { value: "skill", label: "Skills" },
  { value: "role", label: "Roles" },
  { value: "workflow_template", label: "Workflows" },
];

export function MarketplaceFilterPanel({ filters, onChange }: Props) {
  return (
    <div className="space-y-4 p-3">
      <div>
        <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
          Type
        </Label>
        <RadioGroup
          value={filters.type}
          onValueChange={(v) =>
            onChange({ type: v as MarketplaceItemType, page: 1 })
          }
        >
          {TYPES.map((t) => (
            <div key={t.value} className="flex items-center space-x-2">
              <RadioGroupItem value={t.value} id={`type-${t.value}`} />
              <Label htmlFor={`type-${t.value}`} className="text-sm cursor-pointer">
                {t.label}
              </Label>
            </div>
          ))}
        </RadioGroup>
      </div>
      <div>
        <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
          Category
        </Label>
        <Input
          placeholder="e.g. testing"
          value={filters.category}
          onChange={(e) => onChange({ category: e.target.value, page: 1 })}
          className="h-8 text-sm"
        />
      </div>
      <div>
        <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
          Sort by
        </Label>
        <Select
          value={filters.sort}
          onValueChange={(v) =>
            onChange({ sort: v as MarketplaceFilters["sort"] })
          }
        >
          <SelectTrigger className="h-8 text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="downloads">Most Downloaded</SelectItem>
            <SelectItem value="rating">Top Rated</SelectItem>
            <SelectItem value="created_at">Newest</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {filters.tags.length > 0 ? (
        <div>
          <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
            Tags
          </Label>
          <div className="flex flex-wrap gap-1">
            {filters.tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="text-xs gap-1">
                {tag}
                <button
                  type="button"
                  className="ml-0.5 hover:text-destructive"
                  onClick={() =>
                    onChange({
                      tags: filters.tags.filter((t) => t !== tag),
                      page: 1,
                    })
                  }
                >
                  <X className="size-3" />
                </button>
              </Badge>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
