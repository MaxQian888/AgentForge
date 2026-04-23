"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { useTranslations } from "next-intl";
import {
  usePluginStore,
  type MCPToolCallResult,
  type MCPResourceReadResult,
  type MCPPromptGetResult,
  type PluginRecord,
} from "@/lib/stores/plugin-store";
import { PluginDetailSection } from "./plugin-detail-section";
import {
  ChevronDown,
  ChevronRight,
  RefreshCw,
  Terminal,
  FileText,
  MessageSquare,
} from "lucide-react";

type DiagResult =
  | { kind: "tool"; data: MCPToolCallResult }
  | { kind: "resource"; data: MCPResourceReadResult }
  | { kind: "prompt"; data: MCPPromptGetResult }
  | null;

export function PluginMCPPanel({ plugin }: { plugin: PluginRecord }) {
  const t = useTranslations("plugins");
  const mcp = plugin.runtime_metadata?.mcp;
  const snapshot = usePluginStore((s) => s.mcpSnapshots[plugin.metadata.id]);
  const refreshMCP = usePluginStore((s) => s.refreshMCP);
  const callMCPTool = usePluginStore((s) => s.callMCPTool);
  const readMCPResource = usePluginStore((s) => s.readMCPResource);
  const getMCPPrompt = usePluginStore((s) => s.getMCPPrompt);

  const [refreshing, setRefreshing] = useState(false);
  const [diagOpen, setDiagOpen] = useState(false);
  const [selectedTool, setSelectedTool] = useState("");
  const [toolArgs, setToolArgs] = useState("{}");
  const [resourceUri, setResourceUri] = useState("");
  const [selectedPrompt, setSelectedPrompt] = useState("");
  const [promptArgs, setPromptArgs] = useState("{}");
  const [diagResult, setDiagResult] = useState<DiagResult>(null);
  const [diagError, setDiagError] = useState<string | null>(null);

  useEffect(() => {
    if (plugin.spec.runtime === "mcp" && !snapshot) {
      void refreshMCP(plugin.metadata.id);
    }
  }, [plugin.metadata.id, plugin.spec.runtime, snapshot, refreshMCP]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    await refreshMCP(plugin.metadata.id);
    setRefreshing(false);
  }, [plugin.metadata.id, refreshMCP]);

  const handleCallTool = useCallback(async () => {
    setDiagError(null);
    setDiagResult(null);
    try {
      const args = JSON.parse(toolArgs) as Record<string, unknown>;
      const result = await callMCPTool(plugin.metadata.id, selectedTool, args);
      if (result) setDiagResult({ kind: "tool", data: result });
    } catch {
      setDiagError(t("mcpPanel.invalidJson") ?? "Invalid JSON arguments or call failed");
    }
  }, [plugin.metadata.id, selectedTool, toolArgs, callMCPTool, t]);

  const handleReadResource = useCallback(async () => {
    setDiagError(null);
    setDiagResult(null);
    const result = await readMCPResource(plugin.metadata.id, resourceUri);
    if (result) setDiagResult({ kind: "resource", data: result });
  }, [plugin.metadata.id, resourceUri, readMCPResource]);

  const handleGetPrompt = useCallback(async () => {
    setDiagError(null);
    setDiagResult(null);
    try {
      const args = JSON.parse(promptArgs) as Record<string, string>;
      const result = await getMCPPrompt(
        plugin.metadata.id,
        selectedPrompt,
        args,
      );
      if (result) setDiagResult({ kind: "prompt", data: result });
    } catch {
      setDiagError(t("mcpPanel.invalidJson") ?? "Invalid JSON arguments or call failed");
    }
  }, [plugin.metadata.id, selectedPrompt, promptArgs, getMCPPrompt, t]);

  if (plugin.spec.runtime !== "mcp") {
    return (
      <p className="text-sm text-muted-foreground">
        {t("mcpPanel.onlyMcp")}
      </p>
    );
  }

  const interaction = mcp?.latest_interaction;
  const tools = snapshot?.tools ?? [];
  const resources = snapshot?.resources ?? [];
  const prompts = snapshot?.prompts ?? [];

  return (
    <div className="flex flex-col gap-3">
      {/* Capability summary */}
      <PluginDetailSection
        title={t("mcpPanel.capabilitiesTitle")}
        action={
          <Button
            variant="outline"
            size="sm"
            onClick={() => void handleRefresh()}
            disabled={refreshing}
          >
            <RefreshCw
              className={cn("mr-1 size-3.5", refreshing && "animate-spin")}
            />
            {t("mcpPanel.refresh")}
          </Button>
        }
      >
        <div className="grid grid-cols-3 gap-2">
          <div>
            <p className="font-medium text-foreground">
              {mcp?.tool_count ?? 0}
            </p>
            <p>{t("mcpPanel.toolsLabel")}</p>
          </div>
          <div>
            <p className="font-medium text-foreground">
              {mcp?.resource_count ?? 0}
            </p>
            <p>{t("mcpPanel.resourcesLabel")}</p>
          </div>
          <div>
            <p className="font-medium text-foreground">
              {mcp?.prompt_count ?? 0}
            </p>
            <p>{t("mcpPanel.promptsLabel")}</p>
          </div>
        </div>
        <p className="mt-2">
          {t("mcpPanel.transport", { transport: mcp?.transport ?? t("detailOverview.unknown") })}
        </p>
        {mcp?.last_discovery_at ? (
          <p>
            {t("mcpPanel.lastDiscovery", {
              date: new Date(mcp.last_discovery_at).toLocaleString(),
            })}
          </p>
        ) : null}
      </PluginDetailSection>

      {/* Latest interaction */}
      {interaction ? (
        <PluginDetailSection title={t("mcpPanel.latestInteraction")}>
          <div className="grid gap-1">
            <p>
              {t("mcpPanel.operation", { operation: interaction.operation })}
            </p>
            <p>
              {t("mcpPanel.status")}{" "}
              <Badge
                variant="secondary"
                className={cn(
                  "text-xs",
                  interaction.status === "succeeded"
                    ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400"
                    : "bg-red-500/15 text-red-700 dark:text-red-400",
                )}
              >
                {interaction.status}
              </Badge>
            </p>
            {interaction.target ? (
              <p>{t("mcpPanel.target", { target: interaction.target })}</p>
            ) : null}
            {interaction.summary ? <p>{interaction.summary}</p> : null}
            {interaction.at ? (
              <p>
                {t("mcpPanel.at", { date: new Date(interaction.at).toLocaleString() })}
              </p>
            ) : null}
            {interaction.error_message ? (
              <p className="text-destructive">
                {t("mcpPanel.error", {
                  message: interaction.error_code
                    ? `[${interaction.error_code}] ${interaction.error_message}`
                    : interaction.error_message,
                })}
              </p>
            ) : null}
          </div>
        </PluginDetailSection>
      ) : null}

      {/* Discovered tools */}
      {tools.length > 0 ? (
        <PluginDetailSection
          title={t("mcpPanel.toolsTitle", { count: tools.length })}
        >
          <ul className="grid gap-1">
            {tools.map((tItem) => (
              <li key={tItem.name} className="flex items-start gap-2">
                <Terminal className="mt-0.5 size-3.5 shrink-0" />
                <div>
                  <span className="font-medium text-foreground">{tItem.name}</span>
                  {tItem.description ? (
                    <span className="ml-1">{tItem.description}</span>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        </PluginDetailSection>
      ) : null}

      {/* Discovered resources */}
      {resources.length > 0 ? (
        <PluginDetailSection
          title={t("mcpPanel.resourcesTitle", { count: resources.length })}
        >
          <ul className="grid gap-1">
            {resources.map((r) => (
              <li key={r.uri} className="flex items-start gap-2">
                <FileText className="mt-0.5 size-3.5 shrink-0" />
                <div>
                  <span className="font-medium text-foreground">
                    {r.name ?? r.uri}
                  </span>
                  {r.name ? (
                    <span className="ml-1 text-xs">{r.uri}</span>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        </PluginDetailSection>
      ) : null}

      {/* Discovered prompts */}
      {prompts.length > 0 ? (
        <PluginDetailSection
          title={t("mcpPanel.promptsTitle", { count: prompts.length })}
        >
          <ul className="grid gap-1">
            {prompts.map((p) => (
              <li key={p.name} className="flex items-start gap-2">
                <MessageSquare className="mt-0.5 size-3.5 shrink-0" />
                <div>
                  <span className="font-medium text-foreground">{p.name}</span>
                  {p.description ? (
                    <span className="ml-1">{p.description}</span>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        </PluginDetailSection>
      ) : null}

      {/* Diagnostic actions */}
      <div className="rounded-lg border border-border/60 p-3">
        <button
          type="button"
          className="flex w-full items-center gap-1 text-sm font-medium"
          onClick={() => setDiagOpen(!diagOpen)}
        >
          {diagOpen ? (
            <ChevronDown className="size-4" />
          ) : (
            <ChevronRight className="size-4" />
          )}
          {t("mcpPanel.diagnosticActions")}
        </button>

        {diagOpen ? (
          <div className="mt-3 flex flex-col gap-4">
            {/* Tool call */}
            <div className="grid gap-2">
              <Label className="text-xs font-medium">{t("mcpPanel.callTool")}</Label>
              <select
                className="h-8 rounded-md border bg-background px-2 text-xs"
                value={selectedTool}
                onChange={(e) => setSelectedTool(e.target.value)}
              >
                <option value="">{t("mcpPanel.selectTool")}</option>
                {tools.map((tItem) => (
                  <option key={tItem.name} value={tItem.name}>
                    {tItem.name}
                  </option>
                ))}
              </select>
              <Textarea
                className="min-h-[60px] w-full rounded-md border bg-background px-2 py-1 font-mono text-xs"
                placeholder={t("mcpPanel.toolArgsPlaceholder")}
                value={toolArgs}
                onChange={(e) => setToolArgs(e.target.value)}
              />
              <Button
                variant="outline"
                size="sm"
                disabled={!selectedTool}
                onClick={() => void handleCallTool()}
              >
                {t("mcpPanel.call")}
              </Button>
            </div>

            {/* Resource read */}
            <div className="grid gap-2">
              <Label className="text-xs font-medium">{t("mcpPanel.readResource")}</Label>
              <Input
                className="h-8 text-xs"
                placeholder={t("mcpPanel.resourcePlaceholder")}
                value={resourceUri}
                onChange={(e) => setResourceUri(e.target.value)}
              />
              <Button
                variant="outline"
                size="sm"
                disabled={!resourceUri}
                onClick={() => void handleReadResource()}
              >
                {t("mcpPanel.read")}
              </Button>
            </div>

            {/* Prompt get */}
            <div className="grid gap-2">
              <Label className="text-xs font-medium">{t("mcpPanel.getPrompt")}</Label>
              <select
                className="h-8 rounded-md border bg-background px-2 text-xs"
                value={selectedPrompt}
                onChange={(e) => setSelectedPrompt(e.target.value)}
              >
                <option value="">{t("mcpPanel.selectPrompt")}</option>
                {prompts.map((p) => (
                  <option key={p.name} value={p.name}>
                    {p.name}
                  </option>
                ))}
              </select>
              <Textarea
                className="min-h-[60px] w-full rounded-md border bg-background px-2 py-1 font-mono text-xs"
                placeholder={t("mcpPanel.promptArgsPlaceholder")}
                value={promptArgs}
                onChange={(e) => setPromptArgs(e.target.value)}
              />
              <Button
                variant="outline"
                size="sm"
                disabled={!selectedPrompt}
                onClick={() => void handleGetPrompt()}
              >
                {t("mcpPanel.get")}
              </Button>
            </div>

            {/* Result display */}
            {diagError ? (
              <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                {diagError}
              </div>
            ) : null}

            {diagResult ? (
              <div className="rounded-md border border-border/60 bg-muted/30 p-3">
                <p className="mb-1 text-xs font-medium">
                  {t("mcpPanel.result", { kind: diagResult.kind })}
                </p>
                <pre className="max-h-[200px] overflow-auto text-xs">
                  {JSON.stringify(diagResult.data, null, 2)}
                </pre>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}
