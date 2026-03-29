"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { FormDefinition } from "@/lib/stores/form-store";
import { useFormStore } from "@/lib/stores/form-store";

export function FormRenderer({ form }: { form: FormDefinition }) {
  const t = useTranslations("forms");
  const submitForm = useFormStore((state) => state.submitForm);
  const fields = useMemo(() => (Array.isArray(form.fields) ? form.fields : []), [form.fields]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [submittedTaskId, setSubmittedTaskId] = useState<string | null>(null);

  return (
    <div className="space-y-4">
      {fields.map((field) => {
        const item = field as { key?: string; label?: string };
        const key = item.key ?? "";
        return (
          <div key={key} className="space-y-2">
            <Label>{item.label ?? key}</Label>
            <Input
              value={values[key] ?? ""}
              onChange={(event) => setValues((current) => ({ ...current, [key]: event.target.value }))}
            />
          </div>
        );
      })}
      <Button
        type="button"
        onClick={async () => {
          const task = await submitForm(form.slug, { values });
          setSubmittedTaskId(task.id);
        }}
      >
        {t("submitForm")}
      </Button>
      {submittedTaskId ? <div className="text-sm text-muted-foreground">{t("createdTask", { id: submittedTaskId })}</div> : null}
    </div>
  );
}
