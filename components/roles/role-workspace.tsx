"use client";

import { useEffect, useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { RoleCard } from "./role-card";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { RoleManifest } from "@/lib/stores/role-store";
import {
  buildRoleDraft,
  buildRoleExecutionSummary,
  serializeRoleDraft,
  type RoleDraft,
} from "@/lib/roles/role-management";

interface RoleWorkspaceProps {
  roles: RoleManifest[];
  loading: boolean;
  error: string | null;
  onCreateRole: (data: Partial<RoleManifest>) => Promise<unknown>;
  onUpdateRole: (id: string, data: Partial<RoleManifest>) => Promise<unknown>;
  onDeleteRole: (role: RoleManifest) => Promise<unknown>;
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

function RoleSection({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
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
}: RoleWorkspaceProps) {
  const [mode, setMode] = useState<"create" | "edit">("create");
  const [selectedRoleId, setSelectedRoleId] = useState<string>("");
  const [templateId, setTemplateId] = useState("");
  const [draft, setDraft] = useState<RoleDraft>(() => buildRoleDraft());
  const [saving, setSaving] = useState(false);

  const selectedRole = useMemo(
    () => roles.find((role) => role.metadata.id === selectedRoleId),
    [roles, selectedRoleId],
  );

  useEffect(() => {
    if (mode === "edit" && selectedRole) {
      setDraft(buildRoleDraft(selectedRole));
      return;
    }

    if (mode === "edit" && !selectedRole && roles.length > 0) {
      const nextRole = roles[0];
      setSelectedRoleId(nextRole.metadata.id);
      setDraft(buildRoleDraft(nextRole));
    }
  }, [mode, roles, selectedRole]);

  const executionSummary = useMemo(
    () => buildRoleExecutionSummary(draft),
    [draft],
  );

  const updateDraft = <K extends keyof RoleDraft>(key: K, value: RoleDraft[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const handleNewRole = () => {
    setMode("create");
    setSelectedRoleId("");
    setTemplateId("");
    setDraft(buildRoleDraft());
  };

  const handleEditRole = (role: RoleManifest) => {
    setMode("edit");
    setTemplateId("");
    setSelectedRoleId(role.metadata.id);
    setDraft(buildRoleDraft(role));
  };

  const handleTemplateChange = (value: string) => {
    setTemplateId(value);
    if (!value) {
      setDraft(buildRoleDraft());
      return;
    }

    const template = roles.find((role) => role.metadata.id === value);
    if (!template) return;

    const templateDraft = buildRoleDraft(template);
    setDraft({
      ...templateDraft,
      roleId: `${template.metadata.id}-copy`,
      extendsValue: templateDraft.extendsValue,
    });
  };

  const handleSave = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSaving(true);
    try {
      const payload = serializeRoleDraft(draft, selectedRole);
      if (mode === "edit" && selectedRole) {
        await onUpdateRole(selectedRole.metadata.id, payload);
      } else {
        await onCreateRole(payload);
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,1.45fr)]">
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold">Role Configuration</h1>
            <p className="text-sm text-muted-foreground">
              Manage reusable role manifests and compare their execution constraints.
            </p>
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
          <Card>
            <CardHeader>
              <CardTitle>Role Library</CardTitle>
              <CardDescription>Create the first role to start shaping your engineering roster.</CardDescription>
            </CardHeader>
          </Card>
        ) : (
          <div className="grid gap-4">
            {roles.map((role) => (
              <RoleCard
                key={role.metadata.id}
                role={role}
                onEdit={() => handleEditRole(role)}
                onDelete={() => void onDeleteRole(role)}
              />
            ))}
          </div>
        )}
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.6fr)_minmax(280px,0.9fr)]">
        <Card>
          <CardHeader>
            <CardTitle>{mode === "edit" ? "Role Workspace" : "Create Role"}</CardTitle>
            <CardDescription>
              Use structured sections to define identity, capabilities, knowledge, and governance.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form className="flex flex-col gap-4" onSubmit={handleSave}>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="role-template">Start from template</Label>
                  <select
                    id="role-template"
                    aria-label="Start from template"
                    className="h-10 rounded-md border bg-background px-3 text-sm"
                    value={templateId}
                    onChange={(event) => handleTemplateChange(event.target.value)}
                    disabled={mode === "edit"}
                  >
                    <option value="">Blank role</option>
                    {roles.map((role) => (
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
                    {roles.map((role) => (
                      <option key={role.metadata.id} value={role.metadata.id}>
                        {role.metadata.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <RoleSection title="Identity">
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
              </RoleSection>

              <RoleSection title="Capabilities">
                <div className="grid gap-4 md:grid-cols-2">
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
                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="flex flex-col gap-1.5">
                      <Label htmlFor="cap-turns">Max Turns</Label>
                      <Input
                        id="cap-turns"
                        value={draft.maxTurns}
                        onChange={(event) => updateDraft("maxTurns", event.target.value)}
                      />
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <Label htmlFor="cap-budget">Max Budget USD</Label>
                      <Input
                        id="cap-budget"
                        value={draft.maxBudgetUsd}
                        onChange={(event) => updateDraft("maxBudgetUsd", event.target.value)}
                      />
                    </div>
                  </div>
                </div>
              </RoleSection>

              <RoleSection title="Knowledge">
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
              </RoleSection>

              <RoleSection title="Security">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="security-permission">Permission Mode</Label>
                    <Input
                      id="security-permission"
                      aria-label="Permission Mode"
                      value={draft.permissionMode}
                      onChange={(event) => updateDraft("permissionMode", event.target.value)}
                    />
                  </div>
                  <div className="flex items-center gap-2 pt-7">
                    <input
                      id="security-review"
                      type="checkbox"
                      checked={draft.requireReview}
                      onChange={(event) => updateDraft("requireReview", event.target.checked)}
                    />
                    <Label htmlFor="security-review">Require review before execution</Label>
                  </div>
                </div>
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
              </RoleSection>

              <div className="flex items-center gap-3">
                <Button type="submit" disabled={saving || !draft.roleId || !draft.name}>
                  {saving ? "Saving..." : "Save Role"}
                </Button>
                {mode === "edit" && selectedRole ? (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleNewRole}
                  >
                    Switch to Create
                  </Button>
                ) : null}
              </div>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Execution Summary</CardTitle>
            <CardDescription>
              Review the draft's execution intent and governance settings before saving.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 text-sm">
            <div>
              <p className="font-medium">Prompt intent</p>
              <p className="text-muted-foreground">{executionSummary.promptIntent || "No prompt intent yet"}</p>
            </div>
            <div>
              <p className="font-medium">Allowed tools</p>
              <p className="text-muted-foreground">{executionSummary.toolsLabel}</p>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <p className="font-medium">Budget</p>
                <p className="text-muted-foreground">{executionSummary.budgetLabel}</p>
              </div>
              <div>
                <p className="font-medium">Turn limit</p>
                <p className="text-muted-foreground">{executionSummary.turnsLabel}</p>
              </div>
            </div>
            <div>
              <p className="font-medium">Permission mode</p>
              <p className="text-muted-foreground">{executionSummary.permissionMode}</p>
            </div>
            <div>
              <p className="font-medium">Safety cues</p>
              <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                {executionSummary.safetyCues.length > 0 ? (
                  executionSummary.safetyCues.map((cue) => <li key={cue}>{cue}</li>)
                ) : (
                  <li>No additional safety cues configured</li>
                )}
              </ul>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
