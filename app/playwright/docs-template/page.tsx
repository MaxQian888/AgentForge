"use client"

import { useMemo, useState } from "react"
import { TemplateCenter } from "@/components/docs/template-center"
import { TemplatePicker } from "@/components/docs/template-picker"
import type { DocsPage } from "@/lib/stores/docs-store"

type CreatedDocument = {
  id: string
  title: string
  parentId: string | null
  templateId: string
}

function createTemplate(overrides: Partial<DocsPage> = {}): DocsPage {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    spaceId: "playwright-space",
    parentId: null,
    title: overrides.title ?? "Incident Runbook",
    content: overrides.content ?? "[]",
    contentText: overrides.contentText ?? "Operational checklist",
    path: overrides.path ?? `/templates/${overrides.id ?? "incident-runbook"}`,
    sortOrder: overrides.sortOrder ?? 0,
    isTemplate: true,
    templateCategory: overrides.templateCategory ?? "runbook",
    isSystem: overrides.isSystem ?? true,
    isPinned: false,
    createdBy: "playwright-user",
    updatedBy: "playwright-user",
    createdAt: "2026-04-15T00:00:00.000Z",
    updatedAt: "2026-04-15T00:00:00.000Z",
    deletedAt: null,
    templateSource: overrides.templateSource ?? (overrides.isSystem === false ? "custom" : "system"),
    previewSnippet: overrides.previewSnippet ?? "Operational checklist",
    canEdit: overrides.canEdit ?? !overrides.isSystem,
    canDelete: overrides.canDelete ?? !overrides.isSystem,
    canDuplicate: overrides.canDuplicate ?? true,
    canUse: overrides.canUse ?? true,
  }
}

export default function DocsTemplatePlaywrightPage() {
  const [templates, setTemplates] = useState<DocsPage[]>([
    createTemplate({
      id: "template-system",
      title: "Incident Runbook",
      templateCategory: "runbook",
      isSystem: true,
      templateSource: "system",
    }),
  ])
  const [documents, setDocuments] = useState<CreatedDocument[]>([])
  const [pickerOpen, setPickerOpen] = useState(false)
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null)
  const destinations = useMemo(
    () => [
      { id: null, title: "Workspace Root" },
      { id: "ops-folder", title: "Operations" },
    ],
    [],
  )

  return (
    <section className="grid gap-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
      <TemplateCenter
        templates={templates}
        onCreateFromTemplate={(templateId) => {
          setSelectedTemplateId(templateId)
          setPickerOpen(true)
        }}
        onCreateTemplate={({ title, category }) => {
          setTemplates((current) => [
            createTemplate({
              id: title.toLowerCase().replace(/\s+/g, "-"),
              title,
              templateCategory: category,
              isSystem: false,
              templateSource: "custom",
              canEdit: true,
              canDelete: true,
            }),
            ...current,
          ])
        }}
        onEditTemplate={() => undefined}
        onDuplicateTemplate={({ templateId, name, category }) => {
          const source = templates.find((template) => template.id === templateId)
          if (!source) return
          setTemplates((current) => [
            createTemplate({
              ...source,
              id: `${templateId}-copy`,
              title: name,
              templateCategory: category,
              isSystem: false,
              templateSource: "custom",
              canEdit: true,
              canDelete: true,
            }),
            ...current,
          ])
        }}
        onDeleteTemplate={(templateId) => {
          setTemplates((current) => current.filter((template) => template.id !== templateId))
        }}
      />

      <section className="rounded-xl border border-border/60 bg-card/70 p-4">
        <h2 className="text-base font-semibold">Created Documents</h2>
        <ul className="mt-3 space-y-2" data-testid="created-documents">
          {documents.map((document) => (
            <li
              key={document.id}
              className="rounded-lg border border-border/60 px-3 py-2 text-sm"
            >
              {document.title} · {document.parentId ?? "root"} · {document.templateId}
            </li>
          ))}
          {documents.length === 0 ? (
            <li className="text-sm text-muted-foreground">No created documents yet.</li>
          ) : null}
        </ul>
      </section>

      <TemplatePicker
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        templates={templates}
        destinations={destinations}
        initialTemplateId={selectedTemplateId}
        defaultTitle="New document from template"
        onPick={({ templateId, title, parentId }) => {
          setDocuments((current) => [
            {
              id: crypto.randomUUID(),
              title,
              parentId: parentId ?? null,
              templateId,
            },
            ...current,
          ])
          setPickerOpen(false)
          setSelectedTemplateId(null)
        }}
      />
    </section>
  )
}
