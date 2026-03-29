"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import type { RoleManifest } from "@/lib/stores/role-store";

interface RoleFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  role?: RoleManifest;
  availableRoles?: RoleManifest[];
  onSubmit: (data: Partial<RoleManifest>) => Promise<void>;
}

function parseList(input: string): string[] {
  return input
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function stringifyList(values?: string[]): string {
  return (values ?? []).join(", ");
}

function buildDraft(role?: RoleManifest) {
  return {
    roleId: role?.metadata.id ?? "",
    name: role?.metadata.name ?? "",
    description: role?.metadata.description ?? "",
    tagsInput: stringifyList(role?.metadata.tags),
    extendsValue: role?.extends ?? "",
    identityRole: role?.identity.role ?? "",
    goal: role?.identity.goal ?? "",
    backstory: role?.identity.backstory ?? "",
    systemPrompt: role?.identity.systemPrompt ?? "",
    allowedTools: stringifyList(role?.capabilities.allowedTools),
    skills: (role?.capabilities.skills ?? []).map((skill) => ({
      path: skill.path,
      autoLoad: skill.autoLoad,
    })),
    languages: stringifyList(role?.capabilities.languages),
    frameworks: stringifyList(role?.capabilities.frameworks),
    maxTurns:
      role?.capabilities.maxTurns != null ? String(role.capabilities.maxTurns) : "",
    maxBudgetUsd:
      role?.capabilities.maxBudgetUsd != null
        ? String(role.capabilities.maxBudgetUsd)
        : "",
    repositories: stringifyList(role?.knowledge.repositories),
    documents: stringifyList(role?.knowledge.documents),
    patterns: stringifyList(role?.knowledge.patterns),
    permissionMode: role?.security.permissionMode ?? "default",
    allowedPaths: stringifyList(role?.security.allowedPaths),
    deniedPaths: stringifyList(role?.security.deniedPaths),
    requireReview: role?.security.requireReview ?? false,
  };
}

export function RoleFormDialog({
  open,
  onOpenChange,
  role,
  availableRoles = [],
  onSubmit,
}: RoleFormDialogProps) {
  const t = useTranslations("roles");
  const [templateId, setTemplateId] = useState("");
  const [roleId, setRoleId] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [extendsValue, setExtendsValue] = useState("");
  const [identityRole, setIdentityRole] = useState("");
  const [goal, setGoal] = useState("");
  const [backstory, setBackstory] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [allowedTools, setAllowedTools] = useState("");
  const [skills, setSkills] = useState<Array<{ path: string; autoLoad: boolean }>>([]);
  const [languages, setLanguages] = useState("");
  const [frameworks, setFrameworks] = useState("");
  const [maxTurns, setMaxTurns] = useState("");
  const [maxBudgetUsd, setMaxBudgetUsd] = useState("");
  const [repositories, setRepositories] = useState("");
  const [documents, setDocuments] = useState("");
  const [patterns, setPatterns] = useState("");
  const [permissionMode, setPermissionMode] = useState("default");
  const [allowedPaths, setAllowedPaths] = useState("");
  const [deniedPaths, setDeniedPaths] = useState("");
  const [requireReview, setRequireReview] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const isEdit = !!role;

  useEffect(() => {
    if (!open) {
      return;
    }

    const draft = buildDraft(role);
    setTemplateId("");
    setRoleId(draft.roleId);
    setName(draft.name);
    setDescription(draft.description);
    setTagsInput(draft.tagsInput);
    setExtendsValue(draft.extendsValue);
    setIdentityRole(draft.identityRole);
    setGoal(draft.goal);
    setBackstory(draft.backstory);
    setSystemPrompt(draft.systemPrompt);
    setAllowedTools(draft.allowedTools);
    setSkills(draft.skills);
    setLanguages(draft.languages);
    setFrameworks(draft.frameworks);
    setMaxTurns(draft.maxTurns);
    setMaxBudgetUsd(draft.maxBudgetUsd);
    setRepositories(draft.repositories);
    setDocuments(draft.documents);
    setPatterns(draft.patterns);
    setPermissionMode(draft.permissionMode);
    setAllowedPaths(draft.allowedPaths);
    setDeniedPaths(draft.deniedPaths);
    setRequireReview(draft.requireReview);
  }, [open, role]);

  const templateOptions = useMemo(
    () =>
      availableRoles.filter((item) =>
        role ? item.metadata.id !== role.metadata.id : true,
      ),
    [availableRoles, role],
  );

  const applyTemplate = (templateRoleId: string) => {
    setTemplateId(templateRoleId);
    const selectedTemplate = availableRoles.find(
      (item) => item.metadata.id === templateRoleId,
    );
    if (!selectedTemplate) {
      return;
    }

    const draft = buildDraft(selectedTemplate);
    setName(draft.name);
    setDescription(draft.description);
    setTagsInput(draft.tagsInput);
    setIdentityRole(draft.identityRole);
    setGoal(draft.goal);
    setBackstory(draft.backstory);
    setSystemPrompt(draft.systemPrompt);
    setAllowedTools(draft.allowedTools);
    setSkills(draft.skills);
    setLanguages(draft.languages);
    setFrameworks(draft.frameworks);
    setMaxTurns(draft.maxTurns);
    setMaxBudgetUsd(draft.maxBudgetUsd);
    setRepositories(draft.repositories);
    setDocuments(draft.documents);
    setPatterns(draft.patterns);
    setPermissionMode(draft.permissionMode);
    setAllowedPaths(draft.allowedPaths);
    setDeniedPaths(draft.deniedPaths);
    setRequireReview(draft.requireReview);
    if (!roleId) {
      setRoleId(`${selectedTemplate.metadata.id}-copy`);
    }
  };

  const tagPreview = parseList(tagsInput);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    try {
      await onSubmit({
        metadata: {
          ...(role?.metadata ?? {
            id: roleId,
            version: "1.0.0",
            author: "AgentForge",
          }),
          id: roleId,
          name,
          description,
          author: role?.metadata.author ?? "AgentForge",
          version: role?.metadata.version ?? "1.0.0",
          tags: parseList(tagsInput),
        },
        identity: {
          ...(role?.identity ?? {
            persona: "",
            goals: [],
            constraints: [],
          }),
          role: identityRole,
          goal,
          backstory,
          systemPrompt,
        },
        capabilities: {
          ...(role?.capabilities ?? {
            languages: [],
            frameworks: [],
          }),
          allowedTools: parseList(allowedTools),
          skills,
          languages: parseList(languages),
          frameworks: parseList(frameworks),
          maxTurns: maxTurns ? Number(maxTurns) : undefined,
          maxBudgetUsd: maxBudgetUsd ? Number(maxBudgetUsd) : undefined,
        },
        knowledge: {
          ...(role?.knowledge ?? {
            repositories: [],
            documents: [],
            patterns: [],
          }),
          repositories: parseList(repositories),
          documents: parseList(documents),
          patterns: parseList(patterns),
        },
        security: {
          ...(role?.security ?? {
            allowedPaths: [],
            deniedPaths: [],
            maxBudgetUsd: 0,
          }),
          permissionMode,
          allowedPaths: parseList(allowedPaths),
          deniedPaths: parseList(deniedPaths),
          maxBudgetUsd: maxBudgetUsd ? Number(maxBudgetUsd) : 0,
          requireReview,
        },
        extends: extendsValue || undefined,
      });
      onOpenChange(false);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{isEdit ? t("formDialog.editTitle") : t("formDialog.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("formDialog.desc")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex max-h-[75vh] flex-col gap-5 overflow-y-auto pr-1">
          <section className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="role-template">{t("formDialog.startFromTemplate")}</Label>
              <select
                id="role-template"
                aria-label="Start from template"
                className="h-10 rounded-md border bg-background px-3 text-sm"
                value={templateId}
                onChange={(event) => applyTemplate(event.target.value)}
                disabled={isEdit}
              >
                <option value="">{t("formDialog.blankRole")}</option>
                {templateOptions.map((item) => (
                  <option key={item.metadata.id} value={item.metadata.id}>
                    {item.metadata.name}
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
                value={extendsValue}
                onChange={(event) => setExtendsValue(event.target.value)}
              >
                <option value="">{t("formDialog.noParent")}</option>
                {templateOptions.map((item) => (
                  <option key={item.metadata.id} value={item.metadata.id}>
                    {item.metadata.name}
                  </option>
                ))}
              </select>
            </div>
          </section>

          <section className="grid gap-4 rounded-lg border p-4">
            <h3 className="text-sm font-semibold">{t("formDialog.identity")}</h3>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-id">{t("formDialog.roleId")}</Label>
                <Input
                  id="role-id"
                  aria-label="Role ID"
                  value={roleId}
                  onChange={(event) => setRoleId(event.target.value)}
                  placeholder="frontend-developer"
                  required
                  disabled={isEdit}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-name">{t("formDialog.name")}</Label>
                <Input
                  id="role-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder="Frontend Developer"
                  required
                />
              </div>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="role-description">{t("formDialog.description")}</Label>
              <Input
                id="role-description"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="What does this role do?"
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="role-tags">{t("formDialog.tags")}</Label>
              <Input
                id="role-tags"
                value={tagsInput}
                onChange={(event) => setTagsInput(event.target.value)}
                placeholder="frontend, react, nextjs"
              />
              {tagPreview.length > 0 ? (
                <div className="flex flex-wrap gap-1">
                  {tagPreview.map((tag) => (
                    <Badge key={tag} variant="secondary" className="text-xs">
                      {tag}
                    </Badge>
                  ))}
                </div>
              ) : null}
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="identity-role">{t("formDialog.role")}</Label>
                <Input
                  id="identity-role"
                  value={identityRole}
                  onChange={(event) => setIdentityRole(event.target.value)}
                  placeholder="Senior Frontend Developer"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="identity-goal">{t("formDialog.goal")}</Label>
                <Input
                  id="identity-goal"
                  value={goal}
                  onChange={(event) => setGoal(event.target.value)}
                  placeholder="Build responsive UI"
                />
              </div>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="identity-backstory">{t("formDialog.backstory")}</Label>
              <textarea
                id="identity-backstory"
                className="min-h-[84px] rounded-md border bg-background px-3 py-2 text-sm"
                value={backstory}
                onChange={(event) => setBackstory(event.target.value)}
                placeholder="Explain the role context..."
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="role-system-prompt">{t("formDialog.systemPrompt")}</Label>
              <textarea
                id="role-system-prompt"
                className="min-h-[120px] rounded-md border bg-background px-3 py-2 text-sm"
                value={systemPrompt}
                onChange={(event) => setSystemPrompt(event.target.value)}
                placeholder="System prompt for the role..."
              />
            </div>
          </section>

          <section className="grid gap-4 rounded-lg border p-4">
            <h3 className="text-sm font-semibold">{t("formDialog.capabilities")}</h3>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-allowed-tools">{t("formDialog.allowedTools")}</Label>
                <Input
                  id="role-allowed-tools"
                  aria-label="Allowed Tools"
                  value={allowedTools}
                  onChange={(event) => setAllowedTools(event.target.value)}
                  placeholder="Read, Edit, Bash"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-languages">{t("formDialog.languages")}</Label>
                <Input
                  id="role-languages"
                  value={languages}
                  onChange={(event) => setLanguages(event.target.value)}
                  placeholder="TypeScript, Go"
                />
              </div>
            </div>
            <div className="grid gap-4 md:grid-cols-3">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-frameworks">{t("formDialog.frameworks")}</Label>
                <Input
                  id="role-frameworks"
                  value={frameworks}
                  onChange={(event) => setFrameworks(event.target.value)}
                  placeholder="Next.js, Echo"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-max-turns">{t("formDialog.maxTurns")}</Label>
                <Input
                  id="role-max-turns"
                  type="number"
                  min="1"
                  value={maxTurns}
                  onChange={(event) => setMaxTurns(event.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-max-budget">{t("formDialog.maxBudget")}</Label>
                <Input
                  id="role-max-budget"
                  type="number"
                  min="0"
                  step="0.01"
                  value={maxBudgetUsd}
                  onChange={(event) => setMaxBudgetUsd(event.target.value)}
                />
              </div>
            </div>
          </section>

          <section className="grid gap-4 rounded-lg border p-4">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold">{t("formDialog.skillsTitle")}</h3>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setSkills((current) => [...current, { path: "", autoLoad: false }])}
              >
                {t("formDialog.addSkill")}
              </Button>
            </div>
            {skills.length > 0 ? (
              <div className="grid gap-3">
                {skills.map((skill, index) => (
                  <div
                    key={`dialog-skill-${index}`}
                    className="grid gap-3 rounded-md border p-3 md:grid-cols-[minmax(0,1fr)_auto_auto]"
                  >
                    <div className="flex flex-col gap-1.5">
                      <Label htmlFor={`dialog-skill-path-${index}`}>{t("formDialog.skillPath")}</Label>
                      <Input
                        id={`dialog-skill-path-${index}`}
                        aria-label="Skill Path"
                        value={skill.path}
                        onChange={(event) =>
                          setSkills((current) =>
                            current.map((item, itemIndex) =>
                              itemIndex === index ? { ...item, path: event.target.value } : item,
                            ),
                          )
                        }
                        placeholder="skills/react"
                      />
                    </div>
                    <div className="flex items-end gap-2">
                      <input
                        id={`dialog-skill-auto-${index}`}
                        type="checkbox"
                        checked={skill.autoLoad}
                        onChange={(event) =>
                          setSkills((current) =>
                            current.map((item, itemIndex) =>
                              itemIndex === index
                                ? { ...item, autoLoad: event.target.checked }
                                : item,
                            ),
                          )
                        }
                        className="size-4 rounded border-input"
                      />
                      <Label htmlFor={`dialog-skill-auto-${index}`}>{t("formDialog.autoLoadSkill")}</Label>
                    </div>
                    <div className="flex items-end justify-end">
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() =>
                          setSkills((current) => current.filter((_, itemIndex) => itemIndex !== index))
                        }
                      >
                        {t("formDialog.remove")}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">{t("formDialog.noSkills")}</p>
            )}
          </section>

          <section className="grid gap-4 rounded-lg border p-4">
            <h3 className="text-sm font-semibold">{t("formDialog.knowledge")}</h3>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="role-repositories">{t("formDialog.repositories")}</Label>
              <Input
                id="role-repositories"
                value={repositories}
                onChange={(event) => setRepositories(event.target.value)}
                placeholder="app, components"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="role-documents">{t("formDialog.documents")}</Label>
              <Input
                id="role-documents"
                value={documents}
                onChange={(event) => setDocuments(event.target.value)}
                placeholder="docs/PRD.md, docs/part/PLUGIN_SYSTEM_DESIGN.md"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="role-patterns">{t("formDialog.patterns")}</Label>
              <Input
                id="role-patterns"
                value={patterns}
                onChange={(event) => setPatterns(event.target.value)}
                placeholder="rsc, task-workspace"
              />
            </div>
          </section>

          <section className="grid gap-4 rounded-lg border p-4">
            <h3 className="text-sm font-semibold">{t("formDialog.security")}</h3>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-permission-mode">{t("formDialog.permissionMode")}</Label>
                <select
                  id="role-permission-mode"
                  aria-label="Permission Mode"
                  className="h-10 rounded-md border bg-background px-3 text-sm"
                  value={permissionMode}
                  onChange={(event) => setPermissionMode(event.target.value)}
                >
                  <option value="default">default</option>
                  <option value="acceptEdits">acceptEdits</option>
                  <option value="bypassPermissions">bypassPermissions</option>
                </select>
              </div>
              <div className="flex items-end gap-2">
                <input
                  id="role-require-review"
                  type="checkbox"
                  checked={requireReview}
                  onChange={(event) => setRequireReview(event.target.checked)}
                  className="size-4 rounded border-input"
                />
                <Label htmlFor="role-require-review">{t("formDialog.requireReview")}</Label>
              </div>
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-allowed-paths">{t("formDialog.allowedPaths")}</Label>
                <Input
                  id="role-allowed-paths"
                  value={allowedPaths}
                  onChange={(event) => setAllowedPaths(event.target.value)}
                  placeholder="app/, components/"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-denied-paths">{t("formDialog.deniedPaths")}</Label>
                <Input
                  id="role-denied-paths"
                  value={deniedPaths}
                  onChange={(event) => setDeniedPaths(event.target.value)}
                  placeholder="secrets/, keys/"
                />
              </div>
            </div>
          </section>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("formDialog.cancel")}
            </Button>
            <Button type="submit" disabled={submitting || !name || !roleId}>
              {submitting ? t("formDialog.saving") : isEdit ? t("formDialog.update") : t("formDialog.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
