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
import { useTranslations } from "next-intl";
import type {
  MarketplaceFilters,
  MarketplaceItemType,
} from "@/lib/stores/marketplace-store";

interface Props {
  filters: MarketplaceFilters;
  onChange: (f: Partial<MarketplaceFilters>) => void;
}

export function MarketplaceFilterPanel({ filters, onChange }: Props) {
  const t = useTranslations("marketplace");

  const types: { value: MarketplaceItemType; labelKey: string }[] = [
    { value: "all", labelKey: "filter.typeAll" },
    { value: "plugin", labelKey: "filter.typePlugin" },
    { value: "skill", labelKey: "filter.typeSkill" },
    { value: "role", labelKey: "filter.typeRole" },
    { value: "workflow_template", labelKey: "filter.typeWorkflow" },
  ];

  return (
    <div className="space-y-4 p-3">
      <div>
        <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
          {t("filter.type")}
        </Label>
        <RadioGroup
          value={filters.type}
          onValueChange={(v) =>
            onChange({ type: v as MarketplaceItemType, page: 1 })
          }
        >
          {types.map((tItem) => (
            <div key={tItem.value} className="flex items-center space-x-2">
              <RadioGroupItem value={tItem.value} id={`type-${tItem.value}`} />
              <Label htmlFor={`type-${tItem.value}`} className="text-sm cursor-pointer">
                {t(tItem.labelKey as Parameters<typeof t>[0])}
              </Label>
            </div>
          ))}
        </RadioGroup>
      </div>
      <div>
        <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
          {t("filter.category")}
        </Label>
        <Input
          placeholder={t("filter.categoryPlaceholder")}
          value={filters.category}
          onChange={(e) => onChange({ category: e.target.value, page: 1 })}
          className="h-8 text-sm"
        />
      </div>
      <div>
        <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
          {t("filter.sort")}
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
            <SelectItem value="downloads">{t("filter.sortDownloads")}</SelectItem>
            <SelectItem value="rating">{t("filter.sortRating")}</SelectItem>
            <SelectItem value="created_at">{t("filter.sortNewest")}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {filters.tags.length > 0 ? (
        <div>
          <Label className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2 block">
            {t("filter.tags")}
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
