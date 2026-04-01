"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { PanelLeftIcon, PanelRightIcon } from "lucide-react";
import { useBreakpoint } from "@/hooks/use-breakpoint";
import { cn } from "@/lib/utils";
import type {
  RoleManifest,
  RolePreviewResponse,
  RoleSandboxResponse,
  RoleSkillCatalogEntry,
} from "@/lib/stores/role-store";
import type { PluginRecord } from "@/lib/stores/plugin-store";
import {
  buildRoleCapabilitySourceFromDraft,
  buildRoleCapabilitySourceFromManifest,
  buildRoleDraft,
  buildRoleExecutionSummary,
  computeFieldProvenance,
  groupRoleDraftValidationErrors,
  resolveRoleSkillReferences,
  renderRoleManifestYaml,
  serializeRoleDraft,
  type RoleDraft,
  type RoleSkillResolution,
  type RoleKeyValueDraft,
  type RoleKnowledgeSourceDraft,
  type RoleMCPServerDraft,
  type RoleSkillDraft,
  type RoleTriggerDraft,
} from "@/lib/roles/role-management";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { RoleWorkspaceCatalog } from "./role-workspace-catalog";
import { RoleWorkspaceContextRail } from "./role-workspace-context-rail";
import { RoleWorkspaceEditor } from "./role-workspace-editor";
import type { RoleWorkspaceSectionId } from "./role-workspace-sections";

interface RoleWorkspaceProps {
  roles: RoleManifest[];
  skillCatalog?: RoleSkillCatalogEntry[];
  availablePlugins?: PluginRecord[];
  skillCatalogLoading?: boolean;
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

export function RoleWorkspace({
  roles,
  skillCatalog = [],
  availablePlugins = [],
  skillCatalogLoading = false,
  loading,
  error,
  onCreateRole,
  onUpdateRole,
  onDeleteRole,
  onPreviewRole,
  onSandboxRole,
}: RoleWorkspaceProps) {
  const t = useTranslations("roles");
  const { isDesktop, isMobile } = useBreakpoint();
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
  const sandboxInputRef = useRef("");

  // Panel visibility
  const [catalogOpen, setCatalogOpen] = useState(true);
  const [contextOpen, setContextOpen] = useState(true);
  const [catalogSheetOpen, setCatalogSheetOpen] = useState(false);
  const [contextSheetOpen, setContextSheetOpen] = useState(false);

  const selectedRole = useMemo(
    () => roles.find((role) => role.metadata.id === selectedRoleId),
    [roles, selectedRoleId],
  );
  const selectedTemplateRole = useMemo(
    () => roles.find((role) => role.metadata.id === templateId),
    [roles, templateId],
  );

  // Auto-collapse panels based on viewport width
  useEffect(() => {
    setCatalogOpen(isDesktop);
    setContextOpen(isDesktop);
  }, [isDesktop]);

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
  const draftValidationErrors = useMemo(
    () => serializedDraft.validationErrors ?? [],
    [serializedDraft],
  );
  const validationBySection = useMemo(
    () => groupRoleDraftValidationErrors(draftValidationErrors),
    [draftValidationErrors],
  );
  const executionSummary = useMemo(
    () => buildRoleExecutionSummary(draft, skillCatalog),
    [draft, skillCatalog],
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
    if (isMobile) setCatalogSheetOpen(false);
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

  const updatePrivateKnowledgeRow = (
    index: number,
    field: keyof RoleKnowledgeSourceDraft,
    value: RoleKnowledgeSourceDraft[keyof RoleKnowledgeSourceDraft],
  ) => {
    setDraft((current) => ({
      ...current,
      privateKnowledgeRows: current.privateKnowledgeRows.map((source, sourceIndex) =>
        sourceIndex === index ? { ...source, [field]: value } : source,
      ),
    }));
  };

  const updateMCPServerRow = (
    index: number,
    field: keyof RoleMCPServerDraft,
    value: RoleMCPServerDraft[keyof RoleMCPServerDraft],
  ) => {
    setDraft((current) => ({
      ...current,
      mcpServerRows: current.mcpServerRows.map((server, serverIndex) =>
        serverIndex === index ? { ...server, [field]: value } : server,
      ),
    }));
  };

  const updateCustomSettingRow = (
    index: number,
    field: keyof RoleKeyValueDraft,
    value: RoleKeyValueDraft[keyof RoleKeyValueDraft],
  ) => {
    setDraft((current) => ({
      ...current,
      customSettingRows: current.customSettingRows.map((setting, settingIndex) =>
        settingIndex === index ? { ...setting, [field]: value } : setting,
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
      setPreviewResult(
        (await onPreviewRole({
          roleId: mode === "edit" ? selectedRole?.metadata.id : undefined,
          draft: getRequestPayload(),
        })) ?? null,
      );
      if (isMobile) setContextSheetOpen(true);
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleSandbox = async () => {
    setSandboxLoading(true);
    try {
      setSandboxResult(
        (await onSandboxRole({
          roleId: mode === "edit" ? selectedRole?.metadata.id : undefined,
          draft: getRequestPayload(),
          input: sandboxInputRef.current,
        })) ?? null,
      );
      if (isMobile) setContextSheetOpen(true);
    } finally {
      setSandboxLoading(false);
    }
  };

  const selectedTemplateName =
    selectedTemplateRole?.metadata.name ?? null;
  const selectedParentRole = roles.find((role) => role.metadata.id === draft.extendsValue);
  const selectedParentName = selectedParentRole?.metadata.name ?? (draft.extendsValue || null);
  const draftSkillResolution = useMemo(
    () =>
      resolveRoleSkillReferences({
        skills: draft.skillRows,
        catalog: skillCatalog,
        templateSkills: selectedTemplateRole?.capabilities.skills ?? [],
        parentSkills: selectedParentRole?.capabilities.skills ?? [],
        roleCapabilities: buildRoleCapabilitySourceFromDraft(draft),
      }),
    [draft, selectedParentRole, selectedTemplateRole, skillCatalog],
  );
  const effectiveRoleCapabilitySource = useMemo(
    () =>
      sandboxResult?.effectiveManifest || previewResult?.effectiveManifest
        ? buildRoleCapabilitySourceFromManifest(
            sandboxResult?.effectiveManifest ?? previewResult?.effectiveManifest,
          )
        : buildRoleCapabilitySourceFromDraft(draft),
    [draft, previewResult, sandboxResult],
  );
  const effectiveSkillResolution = useMemo<RoleSkillResolution[]>(
    () =>
      resolveRoleSkillReferences({
        skills:
          sandboxResult?.effectiveManifest?.capabilities.skills ??
          previewResult?.effectiveManifest?.capabilities.skills ??
          draft.skillRows,
        catalog: skillCatalog,
        templateSkills: selectedTemplateRole?.capabilities.skills ?? [],
        parentSkills: selectedParentRole?.capabilities.skills ?? [],
        roleCapabilities: effectiveRoleCapabilitySource,
      }),
    [draft.skillRows, effectiveRoleCapabilitySource, previewResult, sandboxResult, selectedParentRole, selectedTemplateRole, skillCatalog],
  );

  const provenanceMap = useMemo(
    () => computeFieldProvenance(draft, selectedParentRole ?? null, selectedTemplateRole ?? null),
    [draft, selectedParentRole, selectedTemplateRole],
  );

  const editor = (
    <RoleWorkspaceEditor
      mode={mode}
      draft={draft}
      templateId={templateId}
      selectedRole={selectedRole}
      skillCatalog={skillCatalog}
      availablePlugins={availablePlugins}
      skillCatalogLoading={skillCatalogLoading}
      draftSkillResolution={draftSkillResolution}
      selectedTemplateName={selectedTemplateName}
      selectedParentName={selectedParentName}
      validationBySection={validationBySection}
      saving={saving}
      activeSection={activeSection}
      onSelectSection={setActiveSection}
      onSubmit={handleSave}
      onSwitchToCreate={handleNewRole}
      updateDraft={updateDraft}
      updateSkillRow={updateSkillRow}
      updateMCPServerRow={updateMCPServerRow}
      updateCustomSettingRow={updateCustomSettingRow}
      updateKnowledgeRow={updateKnowledgeRow}
      updatePrivateKnowledgeRow={updatePrivateKnowledgeRow}
      updateTriggerRow={updateTriggerRow}
      onAddSkillRow={() =>
        setDraft((current) => ({
          ...current,
          skillRows: [...current.skillRows, { path: "", autoLoad: false }],
        }))
      }
      onAddMCPServerRow={() =>
        setDraft((current) => ({
          ...current,
          mcpServerRows: [...current.mcpServerRows, { name: "", url: "" }],
        }))
      }
      onAddCustomSettingRow={() =>
        setDraft((current) => ({
          ...current,
          customSettingRows: [...current.customSettingRows, { key: "", value: "" }],
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
      onAddPrivateKnowledgeRow={() =>
        setDraft((current) => ({
          ...current,
          privateKnowledgeRows: [
            ...current.privateKnowledgeRows,
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
      provenanceMap={provenanceMap}
    />
  );

  const catalog = (
    <RoleWorkspaceCatalog
      roles={roles}
      skillCatalog={skillCatalog}
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
      effectiveSkillResolution={effectiveSkillResolution}
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
      provenanceMap={provenanceMap}
    />
  );

  const toggleCatalog = () => {
    if (isMobile) {
      setCatalogSheetOpen((v) => !v);
    } else {
      setCatalogOpen((v) => !v);
    }
  };

  const toggleContext = () => {
    if (isMobile) {
      setContextSheetOpen((v) => !v);
    } else {
      setContextOpen((v) => !v);
    }
  };

  return (
    <TooltipProvider>
    <div className="flex h-[calc(100vh-3.5rem)]">
      {/* Catalog panel: collapsible secondary sidebar */}
      {isMobile ? (
        <Sheet open={catalogSheetOpen} onOpenChange={setCatalogSheetOpen}>
          <SheetContent side="left" className="w-72 p-0" showCloseButton={false}>
            <SheetHeader className="sr-only">
              <SheetTitle>{t("roleLibrary")}</SheetTitle>
            </SheetHeader>
            {catalog}
          </SheetContent>
        </Sheet>
      ) : (
        <div
          className={cn(
            "shrink-0 overflow-hidden border-r bg-sidebar transition-[width] duration-200 ease-linear",
            catalogOpen ? "w-[260px]" : "w-0",
          )}
        >
          <div className="h-full w-[260px] overflow-y-auto">{catalog}</div>
        </div>
      )}

      {/* Editor: fills remaining space */}
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        {/* Toolbar with panel toggle buttons */}
        <div className="flex h-10 shrink-0 items-center gap-1 border-b bg-background px-2">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                onClick={toggleCatalog}
                aria-label={t("toggleCatalog")}
              >
                <PanelLeftIcon className="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{t("toggleCatalog")}</TooltipContent>
          </Tooltip>
          <div className="flex-1" />
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                onClick={toggleContext}
                aria-label={t("toggleContextRail")}
              >
                <PanelRightIcon className="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{t("toggleContextRail")}</TooltipContent>
          </Tooltip>
        </div>
        <div className="flex-1 overflow-y-auto">{editor}</div>
      </div>

      {/* Context rail: collapsible right panel */}
      {isMobile ? (
        <Sheet open={contextSheetOpen} onOpenChange={setContextSheetOpen}>
          <SheetContent side="right" className="w-80 p-0" showCloseButton={false}>
            <SheetHeader className="sr-only">
              <SheetTitle>{t("contextRail.authoringGuide")}</SheetTitle>
            </SheetHeader>
            {contextRail}
          </SheetContent>
        </Sheet>
      ) : (
        <div
          className={cn(
            "shrink-0 overflow-hidden border-l transition-[width] duration-200 ease-linear",
            contextOpen ? "w-80" : "w-0",
          )}
        >
          <div className="h-full w-80 overflow-y-auto">{contextRail}</div>
        </div>
      )}
    </div>
    </TooltipProvider>
  );
}
