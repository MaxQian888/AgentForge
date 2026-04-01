"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type {
  RolePreviewResponse,
  RoleSandboxResponse,
} from "@/lib/stores/role-store";
import type {
  FieldProvenanceMap,
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
  provenanceMap?: FieldProvenanceMap;
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
  provenanceMap,
}: RoleWorkspaceContextRailProps) {
  const t = useTranslations("roles");
  const skillPartLabel = (part: string) => {
    switch (part) {
      case "agents":
        return t("workspace.skillPartAgents");
      case "references":
        return t("workspace.skillPartReferences");
      case "scripts":
        return t("workspace.skillPartScripts");
      case "assets":
        return t("workspace.skillPartAssets");
      default:
        return part;
    }
  };
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
  const pluginDependencies = effectiveManifest?.pluginDependencies ?? [];
  const pluginConsumers = effectiveManifest?.pluginConsumers ?? [];
  const readinessDiagnostics =
    sandboxResult?.readinessDiagnostics ?? previewResult?.readinessDiagnostics ?? [];
  const validationIssues =
    sandboxResult?.validationIssues && sandboxResult.validationIssues.length > 0
      ? sandboxResult.validationIssues
      : (previewResult?.validationIssues ?? []);

  return (
    <div className="flex flex-col">
      {/* Authoring Guide — sticky header */}
      <div className="sticky top-0 z-10 border-b bg-background px-4 py-3">
        <p className="text-xs font-semibold">{t("contextRail.authoringGuide")}</p>
        <p className="text-xs text-muted-foreground">
          {t("contextRail.guidanceFor", { section: activeSectionLabel.toLowerCase() })}
        </p>
      </div>
      <div className="border-b px-4 py-3 text-xs text-muted-foreground">
        <p className="mb-1 font-medium text-foreground">{guidance.title}</p>
        <p className="mb-2">{guidance.summary}</p>
        <ul className="list-disc space-y-0.5 pl-4">
          {guidance.bullets.map((bullet) => (
            <li key={bullet}>{bullet}</li>
          ))}
        </ul>
      </div>

      {/* Preview & Sandbox — promoted to top */}
      <div className="border-b px-4 py-3">
        <p className="mb-2 text-xs font-semibold">{t("contextRail.previewAndSandbox")}</p>
        <div className="flex flex-wrap gap-1.5">
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={onPreview}
            disabled={previewLoading}
          >
            {previewLoading ? t("contextRail.previewing") : t("contextRail.previewDraft")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={onSandbox}
            disabled={sandboxLoading}
          >
            {sandboxLoading ? t("contextRail.running") : t("contextRail.runSandbox")}
          </Button>
        </div>
        <div className="mt-2 grid gap-1">
          <label htmlFor="sandbox-input" className="text-xs font-medium">
            {t("contextRail.sandboxInput")}
          </label>
          <Textarea
            id="sandbox-input"
            className="min-h-16 rounded-md border bg-background px-2.5 py-1.5 text-xs"
            rows={3}
            value={sandboxInput}
            onChange={(event) => onSandboxInputChange(event.target.value)}
          />
        </div>
        {previewResult?.executionProfile ? (
          <p className="mt-2 text-xs text-muted-foreground">
            Effective role: {previewResult.executionProfile.name} ({previewResult.executionProfile.role_id})
          </p>
        ) : null}
        {sandboxResult?.selection ? (
          <p className="mt-1 text-xs text-muted-foreground">{`${sandboxResult.selection.runtime} / ${sandboxResult.selection.provider} / ${sandboxResult.selection.model}`}</p>
        ) : null}
        {sandboxResult?.probe?.text ? (
          <p className="mt-1 text-xs">{sandboxResult.probe.text}</p>
        ) : null}
        <div className="mt-3 grid gap-2 border-t pt-2.5">
          <div>
            <p className="text-xs font-medium">{t("contextRail.readiness")}</p>
            {readinessDiagnostics.length > 0 ? (
              <ul className="mt-0.5 list-disc space-y-0.5 pl-4 text-xs text-muted-foreground">
                {readinessDiagnostics.map((diagnostic) => (
                  <li key={`${diagnostic.code}:${diagnostic.message}`}>
                    {diagnostic.message}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-xs text-muted-foreground">{t("contextRail.readinessNone")}</p>
            )}
          </div>
          <div>
            <p className="text-xs font-medium">{t("contextRail.validationIssues")}</p>
            {validationIssues.length > 0 ? (
              <ul className="mt-0.5 list-disc space-y-0.5 pl-4 text-xs text-muted-foreground">
                {validationIssues.map((issue) => (
                  <li key={`${issue.field}:${issue.message}`}>
                    {issue.field}: {issue.message}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-xs text-muted-foreground">{t("contextRail.validationIssuesNone")}</p>
            )}
          </div>
          {pluginDependencies.length > 0 ? (
            <div>
              <p className="text-xs font-medium">Plugin dependencies</p>
              <ul className="mt-0.5 list-disc space-y-0.5 pl-4 text-xs text-muted-foreground">
                {pluginDependencies.map((dependency) => (
                  <li key={`${dependency.pluginId}:${dependency.referenceType}`}>
                    {dependency.message ??
                      `${dependency.pluginId} (${dependency.referenceType}) — ${dependency.status}`}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          {pluginConsumers.length > 0 ? (
            <div>
              <p className="text-xs font-medium">Downstream plugin consumers</p>
              <ul className="mt-0.5 list-disc space-y-0.5 pl-4 text-xs text-muted-foreground">
                {pluginConsumers.map((consumer) => (
                  <li key={`${consumer.pluginId}:${consumer.status}`}>
                    {consumer.pluginName
                      ? `${consumer.pluginName} (${consumer.pluginId})`
                      : consumer.pluginId}
                    {` — ${consumer.status}`}
                    {consumer.references?.length ? ` · refs: ${consumer.references.join(", ")}` : ""}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </div>
      </div>

      {/* Execution Summary — collapsible, open by default */}
      <details open className="border-b">
        <summary className="flex cursor-pointer select-none items-center justify-between px-4 py-3 text-xs font-semibold hover:bg-muted/40">
          {t("contextRail.executionSummary")}
        </summary>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 px-4 pb-3 pt-1 text-xs">
          <dt className="text-muted-foreground">{t("contextRail.allowedTools")}</dt>
          <dd>{executionSummary.toolsLabel}</dd>
          <dt className="text-muted-foreground">{t("contextRail.skills")}</dt>
          <dd>{executionSummary.skillsLabel}</dd>
          <dt className="text-muted-foreground">{t("contextRail.budget")}</dt>
          <dd>{executionSummary.budgetLabel}</dd>
          <dt className="text-muted-foreground">{t("contextRail.turnLimit")}</dt>
          <dd>{executionSummary.turnsLabel}</dd>
          <dt className="text-muted-foreground">{t("contextRail.permissionMode")}</dt>
          <dd>{executionSummary.permissionMode}</dd>
        </dl>
        {executionSummary.promptIntent ? (
          <p className="border-t px-4 py-2 text-xs text-muted-foreground">
            <span className="font-medium text-foreground">{t("contextRail.promptIntent")}: </span>
            {executionSummary.promptIntent}
          </p>
        ) : null}
        {executionSummary.safetyCues.length > 0 ? (
          <div className="border-t px-4 py-2">
            <p className="mb-0.5 text-xs font-medium">{t("contextRail.safetyCues")}</p>
            <ul className="list-disc space-y-0.5 pl-4 text-xs text-muted-foreground">
              {executionSummary.safetyCues.map((cue) => <li key={cue}>{cue}</li>)}
            </ul>
          </div>
        ) : null}
        {effectiveSkillResolution.length > 0 ? (
          <div className="border-t px-4 py-2">
            <p className="mb-0.5 text-xs font-medium">{t("contextRail.skillResolution")}</p>
            <ul className="list-disc space-y-0.5 pl-4 text-xs text-muted-foreground">
              {effectiveSkillResolution.map((skill, index) => (
                <li key={`${skill.path}:${skill.provenance}:${index}`}>
                  {skill.label} ({skill.path}) — {skill.status} / {skill.provenance} / {skill.compatibilityStatus}
                  {skill.requires?.length ? ` · deps: ${skill.requires.join(", ")}` : ""}
                  {skill.tools?.length ? ` · tools: ${skill.tools.join(", ")}` : ""}
                  {skill.missingTools?.length ? ` · missing: ${skill.missingTools.join(", ")}` : ""}
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </details>

      {/* YAML Preview — collapsible, closed by default */}
      <details className="border-b">
        <summary className="flex cursor-pointer select-none items-center justify-between px-4 py-3 text-xs font-semibold hover:bg-muted/40">
          {t("contextRail.yamlPreview")}
        </summary>
        <div className="px-3 pb-3">
          <pre className="max-h-48 overflow-auto rounded-md border bg-muted/30 p-2.5 font-mono text-[11px] leading-relaxed">
            {yamlPreview}
          </pre>
        </div>
      </details>

      {/* Advanced Authoring — collapsible, closed by default */}
      <details className="border-b">
        <summary className="flex cursor-pointer select-none items-center justify-between px-4 py-3 text-xs font-semibold hover:bg-muted/40">
          {t("contextRail.advancedAuthoring")}
        </summary>
        <div className="grid gap-3 px-4 pb-3 pt-1 text-xs">
          <div>
            <p className="font-medium">{t("contextRail.advancedSettings")}</p>
            {advancedSettings.length > 0 ? (
              <ul className="mt-0.5 list-disc space-y-0.5 pl-4 text-muted-foreground">
                {advancedSettings.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            ) : (
              <p className="text-muted-foreground">{t("contextRail.advancedSettingsNone")}</p>
            )}
          </div>
          {provenanceMap ? (() => {
            const all = [
              ...provenanceMap.customSettings,
              ...provenanceMap.mcpServers,
              ...provenanceMap.sharedKnowledge,
              ...provenanceMap.privateKnowledge,
              ...provenanceMap.triggers,
              ...provenanceMap.collaboration,
            ];
            const inherited = all.filter((e) => e.provenance === "inherited").length;
            const template = all.filter((e) => e.provenance === "template").length;
            const explicit = all.filter((e) => e.provenance === "explicit").length;
            if (all.length === 0) return null;
            return (
              <div>
                <p className="font-medium">Field provenance</p>
                <p className="text-muted-foreground">
                  {t("workspace.provenanceSummary", { inherited: String(inherited), template: String(template), explicit: String(explicit) })}
                </p>
              </div>
            );
          })() : null}
          {inheritanceParent ? (
            <p className="text-muted-foreground">
              {t("contextRail.inheritsFrom", { name: inheritanceParent })}
            </p>
          ) : null}
          <div>
            <p className="font-medium">{t("contextRail.storedOnlyFields")}</p>
            <p className="text-muted-foreground">{t("contextRail.storedOnlyFieldsDesc")}</p>
            {storedOnlyFields.length > 0 ? (
              <ul className="mt-0.5 list-disc space-y-0.5 pl-4 text-muted-foreground">
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
            <p className="text-muted-foreground">{t("contextRail.runtimeProjectionDesc")}</p>
            {runtimeExecutionProfile?.loaded_skills?.length ? (
              <div className="mt-1.5">
                <p className="font-medium text-foreground">Loaded skills</p>
                <ul className="list-disc space-y-0.5 pl-4 text-muted-foreground">
                  {runtimeExecutionProfile.loaded_skills.map((skill) => (
                    <li key={`loaded-${skill.path}`}>
                      {skill.label} ({skill.path})
                      {skill.origin ? ` · origin: ${skill.origin}` : ""}
                      {skill.requires?.length ? ` · deps: ${skill.requires.join(", ")}` : ""}
                      {skill.tools?.length ? ` · tools: ${skill.tools.join(", ")}` : ""}
                      {skill.available_parts?.length
                        ? ` · ${t("workspace.skillPartsLabel")}: ${skill.available_parts
                            .map((part) => skillPartLabel(part))
                            .join(", ")}`
                        : ""}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {runtimeExecutionProfile?.available_skills?.length ? (
              <div className="mt-1.5">
                <p className="font-medium text-foreground">On-demand skills</p>
                <ul className="list-disc space-y-0.5 pl-4 text-muted-foreground">
                  {runtimeExecutionProfile.available_skills.map((skill) => (
                    <li key={`available-${skill.path}`}>
                      {skill.label} ({skill.path})
                      {skill.requires?.length ? ` · deps: ${skill.requires.join(", ")}` : ""}
                      {skill.tools?.length ? ` · tools: ${skill.tools.join(", ")}` : ""}
                      {skill.available_parts?.length
                        ? ` · ${t("workspace.skillPartsLabel")}: ${skill.available_parts
                            .map((part) => skillPartLabel(part))
                            .join(", ")}`
                        : ""}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            {runtimeExecutionProfile?.skill_diagnostics?.length ? (
              <div className="mt-1.5">
                <p className="font-medium text-foreground">Skill diagnostics</p>
                <ul className="list-disc space-y-0.5 pl-4 text-muted-foreground">
                  {runtimeExecutionProfile.skill_diagnostics.map((diagnostic, index) => (
                    <li key={`${diagnostic.code}:${diagnostic.path ?? "global"}:${index}`}>
                      {diagnostic.message}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        </div>
      </details>
    </div>
  );
}
