"use client";

import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { CustomFieldDefinition } from "@/lib/stores/custom-field-store";

export function FieldFilterControl({
  field,
  value,
  onChange,
}: {
  field: CustomFieldDefinition;
  value: string;
  onChange: (value: string) => void;
}) {
  const t = useTranslations("settings");

  if (field.fieldType === "select" || field.fieldType === "multi_select") {
    const options = Array.isArray(field.options) ? field.options : [];
    return (
      <Select
        value={value || "__none__"}
        onValueChange={(val) => onChange(val === "__none__" ? "" : val)}
      >
        <SelectTrigger className="h-9 text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__none__">{t("fields.filterAll")}</SelectItem>
          {options.map((option) => (
            <SelectItem key={String(option)} value={String(option)}>
              {String(option)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }

  return <Input value={value} onChange={(event) => onChange(event.target.value)} placeholder={t("fields.filterPlaceholder", { name: field.name })} />;
}
