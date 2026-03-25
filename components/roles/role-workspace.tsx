"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Plus } from "lucide-react";
import { RoleCard } from "./role-card";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { RoleManifest, RolePreviewResponse, RoleSandboxResponse } from "@/lib/stores/role-store";
import {
  buildRoleDraft,
  buildRoleExecutionSummary,
  renderRoleManifestYaml,
  serializeRoleDraft,
  type RoleDraft,
  type RoleKnowledgeSourceDraft,
  type RoleSkillDraft,
  type RoleTriggerDraft,
} from "@/lib/roles/role-management";

interface RoleWorkspaceProps {
  roles: RoleManifest[];
  loading: boolean;
  error: string | null;
  onCreateRole: (data: Partial<RoleManifest>) => Promise<unknown>;
  onUpdateRole: (id: string, data: Partial<RoleManifest>) => Promise<unknown>;
  onDeleteRole: (role: RoleManifest) => Promise<unknown>;
  onPreviewRole: (payload: { roleId?: string; draft?: Partial<RoleManifest> }) => Promise<RolePreviewResponse | void>;
  onSandboxRole: (payload: { roleId?: string; draft?: Partial<RoleManifest>; input: string }) => Promise<RoleSandboxResponse | void>;
}

function TextAreaField({ id, label, value, onChange, rows = 3 }: { id: string; label: string; value: string; onChange: (value: string) => void; rows?: number }) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <textarea id={id} className="min-h-24 rounded-md border bg-background px-3 py-2 text-sm" value={value} rows={rows} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function RoleSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="grid gap-4 rounded-lg border p-4">
      <h3 className="text-sm font-semibold">{title}</h3>
      {children}
    </section>
  );
}

export function RoleWorkspace({
  roles,
  loading,
  error,
  onCreateRole,
  onUpdateRole,
  onDeleteRole,
  onPreviewRole,
  onSandboxRole,
}: RoleWorkspaceProps) {
  const [mode, setMode] = useState<"create" | "edit">("create");
  const [selectedRoleId, setSelectedRoleId] = useState("");
  const [templateId, setTemplateId] = useState("");
  const [draft, setDraft] = useState<RoleDraft>(() => buildRoleDraft());
  const [saving, setSaving] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [sandboxLoading, setSandboxLoading] = useState(false);
  const [sandboxInput, setSandboxInput] = useState("");
  const [previewResult, setPreviewResult] = useState<RolePreviewResponse | null>(null);
  const [sandboxResult, setSandboxResult] = useState<RoleSandboxResponse | null>(null);
  const sandboxInputRef = useRef("");

  const selectedRole = useMemo(() => roles.find((role) => role.metadata.id === selectedRoleId), [roles, selectedRoleId]);

  useEffect(() => {
    if (mode === "edit" && selectedRole) {
      setDraft(buildRoleDraft(selectedRole));
      return;
    }
    if (mode === "edit" && !selectedRole && roles.length > 0) {
      setSelectedRoleId(roles[0]!.metadata.id);
      setDraft(buildRoleDraft(roles[0]));
    }
  }, [mode, roles, selectedRole]);

  const serializedDraft = useMemo(() => serializeRoleDraft(draft, selectedRole), [draft, selectedRole]);
  const draftValidationErrors = serializedDraft.validationErrors ?? [];
  const executionSummary = useMemo(() => buildRoleExecutionSummary(draft), [draft]);
  const getRequestPayload = () => {
    const payload = { ...serializedDraft };
    delete payload.validationErrors;
    return payload;
  };
  const yamlPreview = renderRoleManifestYaml(previewResult?.effectiveManifest ?? getRequestPayload());

  const updateDraft = <K extends keyof RoleDraft>(key: K, value: RoleDraft[K]) => setDraft((current) => ({ ...current, [key]: value }));

  const handleNewRole = () => {
    setMode("create");
    setSelectedRoleId("");
    setTemplateId("");
    setDraft(buildRoleDraft());
    setPreviewResult(null);
    setSandboxResult(null);
  };

  const handleEditRole = (role: RoleManifest) => {
    setMode("edit");
    setTemplateId("");
    setSelectedRoleId(role.metadata.id);
    setDraft(buildRoleDraft(role));
    setPreviewResult(null);
    setSandboxResult(null);
  };

  const handleTemplateChange = (value: string) => {
    setTemplateId(value);
    const template = roles.find((role) => role.metadata.id === value);
    if (!template) {
      setDraft(buildRoleDraft());
      return;
    }
    const templateDraft = buildRoleDraft(template);
    setDraft({ ...templateDraft, roleId: `${template.metadata.id}-copy` });
  };

  const updateSkillRow = (index: number, field: keyof RoleSkillDraft, value: RoleSkillDraft[keyof RoleSkillDraft]) => {
    setDraft((current) => ({
      ...current,
      skillRows: current.skillRows.map((skill, skillIndex) => (skillIndex === index ? { ...skill, [field]: value } : skill)),
    }));
  };

  const updateKnowledgeRow = (index: number, field: keyof RoleKnowledgeSourceDraft, value: RoleKnowledgeSourceDraft[keyof RoleKnowledgeSourceDraft]) => {
    setDraft((current) => ({
      ...current,
      sharedKnowledgeRows: current.sharedKnowledgeRows.map((source, sourceIndex) => (sourceIndex === index ? { ...source, [field]: value } : source)),
    }));
  };

  const updateTriggerRow = (index: number, field: keyof RoleTriggerDraft, value: RoleTriggerDraft[keyof RoleTriggerDraft]) => {
    setDraft((current) => ({
      ...current,
      triggerRows: current.triggerRows.map((trigger, triggerIndex) => (triggerIndex === index ? { ...trigger, [field]: value } : trigger)),
    }));
  };

  const handleSave = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSaving(true);
    try {
      if (draftValidationErrors.length > 0) return;
      const requestPayload = getRequestPayload();
      if (mode === "edit" && selectedRole) {
        await onUpdateRole(selectedRole.metadata.id, requestPayload);
      } else {
        await onCreateRole(requestPayload);
      }
    } finally {
      setSaving(false);
    }
  };

  const handlePreview = async () => {
    setPreviewLoading(true);
    try {
      setPreviewResult((await onPreviewRole({ draft: getRequestPayload() })) ?? null);
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleSandbox = async () => {
    setSandboxLoading(true);
    try {
      setSandboxResult((await onSandboxRole({ draft: getRequestPayload(), input: sandboxInputRef.current })) ?? null);
    } finally {
      setSandboxLoading(false);
    }
  };

  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,1.45fr)]">
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold">Role Configuration</h1>
            <p className="text-sm text-muted-foreground">Manage reusable role manifests and compare their execution constraints.</p>
          </div>
          <Button onClick={handleNewRole}>
            <Plus className="mr-1 size-4" />
            New Role
          </Button>
        </div>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        {loading && roles.length === 0 ? (
          <p className="text-sm text-muted-foreground">Loading roles...</p>
        ) : roles.length === 0 ? (
          <Card><CardHeader><CardTitle>Role Library</CardTitle><CardDescription>Create the first role to start shaping your engineering roster.</CardDescription></CardHeader></Card>
        ) : (
          <div className="grid gap-4">
            {roles.map((role) => (
              <RoleCard key={role.metadata.id} role={role} onEdit={() => handleEditRole(role)} onDelete={() => void onDeleteRole(role)} />
            ))}
          </div>
        )}
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.6fr)_minmax(280px,0.9fr)]">
        <Card>
          <CardHeader>
            <CardTitle>{mode === "edit" ? "Role Workspace" : "Create Role"}</CardTitle>
            <CardDescription>Use structured sections to define identity, capabilities, knowledge, and governance.</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="flex flex-col gap-4" onSubmit={handleSave}>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="role-template">Start from template</Label>
                  <select id="role-template" aria-label="Start from template" className="h-10 rounded-md border bg-background px-3 text-sm" value={templateId} onChange={(event) => handleTemplateChange(event.target.value)} disabled={mode === "edit"}>
                    <option value="">Blank role</option>
                    {roles.map((role) => <option key={role.metadata.id} value={role.metadata.id}>{role.metadata.name}</option>)}
                  </select>
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="role-extends">Inherits from</Label>
                  <select id="role-extends" aria-label="Inherits from" className="h-10 rounded-md border bg-background px-3 text-sm" value={draft.extendsValue} onChange={(event) => updateDraft("extendsValue", event.target.value)}>
                    <option value="">No parent</option>
                    {roles.map((role) => <option key={role.metadata.id} value={role.metadata.id}>{role.metadata.name}</option>)}
                  </select>
                </div>
              </div>

              <RoleSection title="Identity">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5"><Label htmlFor="role-id">Role ID</Label><Input id="role-id" aria-label="Role ID" value={draft.roleId} onChange={(event) => updateDraft("roleId", event.target.value)} disabled={mode === "edit"} required /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="role-name">Name</Label><Input id="role-name" value={draft.name} onChange={(event) => updateDraft("name", event.target.value)} required /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="role-version">Version</Label><Input id="role-version" value={draft.version} onChange={(event) => updateDraft("version", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="role-tags">Tags</Label><Input id="role-tags" value={draft.tagsInput} onChange={(event) => updateDraft("tagsInput", event.target.value)} /></div>
                </div>
                <TextAreaField id="role-description" label="Description" value={draft.description} onChange={(value) => updateDraft("description", value)} />
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5"><Label htmlFor="identity-role">Role Title</Label><Input id="identity-role" value={draft.identityRole} onChange={(event) => updateDraft("identityRole", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="identity-goal">Goal</Label><Input id="identity-goal" value={draft.goal} onChange={(event) => updateDraft("goal", event.target.value)} /></div>
                </div>
                <TextAreaField id="identity-backstory" label="Backstory" value={draft.backstory} onChange={(value) => updateDraft("backstory", value)} />
                <TextAreaField id="identity-prompt" label="System Prompt" value={draft.systemPrompt} onChange={(value) => updateDraft("systemPrompt", value)} rows={5} />
              </RoleSection>

              <RoleSection title="Advanced Identity">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5"><Label htmlFor="identity-persona">Persona</Label><Input id="identity-persona" value={draft.persona} onChange={(event) => updateDraft("persona", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="identity-personality">Personality</Label><Input id="identity-personality" value={draft.personality} onChange={(event) => updateDraft("personality", event.target.value)} /></div>
                </div>
              </RoleSection>

              <RoleSection title="Capabilities">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5"><Label htmlFor="cap-packages">Packages</Label><Input id="cap-packages" value={draft.packages} onChange={(event) => updateDraft("packages", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="cap-tools">Allowed Tools</Label><Input id="cap-tools" aria-label="Allowed Tools" value={draft.allowedTools} onChange={(event) => updateDraft("allowedTools", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="cap-external-tools">External Tools</Label><Input id="cap-external-tools" value={draft.externalTools} onChange={(event) => updateDraft("externalTools", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="cap-languages">Languages</Label><Input id="cap-languages" value={draft.languages} onChange={(event) => updateDraft("languages", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="cap-frameworks">Frameworks</Label><Input id="cap-frameworks" value={draft.frameworks} onChange={(event) => updateDraft("frameworks", event.target.value)} /></div>
                </div>
              </RoleSection>

              <RoleSection title="Skills">
                <div className="flex items-center justify-between">
                  <p className="text-sm text-muted-foreground">Declare reusable skill references and whether they should auto-load for this role.</p>
                  <Button type="button" variant="outline" size="sm" onClick={() => setDraft((current) => ({ ...current, skillRows: [...current.skillRows, { path: "", autoLoad: false }] }))}>Add Skill</Button>
                </div>
                <div className="grid gap-4">
                  {draft.skillRows.length > 0 ? draft.skillRows.map((skill, index) => (
                    <div key={`skill-row-${index}`} className="grid gap-3 rounded-md border p-3 md:grid-cols-[minmax(0,1fr)_auto]">
                      <div className="flex flex-col gap-1.5"><Label htmlFor={`skill-path-${index}`}>Skill Path</Label><Input id={`skill-path-${index}`} aria-label="Skill Path" value={skill.path} onChange={(event) => updateSkillRow(index, "path", event.target.value)} placeholder="skills/react" /></div>
                      <div className="flex items-center gap-2 pt-7">
                        <input id={`skill-auto-load-${index}`} type="checkbox" checked={skill.autoLoad} onChange={(event) => updateSkillRow(index, "autoLoad", event.target.checked)} />
                        <Label htmlFor={`skill-auto-load-${index}`}>Auto-load skill</Label>
                      </div>
                    </div>
                  )) : <p className="text-sm text-muted-foreground">No skills configured for this role yet.</p>}
                </div>
                {draftValidationErrors.length > 0 ? <div className="grid gap-1">{draftValidationErrors.map((errorMessage) => <p key={errorMessage} className="text-sm text-destructive">{errorMessage}</p>)}</div> : null}
              </RoleSection>

              <RoleSection title="Knowledge">
                <div className="grid gap-4 md:grid-cols-3">
                  <TextAreaField id="knowledge-repositories" label="Repositories" value={draft.repositories} onChange={(value) => updateDraft("repositories", value)} />
                  <TextAreaField id="knowledge-documents" label="Documents" value={draft.documents} onChange={(value) => updateDraft("documents", value)} />
                  <TextAreaField id="knowledge-patterns" label="Patterns" value={draft.patterns} onChange={(value) => updateDraft("patterns", value)} />
                </div>
                <div className="flex items-center justify-between">
                  <p className="text-sm text-muted-foreground">Shared knowledge sources that the role can cite or reuse.</p>
                  <Button type="button" variant="outline" size="sm" onClick={() => setDraft((current) => ({ ...current, sharedKnowledgeRows: [...current.sharedKnowledgeRows, { id: "", type: "", access: "", description: "", sourcesInput: "" }] }))}>Add Shared Knowledge</Button>
                </div>
                {draft.sharedKnowledgeRows.map((source, index) => (
                  <div key={`knowledge-${index}`} className="grid gap-3 rounded-md border p-3 md:grid-cols-2">
                    <Input aria-label="Shared Knowledge ID" value={source.id} placeholder="design-guidelines" onChange={(event) => updateKnowledgeRow(index, "id", event.target.value)} />
                    <Input aria-label="Shared Knowledge Type" value={source.type} placeholder="vector" onChange={(event) => updateKnowledgeRow(index, "type", event.target.value)} />
                  </div>
                ))}
              </RoleSection>

              <RoleSection title="Security">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5"><Label htmlFor="security-profile">Security Profile</Label><Input id="security-profile" value={draft.securityProfile} onChange={(event) => updateDraft("securityProfile", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="security-permission">Permission Mode</Label><Input id="security-permission" aria-label="Permission Mode" value={draft.permissionMode} onChange={(event) => updateDraft("permissionMode", event.target.value)} /></div>
                </div>
                <TextAreaField id="security-output-filters" label="Output Filters" value={draft.outputFilters} onChange={(value) => updateDraft("outputFilters", value)} />
                <div className="grid gap-4 md:grid-cols-2">
                  <TextAreaField id="security-allowed" label="Allowed Paths" value={draft.allowedPaths} onChange={(value) => updateDraft("allowedPaths", value)} />
                  <TextAreaField id="security-denied" label="Denied Paths" value={draft.deniedPaths} onChange={(value) => updateDraft("deniedPaths", value)} />
                </div>
                <div className="flex items-center gap-2">
                  <input id="security-review" type="checkbox" checked={draft.requireReview} onChange={(event) => updateDraft("requireReview", event.target.checked)} />
                  <Label htmlFor="security-review">Require review before execution</Label>
                </div>
              </RoleSection>

              <RoleSection title="Collaboration">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5"><Label htmlFor="collaboration-delegate">Can Delegate To</Label><Input id="collaboration-delegate" value={draft.collaborationCanDelegateTo} onChange={(event) => updateDraft("collaborationCanDelegateTo", event.target.value)} /></div>
                  <div className="flex flex-col gap-1.5"><Label htmlFor="collaboration-accepts">Accepts Delegation From</Label><Input id="collaboration-accepts" value={draft.collaborationAcceptsDelegationFrom} onChange={(event) => updateDraft("collaborationAcceptsDelegationFrom", event.target.value)} /></div>
                </div>
              </RoleSection>

              <RoleSection title="Triggers">
                <div className="flex items-center justify-between">
                  <p className="text-sm text-muted-foreground">Define lightweight activation cues for this role.</p>
                  <Button type="button" variant="outline" size="sm" onClick={() => setDraft((current) => ({ ...current, triggerRows: [...current.triggerRows, { event: "", action: "", condition: "" }] }))}>Add Trigger</Button>
                </div>
                {draft.triggerRows.map((trigger, index) => (
                  <div key={`trigger-${index}`} className="grid gap-3 rounded-md border p-3 md:grid-cols-3">
                    <Input aria-label="Trigger Event" value={trigger.event} placeholder="pr_created" onChange={(event) => updateTriggerRow(index, "event", event.target.value)} />
                    <Input aria-label="Trigger Action" value={trigger.action} placeholder="auto_review" onChange={(event) => updateTriggerRow(index, "action", event.target.value)} />
                    <Input aria-label="Trigger Condition" value={trigger.condition} placeholder="labels.includes('ui')" onChange={(event) => updateTriggerRow(index, "condition", event.target.value)} />
                  </div>
                ))}
              </RoleSection>

              <div className="flex items-center gap-3">
                <Button type="submit" disabled={saving || !draft.roleId || !draft.name}>{saving ? "Saving..." : "Save Role"}</Button>
                {mode === "edit" && selectedRole ? <Button type="button" variant="outline" onClick={handleNewRole}>Switch to Create</Button> : null}
              </div>
            </form>
          </CardContent>
        </Card>

        <div className="grid gap-4">
          <Card>
            <CardHeader><CardTitle>Authoring Guide</CardTitle><CardDescription>Use advanced identity, governance, and preview flows to refine a role before saving.</CardDescription></CardHeader>
            <CardContent className="grid gap-2 text-sm text-muted-foreground">
              <p>Advanced identity fields shape tone and response style.</p>
              <p>Packages and external tools describe reusable capability bundles and connected tool hosts.</p>
              <p>Preview and sandbox let you inspect effective values and run a bounded probe without persisting a draft.</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle>Execution Summary</CardTitle><CardDescription>Review the draft execution intent and governance settings before saving.</CardDescription></CardHeader>
            <CardContent className="grid gap-3 text-sm">
              <div><p className="font-medium">Prompt intent</p><p className="text-muted-foreground">{executionSummary.promptIntent || "No prompt intent yet"}</p></div>
              <div><p className="font-medium">Allowed tools</p><p className="text-muted-foreground">{executionSummary.toolsLabel}</p></div>
              <div><p className="font-medium">Skills</p><p className="text-muted-foreground">{executionSummary.skillsLabel}</p><p className="text-muted-foreground">{executionSummary.keySkillPaths.length > 0 ? executionSummary.keySkillPaths.join(", ") : "No key skills selected"}</p></div>
              <div><p className="font-medium">Budget</p><p className="text-muted-foreground">{executionSummary.budgetLabel}</p></div>
              <div><p className="font-medium">Turn limit</p><p className="text-muted-foreground">{executionSummary.turnsLabel}</p></div>
              <div><p className="font-medium">Permission mode</p><p className="text-muted-foreground">{executionSummary.permissionMode}</p></div>
              <div><p className="font-medium">Safety cues</p><ul className="list-disc space-y-1 pl-5 text-muted-foreground">{executionSummary.safetyCues.length > 0 ? executionSummary.safetyCues.map((cue) => <li key={cue}>{cue}</li>) : <li>No additional safety cues configured</li>}</ul></div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle>YAML Preview</CardTitle><CardDescription>Inspect the current draft as a YAML-oriented manifest before saving.</CardDescription></CardHeader>
            <CardContent><pre className="max-h-72 overflow-auto rounded-md border bg-muted/30 p-3 text-xs">{yamlPreview}</pre></CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle>Preview And Sandbox</CardTitle><CardDescription>Resolve effective values and optionally run a bounded prompt probe.</CardDescription></CardHeader>
            <CardContent className="grid gap-3">
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={() => void handlePreview()} disabled={previewLoading}>{previewLoading ? "Previewing..." : "Preview Role Draft"}</Button>
                <Button type="button" variant="outline" onClick={() => void handleSandbox()} disabled={sandboxLoading}>{sandboxLoading ? "Running..." : "Run Sandbox Probe"}</Button>
              </div>
              <TextAreaField id="sandbox-input" label="Sandbox Input" value={sandboxInput} onChange={(value) => { setSandboxInput(value); sandboxInputRef.current = value; }} rows={4} />
              {sandboxResult?.selection ? <p className="text-sm text-muted-foreground">{`${sandboxResult.selection.runtime} / ${sandboxResult.selection.provider} / ${sandboxResult.selection.model}`}</p> : null}
              {sandboxResult?.probe?.text ? <p className="text-sm">{sandboxResult.probe.text}</p> : null}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
