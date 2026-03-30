"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type {
  RoleManifest,
  RoleSkillCatalogEntry,
} from "@/lib/stores/role-store";
import type {
  FieldProvenanceMap,
  RoleDraft,
  RoleDraftValidationBySection,
  RoleKeyValueDraft,
  RoleKnowledgeSourceDraft,
  RoleMCPServerDraft,
  RoleSkillDraft,
  RoleSkillResolution,
  RoleTriggerDraft,
} from "@/lib/roles/role-management";
import { ProvenanceBadge } from "./provenance-badge";
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
  skillCatalog: RoleSkillCatalogEntry[];
  skillCatalogLoading: boolean;
  draftSkillResolution: RoleSkillResolution[];
  selectedTemplateName: string | null;
  selectedParentName: string | null;
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
  updateMCPServerRow: (
    index: number,
    field: keyof RoleMCPServerDraft,
    value: RoleMCPServerDraft[keyof RoleMCPServerDraft],
  ) => void;
  updateCustomSettingRow: (
    index: number,
    field: keyof RoleKeyValueDraft,
    value: RoleKeyValueDraft[keyof RoleKeyValueDraft],
  ) => void;
  updateKnowledgeRow: (
    index: number,
    field: keyof RoleKnowledgeSourceDraft,
    value: RoleKnowledgeSourceDraft[keyof RoleKnowledgeSourceDraft],
  ) => void;
  updatePrivateKnowledgeRow: (
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
  onAddMCPServerRow: () => void;
  onAddCustomSettingRow: () => void;
  onAddKnowledgeRow: () => void;
  onAddPrivateKnowledgeRow: () => void;
  onAddTriggerRow: () => void;
  availableRoles: RoleManifest[];
  onTemplateChange: (value: string) => void;
  validationBySection: RoleDraftValidationBySection;
  provenanceMap?: FieldProvenanceMap;
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

function SectionErrors({ errors }: { errors: string[] }) {
  if (errors.length === 0) {
    return null;
  }

  return (
    <div className="grid gap-1">
      {errors.map((errorMessage) => (
        <p key={errorMessage} className="text-sm text-destructive">
          {errorMessage}
        </p>
      ))}
    </div>
  );
}

function AuthoringSection({
  sectionId,
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
  if (sectionId !== activeSection) return null;
  return (
    <section
      className="grid gap-4 px-4 py-4"
      onFocusCapture={() => onSelectSection(sectionId)}
    >
      {children}
    </section>
  );
}

export function RoleWorkspaceEditor({
  mode,
  draft,
  templateId,
  selectedRole,
  skillCatalog,
  skillCatalogLoading,
  draftSkillResolution,
  selectedTemplateName,
  selectedParentName,
  saving,
  activeSection,
  onSelectSection,
  onSubmit,
  onSwitchToCreate,
  updateDraft,
  updateSkillRow,
  updateMCPServerRow,
  updateCustomSettingRow,
  updateKnowledgeRow,
  updatePrivateKnowledgeRow,
  updateTriggerRow,
  onAddSkillRow,
  onAddMCPServerRow,
  onAddCustomSettingRow,
  onAddKnowledgeRow,
  onAddPrivateKnowledgeRow,
  onAddTriggerRow,
  availableRoles,
  onTemplateChange,
  validationBySection,
  provenanceMap,
}: RoleWorkspaceEditorProps) {
  const t = useTranslations("roles");
  return (
    <div className="flex flex-col">
      {/* Column header */}
      <div className="sticky top-0 z-10 border-b bg-card px-4 py-3">
        <p className="text-sm font-semibold">
          {mode === "edit" ? t("workspace.roleWorkspace") : t("workspace.createRole")}
        </p>
        <div className="mt-1 flex flex-wrap items-center gap-1.5">
          <span className="text-xs text-muted-foreground">
            {mode === "edit" ? t("workspace.editingExisting") : t("workspace.creatingNew")}
          </span>
          {selectedTemplateName ? (
            <Badge variant="outline" className="text-xs">
              {t("workspace.templateSource", { name: selectedTemplateName })}
            </Badge>
          ) : null}
          {selectedParentName ? (
            <Badge variant="outline" className="text-xs">
              {t("workspace.inheritedFrom", { name: selectedParentName })}
            </Badge>
          ) : (
            <Badge variant="outline" className="text-xs">{t("workspace.noParent")}</Badge>
          )}
          <Badge variant="outline" className="text-xs">
            {selectedRole ? t("workspace.editing", { id: selectedRole.metadata.id }) : t("workspace.unsavedDraft")}
          </Badge>
        </div>
      </div>

      {/* Section tab navigation */}
      <div className="flex overflow-x-auto border-b">
        {ROLE_WORKSPACE_SECTIONS.map((section) => {
          const isActive = section.id === activeSection;
          const hasErrors = (validationBySection[section.id as keyof typeof validationBySection] ?? []).length > 0;
          return (
            <button
              key={section.id}
              type="button"
              onClick={() => onSelectSection(section.id)}
              className={`shrink-0 border-b-2 px-4 py-2.5 text-xs font-medium transition-colors ${
                isActive
                  ? "border-primary text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              }`}
            >
              {section.label}
              {hasErrors ? (
                <span className="ml-1.5 inline-block size-1.5 rounded-full bg-destructive align-middle" />
              ) : null}
            </button>
          );
        })}
      </div>

      <form className="flex flex-col" onSubmit={onSubmit}>

          <AuthoringSection
            sectionId="setup"
            title="Setup"
            description="Start from a blank role, a template copy, or a child role with inheritance."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-template">{t("formDialog.startFromTemplate")}</Label>
                <select
                  id="role-template"
                  aria-label="Start from template"
                  className="h-10 rounded-md border bg-background px-3 text-sm"
                  value={templateId}
                  onChange={(event) => onTemplateChange(event.target.value)}
                  disabled={mode === "edit"}
                >
                  <option value="">{t("formDialog.blankRole")}</option>
                  {availableRoles.map((role) => (
                    <option key={role.metadata.id} value={role.metadata.id}>
                      {role.metadata.name}
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-extends">{t("formDialog.inheritsFrom")}</Label>
                <select
                  id="role-extends"
                  aria-label="Inherits from"
                  className="h-10 rounded-md border bg-background px-3 text-sm"
                  value={draft.extendsValue}
                  onChange={(event) => updateDraft("extendsValue", event.target.value)}
                >
                  <option value="">{t("formDialog.noParent")}</option>
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
                <Label htmlFor="role-id">{t("formDialog.roleId")}</Label>
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
                <Label htmlFor="role-name">{t("formDialog.name")}</Label>
                <Input
                  id="role-name"
                  value={draft.name}
                  onChange={(event) => updateDraft("name", event.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-version">{t("workspace.version")}</Label>
                <Input
                  id="role-version"
                  value={draft.version}
                  onChange={(event) => updateDraft("version", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-tags">{t("formDialog.tags")}</Label>
                <Input
                  id="role-tags"
                  value={draft.tagsInput}
                  onChange={(event) => updateDraft("tagsInput", event.target.value)}
                />
              </div>
            </div>
            <TextAreaField
              id="role-description"
              label={t("formDialog.description")}
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
                <Label htmlFor="identity-role">{t("formDialog.role")}</Label>
                <Input
                  id="identity-role"
                  value={draft.identityRole}
                  onChange={(event) => updateDraft("identityRole", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="identity-goal">{t("formDialog.goal")}</Label>
                <Input
                  id="identity-goal"
                  value={draft.goal}
                  onChange={(event) => updateDraft("goal", event.target.value)}
                />
              </div>
            </div>
            <TextAreaField
              id="identity-backstory"
              label={t("formDialog.backstory")}
              value={draft.backstory}
              onChange={(value) => updateDraft("backstory", value)}
            />
            <TextAreaField
              id="identity-prompt"
              label={t("formDialog.systemPrompt")}
              value={draft.systemPrompt}
              onChange={(value) => updateDraft("systemPrompt", value)}
              rows={5}
            />

            <div className="grid gap-4 rounded-lg border p-4">
              <h3 className="text-sm font-semibold">{t("workspace.advancedIdentity")}</h3>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-persona">{t("workspace.persona")}</Label>
                  <Input
                    id="identity-persona"
                    value={draft.persona}
                    onChange={(event) => updateDraft("persona", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-personality">{t("workspace.personality")}</Label>
                  <Input
                    id="identity-personality"
                    value={draft.personality}
                    onChange={(event) => updateDraft("personality", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-language">{t("workspace.language")}</Label>
                  <Input
                    id="identity-language"
                    value={draft.language}
                    onChange={(event) => updateDraft("language", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-tone">{t("workspace.responseTone")}</Label>
                  <Input
                    id="identity-tone"
                    value={draft.responseTone}
                    onChange={(event) => updateDraft("responseTone", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-verbosity">{t("workspace.responseVerbosity")}</Label>
                  <Input
                    id="identity-verbosity"
                    value={draft.responseVerbosity}
                    onChange={(event) => updateDraft("responseVerbosity", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="identity-format-preference">{t("workspace.formatPreference")}</Label>
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
                <Label htmlFor="cap-packages">{t("workspace.packages")}</Label>
                <Input
                  id="cap-packages"
                  value={draft.packages}
                  onChange={(event) => updateDraft("packages", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-tools">{t("formDialog.allowedTools")}</Label>
                <Input
                  id="cap-tools"
                  aria-label="Allowed Tools"
                  value={draft.allowedTools}
                  onChange={(event) => updateDraft("allowedTools", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-external-tools">{t("workspace.externalTools")}</Label>
                <Input
                  id="cap-external-tools"
                  value={draft.externalTools}
                  onChange={(event) => updateDraft("externalTools", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-languages">{t("formDialog.languages")}</Label>
                <Input
                  id="cap-languages"
                  value={draft.languages}
                  onChange={(event) => updateDraft("languages", event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="cap-frameworks">{t("formDialog.frameworks")}</Label>
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
                  <h3 className="text-sm font-semibold">{t("workspace.advancedCapabilitySettings")}</h3>
                  <p className="text-sm text-muted-foreground">
                    {t("workspace.advancedCapabilitySettingsDesc")}
                  </p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={onAddCustomSettingRow}
                >
                  {t("workspace.addCustomSetting")}
                </Button>
              </div>
              {draft.customSettingRows.length > 0 ? (
                draft.customSettingRows.map((setting, index) => {
                  const prov = provenanceMap?.customSettings.find((e) => e.key === setting.key.trim());
                  return (
                    <div
                      key={`custom-setting-${index}`}
                      className="grid gap-3 rounded-md border p-3 md:grid-cols-2"
                    >
                      <div className="flex items-center gap-2">
                        <Input
                          aria-label="Custom Setting Key"
                          value={setting.key}
                          placeholder="approval_mode"
                          onChange={(event) =>
                            updateCustomSettingRow(index, "key", event.target.value)
                          }
                        />
                        {prov ? <ProvenanceBadge provenance={prov.provenance} /> : null}
                      </div>
                      <Input
                        aria-label="Custom Setting Value"
                        value={setting.value}
                        placeholder="guided"
                        onChange={(event) =>
                          updateCustomSettingRow(index, "value", event.target.value)
                        }
                      />
                    </div>
                  );
                })
              ) : (
                <p className="text-sm text-muted-foreground">
                  {t("workspace.noCustomSettingsYet")}
                </p>
              )}
            </div>

            <div className="grid gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-semibold">{t("workspace.mcpServers")}</h3>
                  <p className="text-sm text-muted-foreground">
                    {t("workspace.mcpServersDesc")}
                  </p>
                </div>
                <Button type="button" variant="outline" size="sm" onClick={onAddMCPServerRow}>
                  {t("workspace.addMcpServer")}
                </Button>
              </div>
              {draft.mcpServerRows.length > 0 ? (
                draft.mcpServerRows.map((server, index) => {
                  const prov = provenanceMap?.mcpServers.find((e) => e.key === server.name.trim());
                  return (
                    <div
                      key={`mcp-server-${index}`}
                      className="grid gap-3 rounded-md border p-3 md:grid-cols-2"
                    >
                      <div className="flex items-center gap-2">
                        <Input
                          aria-label="MCP Server Name"
                          value={server.name}
                          placeholder="design-mcp"
                          onChange={(event) =>
                            updateMCPServerRow(index, "name", event.target.value)
                          }
                        />
                        {prov ? <ProvenanceBadge provenance={prov.provenance} /> : null}
                      </div>
                      <Input
                        aria-label="MCP Server URL"
                        value={server.url}
                        placeholder="http://localhost:3010/mcp"
                        onChange={(event) =>
                          updateMCPServerRow(index, "url", event.target.value)
                        }
                      />
                    </div>
                  );
                })
              ) : (
                <p className="text-sm text-muted-foreground">{t("workspace.noMcpServersYet")}</p>
              )}
            </div>

            <div className="grid gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-semibold">{t("formDialog.skillsTitle")}</h3>
                  <p className="text-sm text-muted-foreground">
                    {t("workspace.availableSkillsDesc")}
                  </p>
                </div>
                <Button type="button" variant="outline" size="sm" onClick={onAddSkillRow}>
                  {t("formDialog.addSkill")}
                </Button>
              </div>
              <div className="grid gap-1 rounded-md border border-dashed p-3 text-sm text-muted-foreground">
                <p className="font-medium text-foreground">{t("workspace.availableSkillsTitle")}</p>
                {skillCatalogLoading ? (
                  <p>Loading...</p>
                ) : skillCatalog.length > 0 ? (
                  <p>{skillCatalog.map((skill) => skill.label).join(", ")}</p>
                ) : (
                  <p>{t("workspace.availableSkillsEmpty")}</p>
                )}
              </div>
              <div className="grid gap-4">
                {draft.skillRows.length > 0 ? (
                  draft.skillRows.map((skill, index) => {
                    const resolution = draftSkillResolution.find(
                      (entry) => entry.path === skill.path.trim(),
                    );

                    return (
                      <div
                        key={`skill-row-${index}`}
                        className="grid gap-3 rounded-md border p-3 md:grid-cols-[minmax(0,1fr)_auto]"
                      >
                        <div className="flex flex-col gap-1.5">
                          <Label htmlFor={`skill-path-${index}`}>{t("formDialog.skillPath")}</Label>
                          <Input
                            id={`skill-path-${index}`}
                            aria-label="Skill Path"
                            list={`skill-catalog-${index}`}
                            value={skill.path}
                            onChange={(event) =>
                              updateSkillRow(index, "path", event.target.value)
                            }
                            placeholder="skills/react"
                          />
                          <datalist id={`skill-catalog-${index}`}>
                            {skillCatalog.map((catalogEntry) => (
                              <option key={catalogEntry.path} value={catalogEntry.path}>
                                {catalogEntry.label}
                              </option>
                            ))}
                          </datalist>
                          {resolution ? (
                            <p className="text-xs text-muted-foreground">
                              {resolution.status === "resolved"
                                ? `${t("workspace.skillResolvedDetail", {
                                    label: resolution.label,
                                    root: resolution.sourceRoot,
                                  })} · ${
                                      resolution.provenance === "template-derived"
                                        ? t("workspace.skillProvenanceTemplate")
                                        : resolution.provenance === "inherited"
                                          ? t("workspace.skillProvenanceInherited")
                                          : t("workspace.skillProvenanceExplicit")
                                    }`
                                : `${t("workspace.skillUnresolved")} · ${
                                    resolution.provenance === "template-derived"
                                      ? t("workspace.skillProvenanceTemplate")
                                      : resolution.provenance === "inherited"
                                        ? t("workspace.skillProvenanceInherited")
                                        : t("workspace.skillProvenanceExplicit")
                                  }`}
                            </p>
                          ) : null}
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
                          <Label htmlFor={`skill-auto-load-${index}`}>{t("formDialog.autoLoadSkill")}</Label>
                        </div>
                      </div>
                    );
                  })
                ) : (
                  <p className="text-sm text-muted-foreground">
                    {t("workspace.noSkillsYet")}
                  </p>
                )}
              </div>
            </div>
            <SectionErrors errors={validationBySection.capabilities} />
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
                label={t("formDialog.repositories")}
                value={draft.repositories}
                onChange={(value) => updateDraft("repositories", value)}
              />
              <TextAreaField
                id="knowledge-documents"
                label={t("formDialog.documents")}
                value={draft.documents}
                onChange={(value) => updateDraft("documents", value)}
              />
              <TextAreaField
                id="knowledge-patterns"
                label={t("formDialog.patterns")}
                value={draft.patterns}
                onChange={(value) => updateDraft("patterns", value)}
              />
            </div>
            <div className="grid gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-muted-foreground">
                  {t("workspace.addSharedKnowledge")}
                </p>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={onAddKnowledgeRow}
                >
                  {t("workspace.addSharedKnowledge")}
                </Button>
              </div>
              {draft.sharedKnowledgeRows.map((source, index) => {
                const prov = provenanceMap?.sharedKnowledge.find((e) => e.key === source.id.trim());
                return (
                <div
                  key={`knowledge-${index}`}
                  className="grid gap-3 rounded-md border p-3 md:grid-cols-2"
                >
                  <div className="flex items-center gap-2">
                    <Input
                      aria-label="Shared Knowledge ID"
                      value={source.id}
                      placeholder="design-guidelines"
                      onChange={(event) =>
                        updateKnowledgeRow(index, "id", event.target.value)
                      }
                    />
                    {prov ? <ProvenanceBadge provenance={prov.provenance} /> : null}
                  </div>
                  <Input
                    aria-label="Shared Knowledge Type"
                    value={source.type}
                    placeholder="vector"
                    onChange={(event) =>
                      updateKnowledgeRow(index, "type", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Shared Knowledge Access"
                    value={source.access}
                    placeholder="read"
                    onChange={(event) =>
                      updateKnowledgeRow(index, "access", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Shared Knowledge Description"
                    value={source.description}
                    placeholder="Shared UI guidance"
                    onChange={(event) =>
                      updateKnowledgeRow(index, "description", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Shared Knowledge Sources"
                    value={source.sourcesInput}
                    placeholder="docs/PRD.md, docs/part/PLUGIN_SYSTEM_DESIGN.md"
                    onChange={(event) =>
                      updateKnowledgeRow(index, "sourcesInput", event.target.value)
                    }
                  />
                </div>
                );
              })}
            </div>

            <div className="grid gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-muted-foreground">
                  {t("workspace.addPrivateKnowledge")}
                </p>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={onAddPrivateKnowledgeRow}
                >
                  {t("workspace.addPrivateKnowledge")}
                </Button>
              </div>
              {draft.privateKnowledgeRows.map((source, index) => {
                const prov = provenanceMap?.privateKnowledge.find((e) => e.key === source.id.trim());
                return (
                <div
                  key={`private-knowledge-${index}`}
                  className="grid gap-3 rounded-md border p-3 md:grid-cols-2"
                >
                  <div className="flex items-center gap-2">
                    <Input
                      aria-label="Private Knowledge ID"
                      value={source.id}
                      placeholder="operator-notes"
                      onChange={(event) =>
                        updatePrivateKnowledgeRow(index, "id", event.target.value)
                      }
                    />
                    {prov ? <ProvenanceBadge provenance={prov.provenance} /> : null}
                  </div>
                  <Input
                    aria-label="Private Knowledge Type"
                    value={source.type}
                    placeholder="doc"
                    onChange={(event) =>
                      updatePrivateKnowledgeRow(index, "type", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Private Knowledge Access"
                    value={source.access}
                    placeholder="read"
                    onChange={(event) =>
                      updatePrivateKnowledgeRow(index, "access", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Private Knowledge Description"
                    value={source.description}
                    placeholder="Internal notes"
                    onChange={(event) =>
                      updatePrivateKnowledgeRow(index, "description", event.target.value)
                    }
                  />
                  <Input
                    aria-label="Private Knowledge Sources"
                    value={source.sourcesInput}
                    placeholder="docs/notes.md"
                    onChange={(event) =>
                      updatePrivateKnowledgeRow(index, "sourcesInput", event.target.value)
                    }
                  />
                </div>
                );
              })}
            </div>

            <div className="grid gap-4 rounded-lg border p-4">
              <div>
                <h3 className="text-sm font-semibold">{t("workspace.memorySettings")}</h3>
                <p className="text-sm text-muted-foreground">
                  {t("workspace.memorySettingsDesc")}
                </p>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="memory-short-term">{t("workspace.memoryShortTermMaxTokens")}</Label>
                  <Input
                    id="memory-short-term"
                    aria-label="Short-term Memory Max Tokens"
                    value={draft.memoryShortTermMaxTokens}
                    onChange={(event) =>
                      updateDraft("memoryShortTermMaxTokens", event.target.value)
                    }
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="memory-episodic-retention">{t("workspace.memoryEpisodicRetentionDays")}</Label>
                  <Input
                    id="memory-episodic-retention"
                    aria-label="Episodic Memory Retention Days"
                    value={draft.memoryEpisodicRetentionDays}
                    onChange={(event) =>
                      updateDraft("memoryEpisodicRetentionDays", event.target.value)
                    }
                  />
                </div>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={draft.memoryEpisodicEnabled}
                    onChange={(event) =>
                      updateDraft("memoryEpisodicEnabled", event.target.checked)
                    }
                  />
                  {t("workspace.memoryEpisodicEnabled")}
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={draft.memorySemanticEnabled}
                    onChange={(event) =>
                      updateDraft("memorySemanticEnabled", event.target.checked)
                    }
                  />
                  {t("workspace.memorySemanticEnabled")}
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={draft.memorySemanticAutoExtract}
                    onChange={(event) =>
                      updateDraft("memorySemanticAutoExtract", event.target.checked)
                    }
                  />
                  {t("workspace.memorySemanticAutoExtract")}
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={draft.memoryProceduralEnabled}
                    onChange={(event) =>
                      updateDraft("memoryProceduralEnabled", event.target.checked)
                    }
                  />
                  {t("workspace.memoryProceduralEnabled")}
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={draft.memoryProceduralLearnFromFeedback}
                    onChange={(event) =>
                      updateDraft(
                        "memoryProceduralLearnFromFeedback",
                        event.target.checked,
                      )
                    }
                  />
                  {t("workspace.memoryProceduralLearnFromFeedback")}
                </label>
              </div>
            </div>
            <SectionErrors errors={validationBySection.knowledge} />
          </AuthoringSection>

          <AuthoringSection
            sectionId="governance"
            title="Governance"
            description="Keep safety, collaboration, and activation cues visible before execution."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <div className="grid gap-4 rounded-lg border p-4">
              <h3 className="text-sm font-semibold">{t("formDialog.security")}</h3>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="security-profile">{t("workspace.securityProfile")}</Label>
                  <Input
                    id="security-profile"
                    value={draft.securityProfile}
                    onChange={(event) => updateDraft("securityProfile", event.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="security-permission">{t("formDialog.permissionMode")}</Label>
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
                label={t("workspace.outputFilters")}
                value={draft.outputFilters}
                onChange={(value) => updateDraft("outputFilters", value)}
              />
              <div className="grid gap-4 md:grid-cols-2">
                <TextAreaField
                  id="security-allowed"
                  label={t("formDialog.allowedPaths")}
                  value={draft.allowedPaths}
                  onChange={(value) => updateDraft("allowedPaths", value)}
                />
                <TextAreaField
                  id="security-denied"
                  label={t("formDialog.deniedPaths")}
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
                <Label htmlFor="security-review">{t("formDialog.requireReview")}</Label>
              </div>
            </div>

            <details className="rounded-lg border p-4">
              <summary className="cursor-pointer text-sm font-semibold">
                {t("workspace.permissions")}
              </summary>
              <div className="mt-4 grid gap-4">
                <div className="grid gap-4 md:grid-cols-2">
                  <TextAreaField
                    id="perm-file-allowed"
                    label={t("workspace.permFileAllowedPaths")}
                    value={draft.permFileAllowedPaths}
                    onChange={(value) => updateDraft("permFileAllowedPaths", value)}
                  />
                  <TextAreaField
                    id="perm-file-denied"
                    label={t("workspace.permFileDeniedPaths")}
                    value={draft.permFileDeniedPaths}
                    onChange={(value) => updateDraft("permFileDeniedPaths", value)}
                  />
                </div>
                <TextAreaField
                  id="perm-network-domains"
                  label={t("workspace.permNetworkAllowedDomains")}
                  value={draft.permNetworkAllowedDomains}
                  onChange={(value) => updateDraft("permNetworkAllowedDomains", value)}
                />
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex items-center gap-2">
                    <input
                      id="perm-code-sandbox"
                      type="checkbox"
                      checked={draft.permCodeSandbox}
                      onChange={(event) => updateDraft("permCodeSandbox", event.target.checked)}
                    />
                    <Label htmlFor="perm-code-sandbox">{t("workspace.permCodeSandbox")}</Label>
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="perm-code-languages">{t("workspace.permCodeAllowedLanguages")}</Label>
                    <Input
                      id="perm-code-languages"
                      value={draft.permCodeAllowedLanguages}
                      onChange={(event) => updateDraft("permCodeAllowedLanguages", event.target.value)}
                      placeholder="python, javascript"
                    />
                  </div>
                </div>
              </div>
            </details>

            <details className="rounded-lg border p-4">
              <summary className="cursor-pointer text-sm font-semibold">
                {t("workspace.resourceLimits")}
              </summary>
              <div className="mt-4 grid gap-4">
                <div className="grid gap-4 md:grid-cols-3">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-token-task">{t("workspace.resTokenPerTask")}</Label>
                    <Input id="res-token-task" type="number" value={draft.resourceTokenBudgetPerTask} onChange={(e) => updateDraft("resourceTokenBudgetPerTask", e.target.value)} />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-token-day">{t("workspace.resTokenPerDay")}</Label>
                    <Input id="res-token-day" type="number" value={draft.resourceTokenBudgetPerDay} onChange={(e) => updateDraft("resourceTokenBudgetPerDay", e.target.value)} />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-token-month">{t("workspace.resTokenPerMonth")}</Label>
                    <Input id="res-token-month" type="number" value={draft.resourceTokenBudgetPerMonth} onChange={(e) => updateDraft("resourceTokenBudgetPerMonth", e.target.value)} />
                  </div>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-api-minute">{t("workspace.resApiPerMinute")}</Label>
                    <Input id="res-api-minute" type="number" value={draft.resourceApiCallsPerMinute} onChange={(e) => updateDraft("resourceApiCallsPerMinute", e.target.value)} />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-api-hour">{t("workspace.resApiPerHour")}</Label>
                    <Input id="res-api-hour" type="number" value={draft.resourceApiCallsPerHour} onChange={(e) => updateDraft("resourceApiCallsPerHour", e.target.value)} />
                  </div>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-exec-task">{t("workspace.resExecPerTask")}</Label>
                    <Input id="res-exec-task" value={draft.resourceExecTimePerTask} onChange={(e) => updateDraft("resourceExecTimePerTask", e.target.value)} placeholder="30m" />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-exec-day">{t("workspace.resExecPerDay")}</Label>
                    <Input id="res-exec-day" value={draft.resourceExecTimePerDay} onChange={(e) => updateDraft("resourceExecTimePerDay", e.target.value)} placeholder="4h" />
                  </div>
                </div>
                <div className="grid gap-4 md:grid-cols-3">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-cost-task">{t("workspace.resCostPerTask")}</Label>
                    <Input id="res-cost-task" value={draft.resourceCostPerTask} onChange={(e) => updateDraft("resourceCostPerTask", e.target.value)} placeholder="$5.00" />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-cost-day">{t("workspace.resCostPerDay")}</Label>
                    <Input id="res-cost-day" value={draft.resourceCostPerDay} onChange={(e) => updateDraft("resourceCostPerDay", e.target.value)} placeholder="$50.00" />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="res-cost-alert">{t("workspace.resCostAlertThreshold")}</Label>
                    <Input id="res-cost-alert" type="number" step="0.01" value={draft.resourceCostAlertThreshold} onChange={(e) => updateDraft("resourceCostAlertThreshold", e.target.value)} placeholder="0.80" />
                  </div>
                </div>
              </div>
            </details>

            <div className="grid gap-4 rounded-lg border p-4">
              <h3 className="text-sm font-semibold">{t("workspace.collaboration")}</h3>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="collaboration-delegate">{t("workspace.canDelegateTo")}</Label>
                  <Input
                    id="collaboration-delegate"
                    value={draft.collaborationCanDelegateTo}
                    onChange={(event) =>
                      updateDraft("collaborationCanDelegateTo", event.target.value)
                    }
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="collaboration-accepts">{t("workspace.acceptsDelegation")}</Label>
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
                  <h3 className="text-sm font-semibold">{t("workspace.triggers")}</h3>
                  <p className="text-sm text-muted-foreground">
                    {t("workspace.triggersDesc")}
                  </p>
                </div>
                <Button type="button" variant="outline" size="sm" onClick={onAddTriggerRow}>
                  {t("workspace.addTrigger")}
                </Button>
              </div>
              {draft.triggerRows.map((trigger, index) => {
                const trigKey = `${trigger.event.trim()}:${trigger.action.trim()}`;
                const prov = provenanceMap?.triggers.find((e) => e.key === trigKey);
                return (
                  <div
                    key={`trigger-${index}`}
                    className="grid gap-3 rounded-md border p-3 md:grid-cols-3"
                  >
                    <div className="flex items-center gap-2">
                      <Input
                        aria-label="Trigger Event"
                        value={trigger.event}
                        placeholder="pr_created"
                        onChange={(event) =>
                          updateTriggerRow(index, "event", event.target.value)
                        }
                      />
                      {prov ? <ProvenanceBadge provenance={prov.provenance} /> : null}
                    </div>
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
                );
              })}
            </div>
            <SectionErrors errors={validationBySection.governance} />
          </AuthoringSection>

          <AuthoringSection
            sectionId="review"
            title="Review"
            description="Use the execution summary, YAML preview, preview, and sandbox surfaces before saving."
            activeSection={activeSection}
            onSelectSection={onSelectSection}
          >
            <p className="text-sm text-muted-foreground">
              {t("workspace.reviewDesc")}
            </p>
            <div className="grid gap-4 rounded-lg border p-4">
              <div>
                <h3 className="text-sm font-semibold">{t("workspace.overrideEditor")}</h3>
                <p className="text-sm text-muted-foreground">
                  {t("workspace.overrideEditorDesc")}
                </p>
              </div>
              <textarea
                aria-label="Role Overrides"
                className="min-h-32 rounded-md border bg-background px-3 py-2 text-sm font-mono"
                value={draft.overridesInput}
                onChange={(event) => updateDraft("overridesInput", event.target.value)}
              />
            </div>
            <SectionErrors errors={validationBySection.review} />
          </AuthoringSection>

        <div className="sticky bottom-0 flex flex-wrap items-center gap-3 border-t bg-card px-4 py-3">
          <Button type="submit" disabled={saving || !draft.roleId || !draft.name}>
            {saving ? t("formDialog.saving") : t("workspace.saveRole")}
          </Button>
          {mode === "edit" && selectedRole ? (
            <Button type="button" variant="outline" onClick={onSwitchToCreate}>
              {t("workspace.switchToCreate")}
            </Button>
          ) : null}
        </div>
      </form>
    </div>
  );
}
