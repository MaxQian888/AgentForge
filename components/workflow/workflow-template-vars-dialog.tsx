"use client"

import type { FormEvent } from "react"
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
  const title =
    (mode === "clone" ? "Create Workflow Copy: " : "Start Template Execution: ") + template.name
  const description =
    mode === "clone"
      ? "This creates a project-owned workflow definition you can continue editing safely."
      : "This starts an execution by cloning the template into your project with the variables below."

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
                <Label htmlFor="var-taskId">Task ID (optional)</Label>
                <Input
                  id="var-taskId"
                  name="taskId"
                  placeholder="Task ID (optional)"
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
            Cancel
          </Button>
          <Button type="submit" form="template-vars-form" disabled={loading}>
            {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {mode === "clone" ? "Create Workflow Copy" : "Start Execution"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
