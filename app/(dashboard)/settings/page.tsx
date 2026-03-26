"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useProjectStore } from "@/lib/stores/project-store";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

export default function SettingsPage() {
  const { selectedProjectId } = useDashboardStore();
  const { projects, fetchProjects, updateProject } = useProjectStore();

  const project = projects.find((p) => p.id === selectedProjectId);

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("");
  const [runtime, setRuntime] = useState("");
  const [provider, setProvider] = useState("");
  const [model, setModel] = useState("");
  const [maxTaskBudget, setMaxTaskBudget] = useState(0);
  const [maxDailySpend, setMaxDailySpend] = useState(0);
  const [alertThreshold, setAlertThreshold] = useState(80);
  const [autoStopOnExceed, setAutoStopOnExceed] = useState(false);
  const [autoTriggerOnPR, setAutoTriggerOnPR] = useState(false);
  const [requiredLayers, setRequiredLayers] = useState("1");
  const [minRiskLevelForBlock, setMinRiskLevelForBlock] = useState("critical");
  const [requireManualApproval, setRequireManualApproval] = useState(false);
  const [webhookUrl, setWebhookUrl] = useState("");
  const [webhookSecret, setWebhookSecret] = useState("");
  const [webhookEvents, setWebhookEvents] = useState<string[]>([]);
  const [webhookActive, setWebhookActive] = useState(false);
  const [saved, setSaved] = useState(false);
  const savedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    void fetchProjects();
  }, [fetchProjects]);

  useEffect(() => {
    if (project) {
      setName(project.name);
      setDescription(project.description ?? "");
      setRepoUrl(project.repoUrl ?? "");
      setDefaultBranch(project.defaultBranch ?? "main");
      setRuntime(
        project.settings?.codingAgent.runtime ||
          project.codingAgentCatalog?.defaultSelection.runtime ||
          ""
      );
      setProvider(
        project.settings?.codingAgent.provider ||
          project.codingAgentCatalog?.defaultSelection.provider ||
          ""
      );
      setModel(
        project.settings?.codingAgent.model ||
          project.codingAgentCatalog?.defaultSelection.model ||
          ""
      );
      const bg = project.settings?.budgetGovernance;
      if (bg) {
        setMaxTaskBudget(bg.maxTaskBudgetUsd);
        setMaxDailySpend(bg.maxDailySpendUsd);
        setAlertThreshold(bg.alertThresholdPercent);
        setAutoStopOnExceed(bg.autoStopOnExceed);
      }
      const rp = project.settings?.reviewPolicy;
      if (rp) {
        setAutoTriggerOnPR(rp.autoTriggerOnPR);
        setRequiredLayers(String(rp.requiredLayers));
        setMinRiskLevelForBlock(rp.minRiskLevelForBlock);
        setRequireManualApproval(rp.requireManualApproval);
      }
      const wh = project.settings?.webhook;
      if (wh) {
        setWebhookUrl(wh.url);
        setWebhookSecret(wh.secret);
        setWebhookEvents(wh.events);
        setWebhookActive(wh.active);
      }
    }
  }, [project]);

  useEffect(() => {
    return () => {
      if (savedTimeoutRef.current) {
        clearTimeout(savedTimeoutRef.current);
      }
    };
  }, []);

  const runtimeOptions = project?.codingAgentCatalog?.runtimes ?? [];
  const selectedRuntime =
    runtimeOptions.find((option) => option.runtime === runtime) ?? runtimeOptions[0];
  const compatibleProviders = selectedRuntime?.compatibleProviders ?? [];
  const selectedDiagnostics = selectedRuntime?.diagnostics ?? [];

  const handleRuntimeChange = (nextRuntime: string) => {
    setRuntime(nextRuntime);
    const nextOption = runtimeOptions.find((option) => option.runtime === nextRuntime);
    if (!nextOption) {
      return;
    }
    setProvider(nextOption.defaultProvider);
    setModel(nextOption.defaultModel);
  };

  const handleProviderChange = (nextProvider: string) => {
    setProvider(nextProvider);
  };

  const handleSave = async () => {
    if (!project) return;
    await updateProject(project.id, {
      name,
      description,
      repoUrl,
      defaultBranch,
      settings: {
        codingAgent: { runtime, provider, model },
        budgetGovernance: {
          maxTaskBudgetUsd: maxTaskBudget,
          maxDailySpendUsd: maxDailySpend,
          alertThresholdPercent: alertThreshold,
          autoStopOnExceed,
        },
        reviewPolicy: {
          autoTriggerOnPR,
          requiredLayers: Number(requiredLayers),
          minRiskLevelForBlock,
          requireManualApproval,
          enabledPluginDimensions: [],
        },
        webhook: {
          url: webhookUrl,
          secret: webhookSecret,
          events: webhookEvents,
          active: webhookActive,
        },
      },
    });
    setSaved(true);
    if (savedTimeoutRef.current) {
      clearTimeout(savedTimeoutRef.current);
    }
    savedTimeoutRef.current = setTimeout(() => {
      setSaved(false);
      savedTimeoutRef.current = null;
    }, 2000);
  };

  if (!selectedProjectId) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-sm text-muted-foreground">
          Select a project from the Dashboard to configure settings.
        </p>
      </div>
    );
  }

  if (!project) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-sm text-muted-foreground">Loading project...</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">Project Settings</h1>

      <Card>
        <CardHeader>
          <CardTitle>General</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2">
            <Label>Project Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Description</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Repository</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2">
            <Label>Repository URL</Label>
            <Input
              value={repoUrl}
              placeholder="https://github.com/org/repo"
              onChange={(e) => setRepoUrl(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Default Branch</Label>
            <Input
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Coding Agent Defaults</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="flex flex-col gap-2">
              <Label>Runtime</Label>
              <Select value={runtime} onValueChange={handleRuntimeChange}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {runtimeOptions.map((option) => (
                    <SelectItem key={option.runtime} value={option.runtime}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Provider</Label>
              <Select value={provider} onValueChange={handleProviderChange}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {compatibleProviders.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Model</Label>
              <Input value={model} onChange={(e) => setModel(e.target.value)} />
            </div>
          </div>

          {selectedDiagnostics.length > 0 && (
            <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
              {selectedDiagnostics.map((diagnostic) => (
                <p key={`${diagnostic.code}-${diagnostic.message}`}>
                  {diagnostic.message}
                </p>
              ))}
            </div>
          )}

          <div className="grid gap-3 md:grid-cols-2">
            {runtimeOptions.map((option) => (
              <div
                key={option.runtime}
                className="rounded-md border p-4 text-sm"
              >
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="font-medium">{option.label}</p>
                    <p className="text-muted-foreground">
                      {option.defaultProvider} / {option.defaultModel}
                    </p>
                  </div>
                  <Badge variant={option.available ? "default" : "secondary"}>
                    {option.available ? "Ready" : "Unavailable"}
                  </Badge>
                </div>
                {option.diagnostics.length > 0 && (
                  <div className="mt-3 space-y-1 text-xs text-muted-foreground">
                    {option.diagnostics.map((diagnostic) => (
                      <p key={`${option.runtime}-${diagnostic.code}`}>
                        {diagnostic.message}
                      </p>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Budget &amp; Alert Governance</CardTitle>
          <CardDescription>
            Configure spending limits and alert thresholds for agent runs.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>Max Task Budget (USD)</Label>
              <Input
                type="number"
                min={0}
                step={0.01}
                value={maxTaskBudget}
                onChange={(e) => setMaxTaskBudget(Number(e.target.value))}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Max Daily Spend (USD)</Label>
              <Input
                type="number"
                min={0}
                step={0.01}
                value={maxDailySpend}
                onChange={(e) => setMaxDailySpend(Number(e.target.value))}
              />
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>Alert Threshold (%)</Label>
              <Input
                type="number"
                min={0}
                max={100}
                value={alertThreshold}
                onChange={(e) => setAlertThreshold(Number(e.target.value))}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Auto-stop on Exceed</Label>
              <Select
                value={autoStopOnExceed ? "yes" : "no"}
                onValueChange={(v) => setAutoStopOnExceed(v === "yes")}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">Enabled</SelectItem>
                  <SelectItem value="no">Disabled</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Review Policy</CardTitle>
          <CardDescription>
            Control how code reviews are triggered and what approval gates apply.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>Auto-trigger on PR</Label>
              <Select
                value={autoTriggerOnPR ? "yes" : "no"}
                onValueChange={(v) => setAutoTriggerOnPR(v === "yes")}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">Enabled</SelectItem>
                  <SelectItem value="no">Disabled</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Required Review Layers</Label>
              <Select value={requiredLayers} onValueChange={setRequiredLayers}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">Quick (Layer 1)</SelectItem>
                  <SelectItem value="2">Deep (Layer 2)</SelectItem>
                  <SelectItem value="3">Human (Layer 3)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>Min Risk Level to Block Merge</Label>
              <Select
                value={minRiskLevelForBlock}
                onValueChange={setMinRiskLevelForBlock}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="critical">Critical</SelectItem>
                  <SelectItem value="high">High</SelectItem>
                  <SelectItem value="medium">Medium</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Require Manual Approval</Label>
              <Select
                value={requireManualApproval ? "yes" : "no"}
                onValueChange={(v) => setRequireManualApproval(v === "yes")}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">Required</SelectItem>
                  <SelectItem value="no">Not Required</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Webhook Configuration</CardTitle>
          <CardDescription>
            Configure webhook delivery for repository and review events.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>Webhook URL</Label>
              <Input
                value={webhookUrl}
                placeholder="https://example.com/webhook"
                onChange={(e) => setWebhookUrl(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Webhook Secret</Label>
              <Input
                type="password"
                value={webhookSecret}
                placeholder="Secret token"
                onChange={(e) => setWebhookSecret(e.target.value)}
              />
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>Events</Label>
              <div className="flex flex-wrap gap-2">
                {["push", "pr_opened", "pr_merged", "review_completed"].map(
                  (event) => (
                    <Button
                      key={event}
                      type="button"
                      size="sm"
                      variant={
                        webhookEvents.includes(event) ? "default" : "outline"
                      }
                      onClick={() =>
                        setWebhookEvents((prev) =>
                          prev.includes(event)
                            ? prev.filter((e) => e !== event)
                            : [...prev, event]
                        )
                      }
                    >
                      {event}
                    </Button>
                  )
                )}
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label>Active</Label>
              <Select
                value={webhookActive ? "yes" : "no"}
                onValueChange={(v) => setWebhookActive(v === "yes")}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">Active</SelectItem>
                  <SelectItem value="no">Inactive</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Operator Diagnostics</CardTitle>
          <CardDescription>
            Runtime availability and operational health overview.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2">
            {runtimeOptions.map((option) => (
              <div key={option.runtime} className="flex items-center justify-between rounded-md border p-3">
                <span className="text-sm font-medium">{option.label}</span>
                <Badge variant={option.available ? "default" : "secondary"}>
                  {option.available ? "Ready" : "Unavailable"}
                </Badge>
              </div>
            ))}
            {runtimeOptions.length === 0 && (
              <p className="text-sm text-muted-foreground">
                No runtime information available.
              </p>
            )}
          </div>
          <div className="flex gap-3">
            <Button asChild size="sm" variant="outline">
              <Link href="/agents">View Agent Pool</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link href="/reviews">View Review Backlog</Link>
            </Button>
          </div>
        </CardContent>
      </Card>

      <Separator />

      <div className="flex items-center gap-3">
        <Button type="button" onClick={() => void handleSave()}>
          Save Settings
        </Button>
        {saved && (
          <span className="text-sm text-emerald-600 dark:text-emerald-400">
            Settings saved
          </span>
        )}
      </div>
    </div>
  );
}
