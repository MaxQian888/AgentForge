"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { FolderOpen } from "lucide-react";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useProjectStore, type Project } from "@/lib/stores/project-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { EmptyState } from "@/components/shared/empty-state";
import {
  areSettingsDraftsEqual,
  createSettingsWorkspaceDraft,
  extractServerError,
  preserveRedactedWebhookSecret,
  toProjectUpdateInput,
  validateSettingsWorkspaceDraft,
  type SettingsValidationErrors,
  type SettingsWorkspaceDraft,
} from "@/lib/settings/project-settings-workspace";
import { SettingsShell } from "./_components/settings-shell";
import { SectionAppearance } from "./_components/section-appearance";
import { SectionAccessibility } from "./_components/section-accessibility";
import { SectionApiConnection } from "./_components/section-api-connection";
import { SectionIMBridge } from "./_components/section-im-bridge";
import { SectionGeneral } from "./_components/section-general";
import { SectionRepository } from "./_components/section-repository";
import { SectionCodingAgent } from "./_components/section-coding-agent";
import { SectionBudget } from "./_components/section-budget";
import { SectionReviewPolicy } from "./_components/section-review-policy";
import { SectionWebhook } from "./_components/section-webhook";
import { SectionCustomFields } from "./_components/section-custom-fields";
import { SectionForms } from "./_components/section-forms";
import { SectionAutomations } from "./_components/section-automations";
import { SectionAdvanced } from "./_components/section-advanced";
import { SectionAuditLog } from "./_components/section-audit-log";
import { SectionRuntimeDetail } from "./_components/section-runtime-detail";

const RUNTIME_KEYS = ["claude_code", "codex", "opencode", "cursor", "gemini", "qoder", "iflow"];

type SaveState = "idle" | "saving" | "saved" | "error";

function SettingsOrchestrator({
  project,
  initialSection,
}: {
  project: Project;
  initialSection?: string | null;
}) {
  const t = useTranslations("settings");
  const { updateProject } = useProjectStore();
  const [activeSection, setActiveSection] = useState(initialSection ?? "appearance");
  const [persistedSnapshot, setPersistedSnapshot] = useState(() => createSettingsWorkspaceDraft(project));
  const [draft, setDraft] = useState(() => createSettingsWorkspaceDraft(project));
  const [validationErrors, setValidationErrors] = useState<SettingsValidationErrors>({});
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const savedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (savedTimeoutRef.current) clearTimeout(savedTimeoutRef.current);
    };
  }, []);

  const dirty = useMemo(() => !areSettingsDraftsEqual(draft, persistedSnapshot), [draft, persistedSnapshot]);

  const clearValidationError = (field: keyof SettingsValidationErrors) => {
    setValidationErrors((c) => (c[field] ? { ...c, [field]: undefined } : c));
  };

  const patchDraft = (updater: (d: SettingsWorkspaceDraft) => SettingsWorkspaceDraft) => {
    setDraft((c) => updater(c));
    setSaveError(null);
    if (saveState === "error") setSaveState("idle");
  };

  const handleDiscard = () => {
    setDraft(persistedSnapshot);
    setValidationErrors({});
    setSaveError(null);
    setSaveState("idle");
  };

  const handleSave = async () => {
    const errors = validateSettingsWorkspaceDraft(draft, project.codingAgentCatalog);
    if (Object.values(errors).some(Boolean)) {
      setValidationErrors(errors);
      setSaveState("error");
      setSaveError(t("validationSummary"));
      return;
    }

    setValidationErrors({});
    setSaveError(null);
    setSaveState("saving");
    try {
      const input = toProjectUpdateInput(draft);
      const updatedRaw =
        (await updateProject(project.id, input)) ??
        ({ ...project, ...input, settings: input.settings ?? project.settings } as Project);
      const updated = preserveRedactedWebhookSecret(updatedRaw, draft.settings.webhook.secret);
      const next = createSettingsWorkspaceDraft(updated);
      setPersistedSnapshot(next);
      setDraft(next);
      void useDashboardStore.getState().fetchSummary({ projectId: project.id });
      setSaveState("saved");
      if (savedTimeoutRef.current) clearTimeout(savedTimeoutRef.current);
      savedTimeoutRef.current = setTimeout(() => {
        setSaveState("idle");
        savedTimeoutRef.current = null;
      }, 2000);
    } catch (error) {
      const serverError = extractServerError(error);
      setValidationErrors((c) => ({ ...c, ...serverError.fieldErrors }));
      setSaveError(serverError.message);
      setSaveState("error");
    }
  };

  const projectProps = {
    draft,
    patchDraft,
    validationErrors,
    clearValidationError,
    project,
  };

  const renderSection = () => {
    // Handle runtime-* sections
    if (activeSection.startsWith("runtime-")) {
      const runtimeKey = activeSection.replace("runtime-", "");
      if (RUNTIME_KEYS.includes(runtimeKey)) {
        return <SectionRuntimeDetail runtimeKey={runtimeKey} />;
      }
    }

    switch (activeSection) {
      case "appearance":
        return <SectionAppearance />;
      case "accessibility":
        return <SectionAccessibility />;
      case "api-connection":
        return <SectionApiConnection />;
      case "im-bridge":
        return <SectionIMBridge />;
      case "general":
        return <SectionGeneral {...projectProps} />;
      case "repository":
        return <SectionRepository {...projectProps} />;
      case "coding-agent":
        return <SectionCodingAgent {...projectProps} />;
      case "budget":
        return <SectionBudget {...projectProps} />;
      case "review-policy":
        return <SectionReviewPolicy {...projectProps} />;
      case "webhook":
        return <SectionWebhook {...projectProps} />;
      case "custom-fields":
        return <SectionCustomFields {...projectProps} />;
      case "forms":
        return <SectionForms {...projectProps} />;
      case "automations":
        return <SectionAutomations {...projectProps} />;
      case "audit-log":
        return <SectionAuditLog projectId={project.id} />;
      case "advanced":
        return <SectionAdvanced {...projectProps} />;
      default:
        return <SectionAppearance />;
    }
  };

  return (
    <SettingsShell
      activeSection={activeSection}
      onSectionChange={setActiveSection}
      hasProject
      dirty={dirty}
      saveState={saveState}
      saveError={saveError}
      onSave={() => void handleSave()}
      onDiscard={handleDiscard}
    >
      {renderSection()}
    </SettingsShell>
  );
}

function SettingsPageNoProject({
  initialSection,
}: {
  initialSection?: string | null;
} = {}) {
  const t = useTranslations("settings");
  const [activeSection, setActiveSection] = useState(initialSection ?? "appearance");

  const renderSection = () => {
    // Handle runtime-* sections (app-level, no project needed)
    if (activeSection.startsWith("runtime-")) {
      const runtimeKey = activeSection.replace("runtime-", "");
      if (RUNTIME_KEYS.includes(runtimeKey)) {
        return <SectionRuntimeDetail runtimeKey={runtimeKey} />;
      }
    }

    switch (activeSection) {
      case "appearance":
        return <SectionAppearance />;
      case "accessibility":
        return <SectionAccessibility />;
      case "api-connection":
        return <SectionApiConnection />;
      case "im-bridge":
        return <SectionIMBridge />;
      default:
        return (
          <EmptyState
            icon={FolderOpen}
            title={t("titleNoProject")}
            description={t("selectProject")}
          />
        );
    }
  };

  return (
    <SettingsShell
      activeSection={activeSection}
      onSectionChange={setActiveSection}
      hasProject={false}
      dirty={false}
      saveState="idle"
      saveError={null}
      onSave={() => {}}
      onDiscard={() => {}}
    >
      {renderSection()}
    </SettingsShell>
  );
}

function SettingsPageContent() {
  useBreadcrumbs([{ label: "Configuration", href: "/" }, { label: "Settings" }]);
  const searchParams = useSearchParams();
  const requestedProjectId = searchParams.get("project");
  const requestedSection = searchParams.get("section");
  const { selectedProjectId } = useDashboardStore();
  const { projects, fetchProjects } = useProjectStore();
  const activeProjectId = requestedProjectId ?? selectedProjectId;
  const project = projects.find((p) => p.id === activeProjectId);

  useEffect(() => {
    void fetchProjects();
  }, [fetchProjects]);

  if (!activeProjectId || !project) {
    return <SettingsPageNoProject initialSection={requestedSection} />;
  }

  return (
    <SettingsOrchestrator
      key={`${project.id}:${requestedSection ?? "appearance"}`}
      project={project}
      initialSection={requestedSection}
    />
  );
}

export default function SettingsPage() {
  return (
    <Suspense fallback={<SettingsPageNoProject />}>
      <SettingsPageContent />
    </Suspense>
  );
}
