"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import {
  useCustomFieldStore,
  type CustomFieldDefinition,
} from "@/lib/stores/custom-field-store";

const FIELD_TYPES = ["text", "number", "select", "multi_select", "date", "user", "url", "checkbox"];
const EMPTY_DEFINITIONS: CustomFieldDefinition[] = [];

function moveField(ids: string[], from: number, to: number) {
  const next = [...ids];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next;
}

export function FieldDefinitionEditor({ projectId }: { projectId: string }) {
  const t = useTranslations("settings");
  const definitionsByProject = useCustomFieldStore((state) => state.definitionsByProject);
  const fetchDefinitions = useCustomFieldStore((state) => state.fetchDefinitions);
  const createDefinition = useCustomFieldStore((state) => state.createDefinition);
  const deleteDefinition = useCustomFieldStore((state) => state.deleteDefinition);
  const reorderDefinitions = useCustomFieldStore((state) => state.reorderDefinitions);

  const [name, setName] = useState("");
  const [fieldType, setFieldType] = useState("text");
  const [options, setOptions] = useState("");
  const [required, setRequired] = useState(false);

  const definitions = useMemo(
    () => definitionsByProject[projectId] ?? EMPTY_DEFINITIONS,
    [definitionsByProject, projectId]
  );

  useEffect(() => {
    void fetchDefinitions(projectId);
  }, [fetchDefinitions, projectId]);

  const sorted = useMemo(
    () => [...definitions].sort((left, right) => left.sortOrder - right.sortOrder),
    [definitions]
  );

  const handleCreate = async () => {
    await createDefinition(projectId, {
      name,
      fieldType,
      options:
        fieldType === "select" || fieldType === "multi_select"
          ? options.split(",").map((item) => item.trim()).filter(Boolean)
          : [],
      required,
    });
    setName("");
    setOptions("");
    setRequired(false);
    setFieldType("text");
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-4">
        <div className="space-y-2 md:col-span-2">
          <Label>{t("fields.fieldName")}</Label>
          <Input value={name} onChange={(event) => setName(event.target.value)} placeholder={t("fields.fieldNamePlaceholder")} />
        </div>
        <div className="space-y-2">
          <Label>{t("fields.type")}</Label>
          <Select value={fieldType} onValueChange={(value) => setFieldType(value)}>
            <SelectTrigger className="h-10 w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {FIELD_TYPES.map((type) => (
                <SelectItem key={type} value={type}>
                  {t(`fields.types.${type}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>{t("fields.required")}</Label>
          <div className="flex h-10 items-center gap-2 rounded-md border px-3 text-sm">
            <Checkbox
              id="field-required"
              checked={required}
              onCheckedChange={(checked) => setRequired(checked === true)}
            />
            <label htmlFor="field-required" className="cursor-pointer">
              {t("fields.required")}
            </label>
          </div>
        </div>
      </div>

      {(fieldType === "select" || fieldType === "multi_select") && (
        <div className="space-y-2">
          <Label>{t("fields.options")}</Label>
          <Input value={options} onChange={(event) => setOptions(event.target.value)} placeholder={t("fields.optionsPlaceholder")} />
        </div>
      )}

      <Button type="button" onClick={() => void handleCreate()} disabled={!name.trim()}>
        {t("fields.addField")}
      </Button>

      <div className="space-y-2">
        {sorted.map((field, index) => (
          <div key={field.id} className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
            <div>
              <div className="font-medium">{field.name}</div>
              <div className="text-muted-foreground">
                {t(`fields.types.${field.fieldType}`)}
                {field.required ? t("fields.requiredSuffix") : ""}
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={index === 0}
                onClick={() => void reorderDefinitions(projectId, moveField(sorted.map((item) => item.id), index, index - 1))}
              >
                {t("fields.up")}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={index === sorted.length - 1}
                onClick={() => void reorderDefinitions(projectId, moveField(sorted.map((item) => item.id), index, index + 1))}
              >
                {t("fields.down")}
              </Button>
              <Button type="button" size="sm" variant="destructive" onClick={() => void deleteDefinition(projectId, field.id)}>
                {t("fields.delete")}
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
