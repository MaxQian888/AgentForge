"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type {
  RolePreviewResponse,
  RoleSandboxResponse,
} from "@/lib/stores/role-store";
import type { RoleExecutionSummary } from "@/lib/roles/role-management";
import {
  ROLE_WORKSPACE_GUIDANCE,
  ROLE_WORKSPACE_SECTIONS,
  type RoleWorkspaceSectionId,
} from "./role-workspace-sections";

interface RoleWorkspaceContextRailProps {
  activeSection: RoleWorkspaceSectionId;
  executionSummary: RoleExecutionSummary;
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
  const activeSectionLabel =
    ROLE_WORKSPACE_SECTIONS.find((section) => section.id === activeSection)?.label ??
    "Review";
  const guidance = ROLE_WORKSPACE_GUIDANCE[activeSection];

  return (
    <section className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Authoring Guide</CardTitle>
          <CardDescription>
            Focused guidance for the current {activeSectionLabel.toLowerCase()} step.
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
          <CardTitle>Execution Summary</CardTitle>
          <CardDescription>
            Review the draft execution intent and governance settings before saving.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm">
          <div>
            <p className="font-medium">Prompt intent</p>
            <p className="text-muted-foreground">
              {executionSummary.promptIntent || "No prompt intent yet"}
            </p>
          </div>
          <div>
            <p className="font-medium">Allowed tools</p>
            <p className="text-muted-foreground">{executionSummary.toolsLabel}</p>
          </div>
          <div>
            <p className="font-medium">Skills</p>
            <p className="text-muted-foreground">{executionSummary.skillsLabel}</p>
            <p className="text-muted-foreground">
              {executionSummary.keySkillPaths.length > 0
                ? executionSummary.keySkillPaths.join(", ")
                : "No key skills selected"}
            </p>
          </div>
          <div>
            <p className="font-medium">Budget</p>
            <p className="text-muted-foreground">{executionSummary.budgetLabel}</p>
          </div>
          <div>
            <p className="font-medium">Turn limit</p>
            <p className="text-muted-foreground">{executionSummary.turnsLabel}</p>
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

      <Card>
        <CardHeader>
          <CardTitle>YAML Preview</CardTitle>
          <CardDescription>
            Inspect the current draft as a YAML-oriented manifest before saving.
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
          <CardTitle>Preview And Sandbox</CardTitle>
          <CardDescription>
            Resolve effective values and optionally run a bounded prompt probe.
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
              {previewLoading ? "Previewing..." : "Preview Role Draft"}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={onSandbox}
              disabled={sandboxLoading}
            >
              {sandboxLoading ? "Running..." : "Run Sandbox Probe"}
            </Button>
          </div>
          <div className="grid gap-1.5">
            <label htmlFor="sandbox-input" className="text-sm font-medium">
              Sandbox Input
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
        </CardContent>
      </Card>
    </section>
  );
}
