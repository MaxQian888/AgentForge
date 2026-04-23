"use client";

import React from "react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { useTranslations } from "next-intl";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useWorkflowStore, type WorkflowDefinition } from "@/lib/stores/workflow-store";
import { usePluginStore, type PluginRecord } from "@/lib/stores/plugin-store";

// ── Types ─────────────────────────────────────────────────────────────────────

type TargetKind = "dag" | "plugin";

interface SubWorkflowConfigProps {
  config: Record<string, unknown>;
  onChange: (config: Record<string, unknown>) => void;
  /**
   * Optional inputs that let hosts (including tests) supply the selectable
   * target lists without pulling the whole store. When omitted, the component
   * consults the workflow + plugin stores directly.
   */
  dagWorkflows?: WorkflowDefinition[];
  plugins?: PluginRecord[];
  /**
   * Parent workflow id, used to filter DAG candidates so a workflow cannot
   * trivially self-reference. Optional because the Create flow has no id yet.
   */
  parentWorkflowId?: string;
}

// ── Component ─────────────────────────────────────────────────────────────────

export function SubWorkflowConfig({
  config,
  onChange,
  dagWorkflows,
  plugins,
  parentWorkflowId,
}: SubWorkflowConfigProps) {
  const t = useTranslations("workflow");
  // Pull from stores only when callers did not override. Selector subscriptions
  // are deliberately narrow so this component does not re-render on unrelated
  // store mutations.
  const storeDefinitions = useWorkflowStore((s) => s.definitions);
  const storePlugins = usePluginStore((s) => s.plugins);
  const resolvedDagWorkflows = dagWorkflows ?? storeDefinitions;
  const resolvedPlugins = plugins ?? storePlugins;

  const rawKind = (config.targetKind as string | undefined) ?? "dag";
  const targetKind: TargetKind = rawKind === "plugin" ? "plugin" : "dag";
  const targetWorkflowId =
    (config.targetWorkflowId as string | undefined) ??
    (config.workflowId as string | undefined) ??
    "";
  const inputMapping = (config.inputMapping as string | undefined) ?? "";

  function update(partial: Record<string, unknown>) {
    onChange({ ...config, ...partial });
  }

  // Filter DAG candidates: same-project scoping is handled at save-time by the
  // backend; here we filter out the parent workflow itself so the editor UX
  // surfaces the trivial-self-loop case *before* a save round-trip.
  const dagCandidates = resolvedDagWorkflows.filter(
    (w) => w.id !== parentWorkflowId,
  );
  const pluginCandidates = resolvedPlugins.filter(
    (p) => p.kind === "WorkflowPlugin",
  );

  function handleKindChange(nextKind: TargetKind) {
    // Clearing targetWorkflowId when the kind flips avoids a stale id that
    // points to a different-engine target after a user toggles DAG↔Plugin.
    update({ targetKind: nextKind, targetWorkflowId: "" });
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Target Kind */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("nodeConfig.subWorkflow.targetKind")}</Label>
        <Select
          value={targetKind}
          onValueChange={(v) => handleKindChange(v as TargetKind)}
        >
          <SelectTrigger className="text-xs">
            <SelectValue placeholder={t("nodeConfig.subWorkflow.targetKindPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="dag">{t("nodeConfig.subWorkflow.dagWorkflow")}</SelectItem>
            <SelectItem value="plugin">{t("nodeConfig.subWorkflow.workflowPlugin")}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Target Workflow / Plugin Selector */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">
          {targetKind === "dag"
            ? t("nodeConfig.subWorkflow.targetWorkflow")
            : t("nodeConfig.subWorkflow.targetPlugin")}
        </Label>
        {targetKind === "dag" && dagCandidates.length > 0 ? (
          <Select
            value={targetWorkflowId}
            onValueChange={(v) => update({ targetWorkflowId: v })}
          >
            <SelectTrigger className="text-xs">
              <SelectValue placeholder={t("nodeConfig.subWorkflow.pickDagWorkflow")} />
            </SelectTrigger>
            <SelectContent>
              {dagCandidates.map((wf) => (
                <SelectItem key={wf.id} value={wf.id}>
                  {wf.name || wf.id}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : targetKind === "plugin" && pluginCandidates.length > 0 ? (
          <Select
            value={targetWorkflowId}
            onValueChange={(v) => update({ targetWorkflowId: v })}
          >
            <SelectTrigger className="text-xs">
              <SelectValue placeholder={t("nodeConfig.subWorkflow.pickWorkflowPlugin")} />
            </SelectTrigger>
            <SelectContent>
              {pluginCandidates.map((p) => (
                <SelectItem key={p.metadata.id} value={p.metadata.id}>
                  {p.metadata.name || p.metadata.id}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <Input
            placeholder={
              targetKind === "dag"
                ? t("nodeConfig.subWorkflow.enterDagUuid")
                : t("nodeConfig.subWorkflow.enterPluginId")
            }
            value={targetWorkflowId}
            onChange={(e) => update({ targetWorkflowId: e.target.value })}
          />
        )}
      </div>

      {/* Input Mapping */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("nodeConfig.subWorkflow.inputMapping")}</Label>
        <Textarea
          rows={6}
          placeholder={t("nodeConfig.subWorkflow.inputMappingPlaceholder")}
          value={inputMapping}
          onChange={(e) => update({ inputMapping: e.target.value })}
          className="font-mono text-xs"
        />
      </div>
    </div>
  );
}
