"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { RoleManifest } from "@/lib/stores/role-store";
import type {
  RoleDraft,
  RoleKnowledgeSourceDraft,
  RoleSkillDraft,
  RoleTriggerDraft,
} from "@/lib/roles/role-management";
import {
  ROLE_WORKSPACE_SECTIONS,
  type RoleWorkspaceSectionId,
} from "./role-workspace-sections";

type UpdateDraftField = <K extends keyof RoleDraft>(
  key: K,
  value: RoleDraft[K],
) => void;

interface RoleWorkspaceEditorProps {
  mode: "create" | "edit";
  draft: RoleDraft;
  templateId: string;
  selectedRole: RoleManifest | undefined;
  selectedTemplateName: string | null;
  selectedParentName: string | null;
  validationErrors: string[];
  saving: boolean;
  activeSection: RoleWorkspaceSectionId;
  onSelectSection: (section: RoleWorkspaceSectionId) => void;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
  onSwitchToCreate: () => void;
  updateDraft: UpdateDraftField;
  updateSkillRow: (
    index: number,
    field: keyof RoleSkillDraft,
    value: RoleSkillDraft[keyof RoleSkillDraft],
  ) => void;
  updateKnowledgeRow: (
    index: number,
    field: keyof RoleKnowledgeSourceDraft,
    value: RoleKnowledgeSourceDraft[keyof RoleKnowledgeSourceDraft],
  ) => void;
  updateTriggerRow: (
    index: number,
    field: keyof RoleTriggerDraft,
    value: RoleTriggerDraft[keyof RoleTriggerDraft],
  ) => void;
  onAddSkillRow: () => void;
  onAddKnowledgeRow: () => void;
  onAddTriggerRow: () => void;
  availableRoles: RoleManifest[];
  onTemplateChange: (value: string) => void;
}

function TextAreaField({
  id,
  label,
  value,
  onChange,
  rows = 3,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  rows?: number;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <textarea
        id={id}
        className="min-h-24 rounded-md border bg-background px-3 py-2 text-sm"
        value={value}
        rows={rows}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

function AuthoringSection({
  sectionId,
  title,
  description,
  activeSection,
  onSelectSection,
  children,
}: {
  sectionId: RoleWorkspaceSectionId;
  title: string;
  description?: string;
  activeSection: RoleWorkspaceSectionId;
  onSelectSection: (section: RoleWorkspaceSectionId) => void;
  children: React.ReactNode;
}) {
  const isActive = activeSection === sectionId;

  return (
    <section
      className={`grid gap-4 rounded-lg border p-4 transition-colors ${
        isActive ? "border-primary/60 bg-primary/5" : "bg-background"
      }`}
      onFocusCapture={() => onSelectSection(sectionId)}
    >
      <div className="space-y-1">
        <h3 className="text-sm font-semibold">{title}</h3>
        {description ? (
          <p className="text-sm text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {children}
    </section>
  );
}

export function RoleWorkspaceEditor({
  mode,
  draft,
  templateId,
  selectedRole,
  selectedTemplateName,
  selectedParentName,
  validationErrors,
  saving,
  activeSection,
  onSelectSection,
  onSubmit,
  onSwitchToCreate,
  updateDraft,
  updateSkillRow,
  updateKnowledgeRow,
  updateTriggerRow,
  onAddSkillRow,
  onAddKnowledgeRow,
  onAddTriggerRow,
  availableRoles,
  onTemplateChange,
}: RoleWorkspaceEditorProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{mode === "edit" ? "Role Workspace" : "Create Role"}</CardTitle>
        <CardDescription>
          Work through setup, identity, capabilities, knowledge, governance, and review in one structured authoring flow.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="flex flex-col gap-4" onSubmit={onSubmit}>
          <div className="rounded-lg border bg-muted/20 p-4">
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              Current flow
            </p>
            <div className="mt-2 grid gap-2">
              <p className="text-sm font-medium">
                {mode === "edit" ? "Editing existing role" : "Creating new role"}
              </p>
              <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                {selectedTemplateName ? (
                  <span className="rounded-full border px-2 py-1">
                    Template source: {selectedTemplateName}
                  </span>
                ) : null}
                {selectedParentName ? (
                  <span className="rounded-full border px-2 py-1">
                    Inherited from {selectedParentName}
                  </span>
                ) : (
                  <span className="rounded-full border px-2 py-1">No inherited parent</span>
                )}
                <span className="rounded-full border px-2 py-1">
                  {selectedRole ? `Editing ${selectedRole.metadata.id}` : "Unsaved draft"}
                </span>
              </div>
            </div>
          </div>

          <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-6">
            {ROLE_WORKSPACE_SECTIONS.map((section) => {
              const isActive = section.id === activeSection;
              return (
                <Button
                  key={section.id}
                  type="button"
                  variant={isActive ? "default" : "outline"}
                  className="justify-start"
                  onClick={() => onSelectSection(section.id)}
                >
                  {section.label}
                </Button>
              );
            })}
          </div>

          <AuthoringSection
            sectionId="setup"
            title="Setup"
            description="Start from a blank role, a template copy, or a child role with inheritance."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-template">Start from template</Label>
                <select
                  id="role-template"
                  aria-label="Start from template"
                  className="h-10 rounded-md border bg-background px-3 text-sm"
                  value={templateId}
                  onChange={(event) => onTemplateChange(event.target.value)}
                  disabled={mode === "edit"}
                >
                  <option value="">Blank role</option>
                  {availableRoles.map((role) => (
                    <option key={role.metadata.id} value={role.metadata.id}>
                      {role.metadata.name}
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-extends">Inherits from</Label>
                <select
                  id="role-extends"
                  aria-label="Inherits from"
                  className="h-10 rounded-md border bg-background px-3 text-sm"
                  value={draft.extendsValue}
                  onChange={(event) => updateDraft("extendsValue", event.target.value)}
                >
                  <option value="">No parent</option>
                  {availableRoles.map((role) => (
                    <option key={role.metadata.id} value={role.metadata.id}>
                      {role.metadata.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-id">Role ID</Label>
                <Input
                  id="role-id"
                  aria-label="Role ID"
                  value={draft.roleId}
                  onChange={(event) => updateDraft("roleId", event.target.value)}
                  disabled={mode === "edit"}
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-name">Name</Label>
                <Input
                  id="role-name"
                  value={draft.name}
                  onChange={(event) => updateDraft("name", event.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-version">Version</Label>
                <Input
                  id="role-version"
                  value={draft.version}
                  onChange={(event) => updateDraft("version", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-tags">Tags</Label>
                <Input
                  id="role-tags"
                  value={draft.tagsInput}
                  onChange={(event) => updateDraft("tagsInput", event.target.value)}
                />
              </div>
            </div>
            <TextAreaField
              id="role-description"
              label="Description"
              value={draft.description}
              onChange={(value) => updateDraft("description", value)}
            />
          </AuthoringSection>

          <AuthoringSection
            sectionId="identity"
            title="Identity"
            description="Define what this role is, what it aims to do, and how it communicates."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="identity-role">Role Title</Label>
                <Input
                  id="identity-role"
                  value={draft.identityRole}
                  onChange={(event) => updateDraft("identityRole", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="identity-goal">Goal</Label>
                <Input
                  id="identity-goal"
                  value={draft.goal}
                  onChange={(event) => updateDraft("goal", event.target.value)}
                />
              </div>
            </div>
            <TextAreaField
              id="identity-backstory"
              label="Backstory"
              value={draft.backstory}
              onChange={(value) => updateDraft("backstory", value)}
            />
            <TextAreaField
              id="identity-prompt"
              label="System Prompt"
              value={draft.systemPrompt}
              onChange={(value) => updateDraft("systemPrompt", value)}
              rows={5}
            />

            <div className="grid gap-4 rounded-lg border p-4">
              <h3 className="text-sm font-semibold">Advanced Identity</h3>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-persona">Persona</Label>
                  <Input
                    id="identity-persona"
                    value={draft.persona}
                    onChange={(event) => updateDraft("persona", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-personality">Personality</Label>
                  <Input
                    id="identity-personality"
                    value={draft.personality}
                    onChange={(event) => updateDraft("personality", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-language">Language</Label>
                  <Input
                    id="identity-language"
                    value={draft.language}
                    onChange={(event) => updateDraft("language", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-tone">Response Tone</Label>
                  <Input
                    id="identity-tone"
                    value={draft.responseTone}
                    onChange={(event) => updateDraft("responseTone", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-verbosity">Response Verbosity</Label>
                  <Input
                    id="identity-verbosity"
                    value={draft.responseVerbosity}
                    onChange={(event) => updateDraft("responseVerbosity", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-format-preference">Format Preference</Label>
                  <Input
                    id="identity-format-preference"
                    value={draft.responseFormatPreference}
                    onChange={(event) =>
                      updateDraft("responseFormatPreference", event.target.value)
                    }
                  />
                </div>
              </div>
            </div>
          </AuthoringSection>

          <AuthoringSection
            sectionId="capabilities"
            title="Capabilities"
            description="Declare tools, packages, skills, and other execution constraints."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-packages">Packages</Label>
                <Input
                  id="cap-packages"
                  value={draft.packages}
                  onChange={(event) => updateDraft("packages", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-tools">Allowed Tools</Label>
                <Input
                  id="cap-tools"
                  aria-label="Allowed Tools"
                  value={draft.allowedTools}
                  onChange={(event) => updateDraft("allowedTools", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-external-tools">External Tools</Label>
                <Input
                  id="cap-external-tools"
                  value={draft.externalTools}
                  onChange={(event) => updateDraft("externalTools", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-languages">Languages</Label>
                <Input
                  id="cap-languages"
                  value={draft.languages}
                  onChange={(event) => updateDraft("languages", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-frameworks">Frameworks</Label>
                <Input
                  id="cap-frameworks"
                  value={draft.frameworks}
                  onChange={(event) => updateDraft("frameworks", event.target.value)}
                />
              </div>
            </div>

            <div className="grid gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-semibold">Skills</h3>
                  <p className="text-sm text-muted-foreground">
                    Declare reusable skill references and whether they should auto-load for this role.
                  </p>
                </div>
                <Button type="button" variant="outline" size="sm" onClick={onAddSkillRow}>
                  Add Skill
                </Button>
              </div>
              <div className="grid gap-4">
                {draft.skillRows.length > 0 ? (
                  draft.skillRows.map((skill, index) => (
                    <div
                      key={`skill-row-${index}`}
                      className="grid gap-3 rounded-md border p-3 md:grid-cols-[minmax(0,1fr)_auto]"
                    >
                      <div className="flex flex-col gap-1.5">
                        <Label htmlFor={`skill-path-${index}`}>Skill Path</Label>
                        <Input
                          id={`skill-path-${index}`}
                          aria-label="Skill Path"
                          value={skill.path}
                          onChange={(event) =>
                            updateSkillRow(index, "path", event.target.value)
                          }
                          placeholder="skills/react"
                        />
                      </div>
                      <div className="flex items-center gap-2 pt-7">
                        <input
                          id={`skill-auto-load-${index}`}
                          type="checkbox"
                          checked={skill.autoLoad}
                          onChange={(event) =>
                            updateSkillRow(index, "autoLoad", event.target.checked)
                          }
                        />
                        <Label htmlFor={`skill-auto-load-${index}`}>Auto-load skill</Label>
                      </div>
                    </div>
                  ))
                ) : (
                  <p className="text-sm text-muted-foreground">
                    No skills configured for this role yet.
                  </p>
                )}
              </div>
            </div>

            {validationErrors.length > 0 ? (
              <div className="grid gap-1">
                {validationErrors.map((errorMessage) => (
                  <p key={errorMessage} className="text-sm text-destructive">
                    {errorMessage}
                  </p>
                ))}
              </div>
            ) : null}
          </AuthoringSection>

          <AuthoringSection
            sectionId="knowledge"
            title="Knowledge"
            description="Attach the repositories, documents, patterns, and shared sources this role relies on."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <div className="grid gap-4 md:grid-cols-3">
              <TextAreaField
                id="knowledge-repositories"
                label="Repositories"
                value={draft.repositories}
                onChange={(value) => updateDraft("repositories", value)}
              />
              <TextAreaField
                id="knowledge-documents"
                label="Documents"
                value={draft.documents}
                onChange={(value) => updateDraft("documents", value)}
              />
              <TextAreaField
                id="knowledge-patterns"
                label="Patterns"
                value={draft.patterns}
                onChange={(value) => updateDraft("patterns", value)}
              />
            </div>
            <div className="grid gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-muted-foreground">
                  Shared knowledge sources that the role can cite or reuse.
                </p>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={onAddKnowledgeRow}
                >
                  Add Shared Knowledge
                </Button>
              </div>
              {draft.sharedKnowledgeRows.map((source, index) => (
                <div
                  key={`knowledge-${index}`}
                  className="grid gap-3 rounded-md border p-3 md:grid-cols-2"
                >
                  <Input
                    aria-label="Shared Knowledge ID"
                    value={source.id}
                    placeholder="design-guidelines"
                    onChange={(event) =>
                      updateKnowledgeRow(index, "id", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Shared Knowledge Type"
                    value={source.type}
                    placeholder="vector"
                    onChange={(event) =>
                      updateKnowledgeRow(index, "type", event.target.value)
                    }
                  />
                </div>
              ))}
            </div>
          </AuthoringSection>

          <AuthoringSection
            sectionId="governance"
            title="Governance"
            description="Keep safety, collaboration, and activation cues visible before execution."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <div className="grid gap-4 rounded-lg border p-4">
              <h3 className="text-sm font-semibold">Security</h3>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="security-profile">Security Profile</Label>
                  <Input
                    id="security-profile"
                    value={draft.securityProfile}
                    onChange={(event) => updateDraft("securityProfile", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="security-permission">Permission Mode</Label>
                  <Input
                    id="security-permission"
                    aria-label="Permission Mode"
                    value={draft.permissionMode}
                    onChange={(event) => updateDraft("permissionMode", event.target.value)}
                  />
                </div>
              </div>
              <TextAreaField
                id="security-output-filters"
                label="Output Filters"
                value={draft.outputFilters}
                onChange={(value) => updateDraft("outputFilters", value)}
              />
              <div className="grid gap-4 md:grid-cols-2">
                <TextAreaField
                  id="security-allowed"
                  label="Allowed Paths"
                  value={draft.allowedPaths}
                  onChange={(value) => updateDraft("allowedPaths", value)}
                />
                <TextAreaField
                  id="security-denied"
                  label="Denied Paths"
                  value={draft.deniedPaths}
                  onChange={(value) => updateDraft("deniedPaths", value)}
                />
              </div>
              <div className="flex items-center gap-2">
                <input
                  id="security-review"
                  type="checkbox"
                  checked={draft.requireReview}
                  onChange={(event) => updateDraft("requireReview", event.target.checked)}
                />
                <Label htmlFor="security-review">Require review before execution</Label>
              </div>
            </div>

            <div className="grid gap-4 rounded-lg border p-4">
              <h3 className="text-sm font-semibold">Collaboration</h3>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="collaboration-delegate">Can Delegate To</Label>
                  <Input
                    id="collaboration-delegate"
                    value={draft.collaborationCanDelegateTo}
                    onChange={(event) =>
                      updateDraft("collaborationCanDelegateTo", event.target.value)
                    }
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="collaboration-accepts">Accepts Delegation From</Label>
                  <Input
                    id="collaboration-accepts"
                    value={draft.collaborationAcceptsDelegationFrom}
                    onChange={(event) =>
                      updateDraft(
                        "collaborationAcceptsDelegationFrom",
                        event.target.value,
                      )
                    }
                  />
                </div>
              </div>
            </div>

            <div className="grid gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-semibold">Triggers</h3>
                  <p className="text-sm text-muted-foreground">
                    Define lightweight activation cues for this role.
                  </p>
                </div>
                <Button type="button" variant="outline" size="sm" onClick={onAddTriggerRow}>
                  Add Trigger
                </Button>
              </div>
              {draft.triggerRows.map((trigger, index) => (
                <div
                  key={`trigger-${index}`}
                  className="grid gap-3 rounded-md border p-3 md:grid-cols-3"
                >
                  <Input
                    aria-label="Trigger Event"
                    value={trigger.event}
                    placeholder="pr_created"
                    onChange={(event) =>
                      updateTriggerRow(index, "event", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Trigger Action"
                    value={trigger.action}
                    placeholder="auto_review"
                    onChange={(event) =>
                      updateTriggerRow(index, "action", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Trigger Condition"
                    value={trigger.condition}
                    placeholder="labels.includes('ui')"
                    onChange={(event) =>
                      updateTriggerRow(index, "condition", event.target.value)
                    }
                  />
                </div>
              ))}
            </div>
          </AuthoringSection>

          <AuthoringSection
            sectionId="review"
            title="Review"
            description="Use the execution summary, YAML preview, preview, and sandbox surfaces before saving."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <p className="text-sm text-muted-foreground">
              Review the right-hand context surfaces before saving. They stay available in desktop rails and can be reopened from compact review panels on smaller viewports.
            </p>
          </AuthoringSection>

          <div className="flex flex-wrap items-center gap-3 border-t pt-4">
            <Button type="submit" disabled={saving || !draft.roleId || !draft.name}>
              {saving ? "Saving..." : "Save Role"}
            </Button>
            {mode === "edit" && selectedRole ? (
              <Button type="button" variant="outline" onClick={onSwitchToCreate}>
                Switch to Create
              </Button>
            ) : null}
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
