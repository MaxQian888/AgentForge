"use client";

import { Input } from "@/components/ui/input";
import { Search } from "lucide-react";
import { useRef } from "react";
import { useTranslations } from "next-intl";

interface Props {
  value: string;
  onChange: (v: string) => void;
  onSearch: (q: string) => void;
}

export function MarketplaceSearchBar({ value, onChange, onSearch }: Props) {
  const t = useTranslations("marketplace");
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleChange = (v: string) => {
    onChange(v);
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => onSearch(v), 300);
  };

  return (
    <div className="relative">
      <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
      <Input
        className="pl-8 h-9"
        placeholder={t("search.placeholder")}
        value={value}
        onChange={(e) => handleChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            if (timerRef.current) clearTimeout(timerRef.current);
            onSearch(value);
          }
        }}
      />
    </div>
  );
}
