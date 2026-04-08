import type { Project } from "@/lib/stores/project-store";
import type { SettingsWorkspaceDraft, SettingsValidationErrors } from "@/lib/settings/project-settings-workspace";

export interface ProjectSectionProps {
  draft: SettingsWorkspaceDraft;
  patchDraft: (updater: (d: SettingsWorkspaceDraft) => SettingsWorkspaceDraft) => void;
  validationErrors: SettingsValidationErrors;
  clearValidationError: (field: keyof SettingsValidationErrors) => void;
  project: Project;
}
