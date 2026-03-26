"use client";

import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
          <Label>Field Name</Label>
          <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="Priority" />
        </div>
        <div className="space-y-2">
          <Label>Type</Label>
          <select
            className="h-10 w-full rounded-md border bg-background px-3 text-sm"
            value={fieldType}
            onChange={(event) => setFieldType(event.target.value)}
          >
            {FIELD_TYPES.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-2">
          <Label>Required</Label>
          <label className="flex h-10 items-center gap-2 rounded-md border px-3 text-sm">
            <input type="checkbox" checked={required} onChange={(event) => setRequired(event.target.checked)} />
            Required
          </label>
        </div>
      </div>

      {(fieldType === "select" || fieldType === "multi_select") && (
        <div className="space-y-2">
          <Label>Options</Label>
          <Input value={options} onChange={(event) => setOptions(event.target.value)} placeholder="P0, P1, P2" />
        </div>
      )}

      <Button type="button" onClick={() => void handleCreate()} disabled={!name.trim()}>
        Add field
      </Button>

      <div className="space-y-2">
        {sorted.map((field, index) => (
          <div key={field.id} className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
            <div>
              <div className="font-medium">{field.name}</div>
              <div className="text-muted-foreground">
                {field.fieldType}
                {field.required ? " · required" : ""}
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
                Up
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={index === sorted.length - 1}
                onClick={() => void reorderDefinitions(projectId, moveField(sorted.map((item) => item.id), index, index + 1))}
              >
                Down
              </Button>
              <Button type="button" size="sm" variant="destructive" onClick={() => void deleteDefinition(projectId, field.id)}>
                Delete
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
