"use client";

import { Badge } from "@/components/ui/badge";
import { useTranslations } from "next-intl";
import { PluginDetailSection } from "@/components/plugins/plugin-detail-section";
import type { PluginRecord } from "@/lib/stores/plugin-store";

interface PluginKindDetailProps {
  plugin: PluginRecord;
}

function WorkflowDetail({ plugin }: { plugin: PluginRecord }) {
  const t = useTranslations("plugins");
  const workflow = plugin.spec.workflow;
  if (!workflow) return null;
  const roleDependencies = plugin.roleDependencies ?? [];

  return (
    <div className="flex flex-col gap-3">
      <PluginDetailSection title={t("kindDetail.processMode")}>
        {workflow.process}
      </PluginDetailSection>

      {workflow.roles && workflow.roles.length > 0 ? (
        <PluginDetailSection title={t("kindDetail.roles")}>
          <div className="flex flex-wrap gap-1.5">
            {workflow.roles.map((role) => (
              <Badge key={role.id} variant="secondary" className="text-xs">
                {role.id}
              </Badge>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}

      <PluginDetailSection title={t("kindDetail.steps")}>
        <div className="flex flex-col gap-2">
          {workflow.steps.map((step) => (
            <div
              key={step.id}
              className="rounded-md bg-muted/40 px-2 py-1.5 text-xs"
            >
              <p className="font-medium text-foreground">{step.id}</p>
              <p>
                {t("kindDetail.roleLabel", { role: step.role })} ·{" "}
                {t("kindDetail.actionLabel", { action: step.action })}
              </p>
              {step.next && step.next.length > 0 ? (
                <p>{t("kindDetail.nextLabel", { next: step.next.join(", ") })}</p>
              ) : null}
            </div>
          ))}
        </div>
      </PluginDetailSection>

      {workflow.triggers && workflow.triggers.length > 0 ? (
        <PluginDetailSection title={t("kindDetail.triggers")}>
          <div className="flex flex-col gap-1">
            {workflow.triggers.map((trigger, idx) => (
              <p key={idx}>
                {trigger.event ?? t("kindDetail.unknownTrigger")}
              </p>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}

      {workflow.limits?.maxRetries != null ? (
        <PluginDetailSection title={t("kindDetail.limits")}>
          {t("kindDetail.maxRetries", { count: workflow.limits.maxRetries })}
        </PluginDetailSection>
      ) : null}

      {roleDependencies.length > 0 ? (
        <PluginDetailSection title={t("kindDetail.roleDependencyHealth")}>
          <div className="flex flex-col gap-2 text-xs">
            {roleDependencies.map((dependency) => (
              <div
                key={`${dependency.roleId}:${dependency.status}`}
                className="rounded-md bg-muted/40 px-2 py-1.5"
              >
                <p className="font-medium text-foreground">
                  {dependency.roleName
                    ? `${dependency.roleName} (${dependency.roleId})`
                    : dependency.roleId}{" "}
                  · {dependency.status}
                </p>
                {dependency.references?.length ? (
                  <p>
                    {t("kindDetail.references", {
                      refs: dependency.references.join(", "),
                    })}
                  </p>
                ) : null}
                {dependency.message ? <p>{dependency.message}</p> : null}
              </div>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}
    </div>
  );
}

function ReviewDetail({ plugin }: { plugin: PluginRecord }) {
  const t = useTranslations("plugins");
  const review = plugin.spec.review;
  if (!review) return null;

  return (
    <div className="flex flex-col gap-3">
      {review.entrypoint ? (
        <PluginDetailSection title={t("kindDetail.entrypoint")}>
          {review.entrypoint}
        </PluginDetailSection>
      ) : null}

      <PluginDetailSection title={t("kindDetail.triggerEvents")}>
        <div className="flex flex-wrap gap-1.5">
          {review.triggers.events.map((evt) => (
            <Badge key={evt} variant="secondary" className="text-xs">
              {evt}
            </Badge>
          ))}
        </div>
      </PluginDetailSection>

      {review.triggers.filePatterns && review.triggers.filePatterns.length > 0 ? (
        <PluginDetailSection title={t("kindDetail.filePatterns")}>
          <div className="flex flex-wrap gap-1.5">
            {review.triggers.filePatterns.map((pat) => (
              <Badge key={pat} variant="outline" className="text-xs font-mono">
                {pat}
              </Badge>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}

      <PluginDetailSection title={t("kindDetail.outputFormat")}>
        {review.output.format}
      </PluginDetailSection>
    </div>
  );
}

function IntegrationDetail({ plugin }: { plugin: PluginRecord }) {
  const t = useTranslations("plugins");
  const capabilities = plugin.spec.capabilities;
  if (!capabilities || capabilities.length === 0) return null;

  return (
    <div className="flex flex-col gap-3">
      <PluginDetailSection title={t("kindDetail.capabilities")}>
        <div className="flex flex-wrap gap-1.5">
          {capabilities.map((cap) => (
            <Badge key={cap} variant="secondary" className="text-xs">
              {cap}
            </Badge>
          ))}
        </div>
      </PluginDetailSection>
    </div>
  );
}

function RoleDetail({ plugin }: { plugin: PluginRecord }) {
  const t = useTranslations("plugins");
  return (
    <div className="flex flex-col gap-3">
      {plugin.metadata.tags && plugin.metadata.tags.length > 0 ? (
        <PluginDetailSection title={t("kindDetail.tags")}>
          <div className="flex flex-wrap gap-1.5">
            {plugin.metadata.tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="text-xs">
                {tag}
              </Badge>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}

      <PluginDetailSection title={t("kindDetail.description")}>
        {plugin.metadata.description || t("kindDetail.noDescription")}
      </PluginDetailSection>
    </div>
  );
}

function ToolMCPDetail({ plugin }: { plugin: PluginRecord }) {
  const t = useTranslations("plugins");
  const mcp = plugin.runtime_metadata?.mcp;
  if (!mcp) return null;

  return (
    <div className="flex flex-col gap-3">
      <PluginDetailSection title={t("kindDetail.mcpSummary")}>
        <div className="grid gap-1">
          <p>{t("kindDetail.transport", { transport: mcp.transport })}</p>
          <p>{t("kindDetail.tools", { count: mcp.tool_count })}</p>
          <p>{t("kindDetail.resources", { count: mcp.resource_count })}</p>
          <p>{t("kindDetail.prompts", { count: mcp.prompt_count })}</p>
          {mcp.last_discovery_at ? (
            <p>{t("kindDetail.lastDiscovery", { date: mcp.last_discovery_at })}</p>
          ) : null}
        </div>
      </PluginDetailSection>

      {mcp.latest_interaction ? (
        <PluginDetailSection title={t("kindDetail.latestInteraction")}>
          <div className="grid gap-1">
            <p>
              {t("kindDetail.operation", {
                operation: mcp.latest_interaction.operation,
              })}
            </p>
            <p>
              {t("kindDetail.status", { status: mcp.latest_interaction.status })}
            </p>
            {mcp.latest_interaction.target ? (
              <p>
                {t("kindDetail.target", {
                  target: mcp.latest_interaction.target,
                })}
              </p>
            ) : null}
            {mcp.latest_interaction.summary ? (
              <p>
                {t("kindDetail.summary", {
                  summary: mcp.latest_interaction.summary,
                })}
              </p>
            ) : null}
            {mcp.latest_interaction.error_message ? (
              <p className="text-red-600 dark:text-red-400">
                {t("kindDetail.error", {
                  message: mcp.latest_interaction.error_message,
                })}
              </p>
            ) : null}
          </div>
        </PluginDetailSection>
      ) : null}
    </div>
  );
}

export function PluginKindDetail({ plugin }: PluginKindDetailProps) {
  const t = useTranslations("plugins");
  switch (plugin.kind) {
    case "WorkflowPlugin":
      return <WorkflowDetail plugin={plugin} />;
    case "ReviewPlugin":
      return <ReviewDetail plugin={plugin} />;
    case "IntegrationPlugin":
      return <IntegrationDetail plugin={plugin} />;
    case "RolePlugin":
      return <RoleDetail plugin={plugin} />;
    case "ToolPlugin":
      return <ToolMCPDetail plugin={plugin} />;
    default:
      return (
        <div className="rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
          {t("kindDetail.noKindDetails")}
        </div>
      );
  }
}
