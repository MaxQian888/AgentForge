"use client";

import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
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
      <select
        className="h-9 rounded-md border bg-background px-3 text-sm"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{t("fields.filterAll")}</option>
        {options.map((option) => (
          <option key={String(option)} value={String(option)}>
            {String(option)}
          </option>
        ))}
      </select>
    );
  }

  return <Input value={value} onChange={(event) => onChange(event.target.value)} placeholder={t("fields.filterPlaceholder", { name: field.name })} />;
}
