"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type {
  RolePreviewResponse,
  RoleSandboxResponse,
} from "@/lib/stores/role-store";
import type {
  RoleExecutionSummary,
  RoleSkillResolution,
} from "@/lib/roles/role-management";
import {
  ROLE_WORKSPACE_GUIDANCE,
  ROLE_WORKSPACE_SECTIONS,
  type RoleWorkspaceSectionId,
} from "./role-workspace-sections";

interface RoleWorkspaceContextRailProps {
  activeSection: RoleWorkspaceSectionId;
  executionSummary: RoleExecutionSummary;
  effectiveSkillResolution: RoleSkillResolution[];
  yamlPreview: string;
  previewLoading: boolean;
  sandboxLoading: boolean;
  sandboxInput: string;
  onSandboxInputChange: (value: string) => void;
  onPreview: () => void;
  onSandbox: () => void;
  previewResult: RolePreviewResponse | null;
  sandboxResult: RoleSandboxResponse | null;
}

export function RoleWorkspaceContextRail({
  activeSection,
  executionSummary,
  effectiveSkillResolution,
  yamlPreview,
  previewLoading,
  sandboxLoading,
  sandboxInput,
  onSandboxInputChange,
  onPreview,
  onSandbox,
  previewResult,
  sandboxResult,
}: RoleWorkspaceContextRailProps) {
  const t = useTranslations("roles");
  const activeSectionLabel =
    ROLE_WORKSPACE_SECTIONS.find((section) => section.id === activeSection)?.label ??
    "Review";
  const guidance = ROLE_WORKSPACE_GUIDANCE[activeSection];
  const effectiveManifest =
    sandboxResult?.effectiveManifest ?? previewResult?.effectiveManifest;
  const inheritanceParent =
    sandboxResult?.inheritance?.parentRoleId ?? previewResult?.inheritance?.parentRoleId;
  const advancedSettings = [
    effectiveManifest?.capabilities?.customSettings
      ? `custom_settings (${Object.keys(effectiveManifest.capabilities.customSettings).length})`
      : null,
    effectiveManifest?.capabilities?.toolConfig?.mcpServers?.length
      ? `mcp_servers (${effectiveManifest.capabilities.toolConfig.mcpServers.length})`
      : null,
    effectiveManifest?.knowledge?.memory ? "knowledge.memory" : null,
    effectiveManifest?.overrides ? "overrides" : null,
  ].filter((item): item is string => item != null);
  const storedOnlyFields = [
    effectiveManifest?.knowledge?.memory ? "knowledge.memory" : null,
    effectiveManifest?.collaboration ? "collaboration" : null,
    effectiveManifest?.triggers?.length ? "triggers" : null,
    effectiveManifest?.overrides ? "overrides" : null,
  ].filter((item): item is string => item != null);
  const runtimeExecutionProfile =
    sandboxResult?.executionProfile ?? previewResult?.executionProfile;
  const readinessDiagnostics =
    sandboxResult?.readinessDiagnostics ?? previewResult?.readinessDiagnostics ?? [];
  const validationIssues =
    sandboxResult?.validationIssues && sandboxResult.validationIssues.length > 0
      ? sandboxResult.validationIssues
      : (previewResult?.validationIssues ?? []);

  return (
    <section className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("contextRail.authoringGuide")}</CardTitle>
          <CardDescription>
            {t("contextRail.guidanceFor", { section: activeSectionLabel.toLowerCase() })}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm text-muted-foreground">
          <div>
            <p className="font-medium text-foreground">{guidance.title}</p>
            <p>{guidance.summary}</p>
          </div>
          <ul className="list-disc space-y-1 pl-5">
            {guidance.bullets.map((bullet) => (
              <li key={bullet}>{bullet}</li>
            ))}
          </ul>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("contextRail.executionSummary")}</CardTitle>
          <CardDescription>
            {t("contextRail.executionSummaryDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm">
          <div>
            <p className="font-medium">{t("contextRail.promptIntent")}</p>
            <p className="text-muted-foreground">
              {executionSummary.promptIntent || t("contextRail.noPromptIntent")}
            </p>
          </div>
          <div>
            <p className="font-medium">{t("contextRail.allowedTools")}</p>
            <p className="text-muted-foreground">{executionSummary.toolsLabel}</p>
          </div>
          <div>
            <p className="font-medium">{t("contextRail.skills")}</p>
            <p className="text-muted-foreground">{executionSummary.skillsLabel}</p>
            <p className="text-muted-foreground">
              {executionSummary.keySkillPaths.length > 0
                ? executionSummary.keySkillPaths.join(", ")
                : t("contextRail.noKeySkills")}
            </p>
          </div>
          <div>
            <p className="font-medium">{t("contextRail.budget")}</p>
            <p className="text-muted-foreground">{executionSummary.budgetLabel}</p>
          </div>
          <div>
            <p className="font-medium">{t("contextRail.turnLimit")}</p>
            <p className="text-muted-foreground">{executionSummary.turnsLabel}</p>
          </div>
          <div>
            <p className="font-medium">{t("contextRail.permissionMode")}</p>
            <p className="text-muted-foreground">{executionSummary.permissionMode}</p>
          </div>
          <div>
            <p className="font-medium">{t("contextRail.safetyCues")}</p>
            <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
              {executionSummary.safetyCues.length > 0 ? (
                executionSummary.safetyCues.map((cue) => <li key={cue}>{cue}</li>)
              ) : (
                <li>{t("contextRail.noSafetyCues")}</li>
              )}
            </ul>
          </div>
          <div>
            <p className="font-medium">{t("contextRail.skillResolution")}</p>
            <p className="text-muted-foreground">{t("contextRail.skillResolutionDesc")}</p>
            {effectiveSkillResolution.length > 0 ? (
              <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                {effectiveSkillResolution.map((skill, index) => (
                  <li key={`${skill.path}:${skill.provenance}:${index}`}>
                    {skill.label} ({skill.path}) - {skill.status} / {skill.provenance}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-muted-foreground">{t("contextRail.skillResolutionNone")}</p>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("contextRail.yamlPreview")}</CardTitle>
          <CardDescription>
            {t("contextRail.yamlPreviewDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="max-h-72 overflow-auto rounded-md border bg-muted/30 p-3 text-xs">
            {yamlPreview}
          </pre>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("contextRail.advancedAuthoring")}</CardTitle>
          <CardDescription>
            {t("contextRail.advancedAuthoringDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm">
          <div>
            <p className="font-medium">{t("contextRail.advancedSettings")}</p>
            {advancedSettings.length > 0 ? (
              <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                {advancedSettings.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            ) : (
              <p className="text-muted-foreground">{t("contextRail.advancedSettingsNone")}</p>
            )}
          </div>
          {inheritanceParent ? (
            <p className="text-muted-foreground">
              {t("contextRail.inheritsFrom", { name: inheritanceParent })}
            </p>
          ) : null}
          <div>
            <p className="font-medium">{t("contextRail.storedOnlyFields")}</p>
            <p className="text-muted-foreground">
              {t("contextRail.storedOnlyFieldsDesc")}
            </p>
            {storedOnlyFields.length > 0 ? (
              <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                {storedOnlyFields.map((field) => (
                  <li key={field}>{field}</li>
                ))}
              </ul>
            ) : (
              <p className="text-muted-foreground">{t("contextRail.storedOnlyNone")}</p>
            )}
          </div>
          <div>
            <p className="font-medium">{t("contextRail.runtimeProjection")}</p>
            <p className="text-muted-foreground">
              {t("contextRail.runtimeProjectionDesc")}
            </p>
            {runtimeExecutionProfile?.loaded_skills?.length ? (
              <div className="mt-2">
                <p className="font-medium text-foreground">Loaded skills</p>
                <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                  {runtimeExecutionProfile.loaded_skills.map((skill) => (
                    <li key={`loaded-${skill.path}`}>
                      {skill.label} ({skill.path})
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {runtimeExecutionProfile?.available_skills?.length ? (
              <div className="mt-2">
                <p className="font-medium text-foreground">On-demand skills</p>
                <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                  {runtimeExecutionProfile.available_skills.map((skill) => (
                    <li key={`available-${skill.path}`}>
                      {skill.label} ({skill.path})
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {runtimeExecutionProfile?.skill_diagnostics?.length ? (
              <div className="mt-2">
                <p className="font-medium text-foreground">Skill diagnostics</p>
                <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                  {runtimeExecutionProfile.skill_diagnostics.map((diagnostic, index) => (
                    <li key={`${diagnostic.code}:${diagnostic.path ?? "global"}:${index}`}>
                      {diagnostic.message}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("contextRail.previewAndSandbox")}</CardTitle>
          <CardDescription>
            {t("contextRail.previewAndSandboxDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={onPreview}
              disabled={previewLoading}
            >
              {previewLoading ? t("contextRail.previewing") : t("contextRail.previewDraft")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={onSandbox}
              disabled={sandboxLoading}
            >
              {sandboxLoading ? t("contextRail.running") : t("contextRail.runSandbox")}
            </Button>
          </div>
          <div className="grid gap-1.5">
            <label htmlFor="sandbox-input" className="text-sm font-medium">
              {t("contextRail.sandboxInput")}
            </label>
            <textarea
              id="sandbox-input"
              className="min-h-24 rounded-md border bg-background px-3 py-2 text-sm"
              rows={4}
              value={sandboxInput}
              onChange={(event) => onSandboxInputChange(event.target.value)}
            />
          </div>
          {previewResult?.executionProfile ? (
            <p className="text-sm text-muted-foreground">
              Effective role: {previewResult.executionProfile.name} (
              {previewResult.executionProfile.role_id})
            </p>
          ) : null}
          {sandboxResult?.selection ? (
            <p className="text-sm text-muted-foreground">{`${sandboxResult.selection.runtime} / ${sandboxResult.selection.provider} / ${sandboxResult.selection.model}`}</p>
          ) : null}
          {sandboxResult?.probe?.text ? (
            <p className="text-sm">{sandboxResult.probe.text}</p>
          ) : null}
          <div className="grid gap-2 border-t pt-3">
            <div>
              <p className="text-sm font-medium">{t("contextRail.readiness")}</p>
              {readinessDiagnostics.length > 0 ? (
                <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
                  {readinessDiagnostics.map((diagnostic) => (
                    <li key={`${diagnostic.code}:${diagnostic.message}`}>
                      {diagnostic.message}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-muted-foreground">
                  {t("contextRail.readinessNone")}
                </p>
              )}
            </div>
            <div>
              <p className="text-sm font-medium">{t("contextRail.validationIssues")}</p>
              {validationIssues.length > 0 ? (
                <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
                  {validationIssues.map((issue) => (
                    <li key={`${issue.field}:${issue.message}`}>
                      {issue.field}: {issue.message}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-muted-foreground">
                  {t("contextRail.validationIssuesNone")}
                </p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
