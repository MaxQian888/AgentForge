"use client";

import { Badge } from "@/components/ui/badge";
import { PluginDetailSection } from "@/components/plugins/plugin-detail-sidebar";
import type { PluginRecord } from "@/lib/stores/plugin-store";

interface PluginKindDetailProps {
  plugin: PluginRecord;
}

function WorkflowDetail({ plugin }: { plugin: PluginRecord }) {
  const workflow = plugin.spec.workflow;
  if (!workflow) return null;

  return (
    <div className="flex flex-col gap-3">
      <PluginDetailSection title="Process mode">
        {workflow.process}
      </PluginDetailSection>

      {workflow.roles && workflow.roles.length > 0 ? (
        <PluginDetailSection title="Roles">
          <div className="flex flex-wrap gap-1.5">
            {workflow.roles.map((role) => (
              <Badge key={role.id} variant="secondary" className="text-xs">
                {role.id}
              </Badge>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}

      <PluginDetailSection title="Steps">
        <div className="flex flex-col gap-2">
          {workflow.steps.map((step) => (
            <div
              key={step.id}
              className="rounded-md bg-muted/40 px-2 py-1.5 text-xs"
            >
              <p className="font-medium text-foreground">{step.id}</p>
              <p>Role: {step.role} · Action: {step.action}</p>
              {step.next && step.next.length > 0 ? (
                <p>Next: {step.next.join(", ")}</p>
              ) : null}
            </div>
          ))}
        </div>
      </PluginDetailSection>

      {workflow.triggers && workflow.triggers.length > 0 ? (
        <PluginDetailSection title="Triggers">
          <div className="flex flex-col gap-1">
            {workflow.triggers.map((trigger, idx) => (
              <p key={idx}>{trigger.event ?? "Unknown trigger"}</p>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}

      {workflow.limits?.maxRetries != null ? (
        <PluginDetailSection title="Limits">
          Max retries: {workflow.limits.maxRetries}
        </PluginDetailSection>
      ) : null}
    </div>
  );
}

function ReviewDetail({ plugin }: { plugin: PluginRecord }) {
  const review = plugin.spec.review;
  if (!review) return null;

  return (
    <div className="flex flex-col gap-3">
      {review.entrypoint ? (
        <PluginDetailSection title="Entrypoint">
          {review.entrypoint}
        </PluginDetailSection>
      ) : null}

      <PluginDetailSection title="Trigger events">
        <div className="flex flex-wrap gap-1.5">
          {review.triggers.events.map((evt) => (
            <Badge key={evt} variant="secondary" className="text-xs">
              {evt}
            </Badge>
          ))}
        </div>
      </PluginDetailSection>

      {review.triggers.filePatterns && review.triggers.filePatterns.length > 0 ? (
        <PluginDetailSection title="File patterns">
          <div className="flex flex-wrap gap-1.5">
            {review.triggers.filePatterns.map((pat) => (
              <Badge key={pat} variant="outline" className="text-xs font-mono">
                {pat}
              </Badge>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}

      <PluginDetailSection title="Output format">
        {review.output.format}
      </PluginDetailSection>
    </div>
  );
}

function IntegrationDetail({ plugin }: { plugin: PluginRecord }) {
  const capabilities = plugin.spec.capabilities;
  if (!capabilities || capabilities.length === 0) return null;

  return (
    <div className="flex flex-col gap-3">
      <PluginDetailSection title="Capabilities">
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
  return (
    <div className="flex flex-col gap-3">
      {plugin.metadata.tags && plugin.metadata.tags.length > 0 ? (
        <PluginDetailSection title="Tags">
          <div className="flex flex-wrap gap-1.5">
            {plugin.metadata.tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="text-xs">
                {tag}
              </Badge>
            ))}
          </div>
        </PluginDetailSection>
      ) : null}

      <PluginDetailSection title="Description">
        {plugin.metadata.description || "No description provided."}
      </PluginDetailSection>
    </div>
  );
}

function ToolMCPDetail({ plugin }: { plugin: PluginRecord }) {
  const mcp = plugin.runtime_metadata?.mcp;
  if (!mcp) return null;

  return (
    <div className="flex flex-col gap-3">
      <PluginDetailSection title="MCP summary">
        <div className="grid gap-1">
          <p>Transport: {mcp.transport}</p>
          <p>Tools: {mcp.tool_count}</p>
          <p>Resources: {mcp.resource_count}</p>
          <p>Prompts: {mcp.prompt_count}</p>
          {mcp.last_discovery_at ? (
            <p>Last discovery: {mcp.last_discovery_at}</p>
          ) : null}
        </div>
      </PluginDetailSection>

      {mcp.latest_interaction ? (
        <PluginDetailSection title="Latest interaction">
          <div className="grid gap-1">
            <p>Operation: {mcp.latest_interaction.operation}</p>
            <p>Status: {mcp.latest_interaction.status}</p>
            {mcp.latest_interaction.target ? (
              <p>Target: {mcp.latest_interaction.target}</p>
            ) : null}
            {mcp.latest_interaction.summary ? (
              <p>Summary: {mcp.latest_interaction.summary}</p>
            ) : null}
            {mcp.latest_interaction.error_message ? (
              <p className="text-red-600 dark:text-red-400">
                Error: {mcp.latest_interaction.error_message}
              </p>
            ) : null}
          </div>
        </PluginDetailSection>
      ) : null}
    </div>
  );
}

export function PluginKindDetail({ plugin }: PluginKindDetailProps) {
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
          No kind-specific details available for this plugin type.
        </div>
      );
  }
}
