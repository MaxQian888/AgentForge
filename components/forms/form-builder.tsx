"use client";

import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useFormStore, type FormDefinition } from "@/lib/stores/form-store";
import {
  useCustomFieldStore,
  type CustomFieldDefinition,
} from "@/lib/stores/custom-field-store";
import { buildFormHref } from "@/lib/route-hrefs";

type BuilderField = {
  key: string;
  label: string;
  target: string;
};

const BUILT_IN_TARGETS = [
  { value: "title", label: "Task title" },
  { value: "description", label: "Task description" },
  { value: "priority", label: "Task priority" },
];
const EMPTY_FORMS: FormDefinition[] = [];
const EMPTY_CUSTOM_FIELDS: CustomFieldDefinition[] = [];

export function FormBuilder({ projectId }: { projectId: string }) {
  const formsByProject = useFormStore((state) => state.formsByProject);
  const fetchForms = useFormStore((state) => state.fetchForms);
  const createForm = useFormStore((state) => state.createForm);
  const deleteForm = useFormStore((state) => state.deleteForm);
  const definitionsByProject = useCustomFieldStore((state) => state.definitionsByProject);
  const fetchDefinitions = useCustomFieldStore((state) => state.fetchDefinitions);

  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [targetStatus, setTargetStatus] = useState("inbox");
  const [isPublic, setIsPublic] = useState(true);
  const [fields, setFields] = useState<BuilderField[]>([
    { key: "title", label: "Title", target: "title" },
  ]);

  const forms = useMemo(
    () => formsByProject[projectId] ?? EMPTY_FORMS,
    [formsByProject, projectId]
  );
  const customFields = useMemo(
    () => definitionsByProject[projectId] ?? EMPTY_CUSTOM_FIELDS,
    [definitionsByProject, projectId]
  );

  useEffect(() => {
    void fetchForms(projectId);
    void fetchDefinitions(projectId);
  }, [fetchDefinitions, fetchForms, projectId]);

  const targetOptions = useMemo(
    () => [
      ...BUILT_IN_TARGETS,
      ...customFields.map((field) => ({
        value: `cf:${field.id}`,
        label: `Custom field: ${field.name}`,
      })),
    ],
    [customFields]
  );

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-2">
        <div className="space-y-2">
          <Label>Form name</Label>
          <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="Bug Report" />
        </div>
        <div className="space-y-2">
          <Label>Slug</Label>
          <Input value={slug} onChange={(event) => setSlug(event.target.value)} placeholder="bug-report" />
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        <div className="space-y-2">
          <Label>Target status</Label>
          <select className="h-10 w-full rounded-md border bg-background px-3 text-sm" value={targetStatus} onChange={(event) => setTargetStatus(event.target.value)}>
            {["inbox", "triaged", "assigned", "in_progress", "in_review"].map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </div>
        <label className="flex items-center gap-2 text-sm md:self-end">
          <input type="checkbox" checked={isPublic} onChange={(event) => setIsPublic(event.target.checked)} />
          Public form
        </label>
      </div>

      <div className="space-y-3">
        <div className="font-medium text-sm">Field mappings</div>
        {fields.map((field, index) => (
          <div key={`${field.key}-${index}`} className="grid gap-3 rounded-md border p-3 md:grid-cols-3">
            <Input
              value={field.key}
              onChange={(event) =>
                setFields((current) => current.map((item, itemIndex) => (itemIndex === index ? { ...item, key: event.target.value } : item)))
              }
              placeholder="field key"
            />
            <Input
              value={field.label}
              onChange={(event) =>
                setFields((current) => current.map((item, itemIndex) => (itemIndex === index ? { ...item, label: event.target.value } : item)))
              }
              placeholder="field label"
            />
            <select
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={field.target}
              onChange={(event) =>
                setFields((current) => current.map((item, itemIndex) => (itemIndex === index ? { ...item, target: event.target.value } : item)))
              }
            >
              {targetOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        ))}
        <Button
          type="button"
          variant="outline"
          onClick={() => setFields((current) => [...current, { key: `field_${current.length + 1}`, label: "New Field", target: "description" }])}
        >
          Add field mapping
        </Button>
      </div>

      <Button
        type="button"
        disabled={!name.trim() || !slug.trim()}
        onClick={async () => {
          await createForm(projectId, {
            name,
            slug,
            fields,
            targetStatus,
            isPublic,
          });
          setName("");
          setSlug("");
          setTargetStatus("inbox");
          setIsPublic(true);
          setFields([{ key: "title", label: "Title", target: "title" }]);
        }}
      >
        Create form
      </Button>

      <div className="space-y-2">
        {forms.map((form) => (
          <div key={form.id} className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
            <div>
              <div className="font-medium">{form.name}</div>
              <div className="text-muted-foreground">{buildFormHref(form.slug)}</div>
            </div>
            <Button type="button" size="sm" variant="destructive" onClick={() => void deleteForm(projectId, form.id)}>
              Delete
            </Button>
          </div>
        ))}
      </div>
    </div>
  );
}
