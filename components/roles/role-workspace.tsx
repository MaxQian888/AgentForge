"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type {
  RoleManifest,
  RolePreviewResponse,
  RoleSandboxResponse,
} from "@/lib/stores/role-store";
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
import { Button } from "@/components/ui/button";
import { RoleWorkspaceCatalog } from "./role-workspace-catalog";
import { RoleWorkspaceContextRail } from "./role-workspace-context-rail";
import { RoleWorkspaceEditor } from "./role-workspace-editor";
import type { RoleWorkspaceSectionId } from "./role-workspace-sections";

interface RoleWorkspaceProps {
  roles: RoleManifest[];
  loading: boolean;
  error: string | null;
  onCreateRole: (data: Partial<RoleManifest>) => Promise<unknown>;
  onUpdateRole: (id: string, data: Partial<RoleManifest>) => Promise<unknown>;
  onDeleteRole: (role: RoleManifest) => Promise<unknown>;
  onPreviewRole: (payload: {
    roleId?: string;
    draft?: Partial<RoleManifest>;
  }) => Promise<RolePreviewResponse | void>;
  onSandboxRole: (payload: {
    roleId?: string;
    draft?: Partial<RoleManifest>;
    input: string;
  }) => Promise<RoleSandboxResponse | void>;
}

type RoleWorkspaceLayout = "desktop" | "medium" | "narrow";
type CompactPanel = "none" | "catalog" | "review";

function getLayout(): RoleWorkspaceLayout {
  if (typeof window === "undefined") {
    return "desktop";
  }
  if (window.innerWidth >= 1280) {
    return "desktop";
  }
  if (window.innerWidth >= 768) {
    return "medium";
  }
  return "narrow";
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
  const [activeSection, setActiveSection] =
    useState<RoleWorkspaceSectionId>("setup");
  const [layout, setLayout] = useState<RoleWorkspaceLayout>(() => getLayout());
  const [compactPanel, setCompactPanel] = useState<CompactPanel>("none");
  const sandboxInputRef = useRef("");

  const selectedRole = useMemo(
    () => roles.find((role) => role.metadata.id === selectedRoleId),
    [roles, selectedRoleId],
  );

  useEffect(() => {
    const handleResize = () => {
      const nextLayout = getLayout();
      setLayout(nextLayout);
      if (nextLayout === "desktop") {
        setCompactPanel("none");
      }
    };

    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

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

  const serializedDraft = useMemo(
    () => serializeRoleDraft(draft, selectedRole),
    [draft, selectedRole],
  );
  const draftValidationErrors = serializedDraft.validationErrors ?? [];
  const executionSummary = useMemo(
    () => buildRoleExecutionSummary(draft),
    [draft],
  );

  const getRequestPayload = () => {
    const payload = { ...serializedDraft };
    delete payload.validationErrors;
    return payload;
  };

  const yamlPreview = renderRoleManifestYaml(
    previewResult?.effectiveManifest ?? getRequestPayload(),
  );

  const updateDraft = <K extends keyof RoleDraft>(key: K, value: RoleDraft[K]) =>
    setDraft((current) => ({ ...current, [key]: value }));

  const handleNewRole = () => {
    setMode("create");
    setSelectedRoleId("");
    setTemplateId("");
    setDraft(buildRoleDraft());
    setPreviewResult(null);
    setSandboxResult(null);
    setActiveSection("setup");
  };

  const handleEditRole = (role: RoleManifest) => {
    setMode("edit");
    setTemplateId("");
    setSelectedRoleId(role.metadata.id);
    setDraft(buildRoleDraft(role));
    setPreviewResult(null);
    setSandboxResult(null);
    setActiveSection("setup");
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

  const updateSkillRow = (
    index: number,
    field: keyof RoleSkillDraft,
    value: RoleSkillDraft[keyof RoleSkillDraft],
  ) => {
    setDraft((current) => ({
      ...current,
      skillRows: current.skillRows.map((skill, skillIndex) =>
        skillIndex === index ? { ...skill, [field]: value } : skill,
      ),
    }));
  };

  const updateKnowledgeRow = (
    index: number,
    field: keyof RoleKnowledgeSourceDraft,
    value: RoleKnowledgeSourceDraft[keyof RoleKnowledgeSourceDraft],
  ) => {
    setDraft((current) => ({
      ...current,
      sharedKnowledgeRows: current.sharedKnowledgeRows.map((source, sourceIndex) =>
        sourceIndex === index ? { ...source, [field]: value } : source,
      ),
    }));
  };

  const updateTriggerRow = (
    index: number,
    field: keyof RoleTriggerDraft,
    value: RoleTriggerDraft[keyof RoleTriggerDraft],
  ) => {
    setDraft((current) => ({
      ...current,
      triggerRows: current.triggerRows.map((trigger, triggerIndex) =>
        triggerIndex === index ? { ...trigger, [field]: value } : trigger,
      ),
    }));
  };

  const handleSave = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSaving(true);
    try {
      if (draftValidationErrors.length > 0) {
        return;
      }
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
      if (layout !== "desktop") {
        setCompactPanel("review");
      }
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleSandbox = async () => {
    setSandboxLoading(true);
    try {
      setSandboxResult(
        (await onSandboxRole({
          draft: getRequestPayload(),
          input: sandboxInputRef.current,
        })) ?? null,
      );
      if (layout !== "desktop") {
        setCompactPanel("review");
      }
    } finally {
      setSandboxLoading(false);
    }
  };

  const selectedTemplateName =
    roles.find((role) => role.metadata.id === templateId)?.metadata.name ?? null;
  const selectedParentName =
    roles.find((role) => role.metadata.id === draft.extendsValue)?.metadata.name ??
    (draft.extendsValue || null);

  const editor = (
    <RoleWorkspaceEditor
      mode={mode}
      draft={draft}
      templateId={templateId}
      selectedRole={selectedRole}
      selectedTemplateName={selectedTemplateName}
      selectedParentName={selectedParentName}
      validationErrors={draftValidationErrors}
      saving={saving}
      activeSection={activeSection}
      onSelectSection={setActiveSection}
      onSubmit={handleSave}
      onSwitchToCreate={handleNewRole}
      updateDraft={updateDraft}
      updateSkillRow={updateSkillRow}
      updateKnowledgeRow={updateKnowledgeRow}
      updateTriggerRow={updateTriggerRow}
      onAddSkillRow={() =>
        setDraft((current) => ({
          ...current,
          skillRows: [...current.skillRows, { path: "", autoLoad: false }],
        }))
      }
      onAddKnowledgeRow={() =>
        setDraft((current) => ({
          ...current,
          sharedKnowledgeRows: [
            ...current.sharedKnowledgeRows,
            { id: "", type: "", access: "", description: "", sourcesInput: "" },
          ],
        }))
      }
      onAddTriggerRow={() =>
        setDraft((current) => ({
          ...current,
          triggerRows: [
            ...current.triggerRows,
            { event: "", action: "", condition: "" },
          ],
        }))
      }
      availableRoles={roles}
      onTemplateChange={handleTemplateChange}
    />
  );

  const catalog = (
    <RoleWorkspaceCatalog
      roles={roles}
      loading={loading}
      error={error}
      onCreateNew={handleNewRole}
      onEditRole={handleEditRole}
      onDeleteRole={(role) => void onDeleteRole(role)}
    />
  );

  const contextRail = (
    <RoleWorkspaceContextRail
      activeSection={activeSection}
      executionSummary={executionSummary}
      yamlPreview={yamlPreview}
      previewLoading={previewLoading}
      sandboxLoading={sandboxLoading}
      sandboxInput={sandboxInput}
      onSandboxInputChange={(value) => {
        setSandboxInput(value);
        sandboxInputRef.current = value;
      }}
      onPreview={() => void handlePreview()}
      onSandbox={() => void handleSandbox()}
      previewResult={previewResult}
      sandboxResult={sandboxResult}
    />
  );

  if (layout === "desktop") {
    return (
      <div className="grid gap-6 xl:grid-cols-[minmax(260px,0.95fr)_minmax(0,1.35fr)_minmax(320px,0.9fr)]">
        {catalog}
        {editor}
        {contextRail}
      </div>
    );
  }

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          variant={compactPanel === "catalog" ? "default" : "outline"}
          onClick={() =>
            setCompactPanel((current) => (current === "catalog" ? "none" : "catalog"))
          }
        >
          Show Role Library
        </Button>
        <Button
          type="button"
          variant={compactPanel === "review" ? "default" : "outline"}
          onClick={() =>
            setCompactPanel((current) => (current === "review" ? "none" : "review"))
          }
        >
          Show Review Panel
        </Button>
      </div>

      {editor}

      {compactPanel === "catalog" ? catalog : null}
      {compactPanel === "review" ? contextRail : null}
    </div>
  );
}
