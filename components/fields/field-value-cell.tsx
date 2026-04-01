"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useCustomFieldStore, type CustomFieldDefinition, type CustomFieldValue } from "@/lib/stores/custom-field-store";

export function FieldValueCell({
  projectId,
  taskId,
  field,
  value,
}: {
  projectId: string;
  taskId: string;
  field: CustomFieldDefinition;
  value?: CustomFieldValue | null;
}) {
  const t = useTranslations("settings");
  const setTaskValue = useCustomFieldStore((state) => state.setTaskValue);
  const clearTaskValue = useCustomFieldStore((state) => state.clearTaskValue);
  const [draft, setDraft] = useState(value?.value == null ? "" : String(value.value));

  const commit = async (nextValue: unknown) => {
    if (nextValue === "" || nextValue === null || nextValue === undefined) {
      await clearTaskValue(projectId, taskId, field.id);
      return;
    }
    await setTaskValue(projectId, taskId, field.id, nextValue);
  };

  if (field.fieldType === "checkbox") {
    return (
      <input
        type="checkbox"
        checked={draft === "true" || draft === "1"}
        onChange={(event) => {
          const next = event.target.checked;
          setDraft(String(next));
          void commit(next);
        }}
      />
    );
  }

  if (field.fieldType === "select" || field.fieldType === "multi_select") {
    const options = Array.isArray(field.options) ? field.options : [];
    return (
      <Select
        value={draft || "__none__"}
        onValueChange={(value) => {
          const resolved = value === "__none__" ? "" : value;
          setDraft(resolved);
          void commit(resolved);
        }}
      >
        <SelectTrigger className="h-8 min-w-28 text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__none__">{t("fields.unset")}</SelectItem>
          {options.map((option) => (
            <SelectItem key={String(option)} value={String(option)}>
              {String(option)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }

  return (
    <Input
      className="h-8"
      value={draft}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={() => void commit(draft)}
    />
  );
}
