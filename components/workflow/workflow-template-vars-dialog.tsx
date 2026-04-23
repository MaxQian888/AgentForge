"use client"

import type { FormEvent } from "react"
import { useTranslations } from "next-intl"
import { Loader2 } from "lucide-react"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { WorkflowDefinition } from "@/lib/stores/workflow-store"

interface TemplateVarsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  template: WorkflowDefinition
  mode: "clone" | "execute"
  onSubmit: (overrides: Record<string, unknown>, taskId?: string) => Promise<void>
  loading: boolean
}

export function WorkflowTemplateVarsDialog({
  open,
  onOpenChange,
  template,
  mode,
  onSubmit,
  loading,
}: TemplateVarsDialogProps) {
  const t = useTranslations("workflow")
  const title = mode === "clone"
    ? t("templateDialog.cloneTitle", { name: template.name })
    : t("templateDialog.executeTitle", { name: template.name })
  const description = mode === "clone"
    ? t("templateDialog.cloneDesc")
    : t("templateDialog.executeDesc")

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)

    const overrides: Record<string, unknown> = {}
    for (const key of Object.keys(template.templateVars ?? {})) {
      const raw = formData.get(key) as string | null
      if (raw !== null) {
        try {
          overrides[key] = JSON.parse(raw)
        } catch {
          overrides[key] = raw
        }
      }
    }

    const rawTaskId = formData.get("taskId") as string | null
    const taskId = rawTaskId?.trim() || undefined

    await onSubmit(overrides, taskId)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <form id="template-vars-form" onSubmit={handleSubmit}>
          <div className="grid gap-4 py-4">
            {Object.entries(template.templateVars ?? {}).map(([key, value]) => (
              <div key={key} className="grid gap-1.5">
                <Label htmlFor={`var-${key}`}>{key}</Label>
                <Input
                  id={`var-${key}`}
                  name={key}
                  defaultValue={String(value ?? "")}
                />
              </div>
            ))}

            {mode === "execute" && (
              <div className="grid gap-1.5">
                <Label htmlFor="var-taskId">{t("templateDialog.taskIdLabel")}</Label>
                <Input
                  id="var-taskId"
                  name="taskId"
                  placeholder={t("templateDialog.taskIdPlaceholder")}
                />
              </div>
            )}
          </div>
        </form>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={loading}
          >
            {t("templateDialog.cancel")}
          </Button>
          <Button type="submit" form="template-vars-form" disabled={loading}>
            {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {mode === "clone" ? t("templateDialog.createCopy") : t("templateDialog.startExecution")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
