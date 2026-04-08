"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import {
  CheckCircle2,
  Copy,
  Check,
  Eye,
  EyeOff,
  Info,
  Plus,
  Trash2,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useRuntimeConfigStore } from "@/lib/stores/runtime-config-store";

interface RuntimeProfile {
  key: string;
  label: string;
  adapter_family: "dedicated" | "cli";
  default_provider: string;
  compatible_providers: string[];
  default_model?: string;
  model_options?: string[];
  strict_model_options?: boolean;
  command?: {
    default_command?: string;
    env_var?: string;
    install_hint?: string;
  };
  auth?: {
    mode?: string;
    env_vars?: string[];
    message?: string;
  };
  supported_features: string[];
}

const RUNTIME_PROFILES: RuntimeProfile[] = [
  {
    key: "claude_code",
    label: "Claude Code",
    adapter_family: "dedicated",
    default_provider: "anthropic",
    compatible_providers: ["anthropic"],
    default_model: "claude-sonnet-4-5",
    model_options: ["claude-sonnet-4-5", "claude-opus-4-1", "claude-haiku-4-5"],
    strict_model_options: false,
    command: { default_command: "claude", env_var: "CLAUDE_CODE_RUNTIME_COMMAND", install_hint: "Install Claude Code CLI." },
    auth: { mode: "env_any", env_vars: ["ANTHROPIC_API_KEY"], message: "Anthropic API key is required." },
    supported_features: ["structured_output", "agents", "hooks", "thinking", "file_checkpointing", "elicitation", "permissions", "partial_messages", "fallback_model", "env", "rate_limit", "progress", "interrupt", "set_model", "fork", "rollback"],
  },
  {
    key: "codex",
    label: "Codex",
    adapter_family: "dedicated",
    default_provider: "openai",
    compatible_providers: ["openai", "codex"],
    default_model: "gpt-5-codex",
    model_options: ["gpt-5-codex", "o3", "gpt-4.1"],
    strict_model_options: false,
    command: { default_command: "codex", env_var: "CODEX_RUNTIME_COMMAND", install_hint: "Install Codex CLI." },
    auth: { mode: "env_any", env_vars: ["OPENAI_API_KEY"], message: "Codex CLI must be authenticated via `codex login`." },
    supported_features: ["progress", "reasoning", "file_change", "mcp_tool_call", "web_search", "todo_update", "output_schema", "image_attachments", "env", "mcp_config", "fork"],
  },
  {
    key: "opencode",
    label: "OpenCode",
    adapter_family: "dedicated",
    default_provider: "opencode",
    compatible_providers: ["opencode"],
    default_model: "opencode-default",
    model_options: ["opencode-default"],
    strict_model_options: false,
    command: { default_command: "opencode", env_var: "OPENCODE_RUNTIME_COMMAND", install_hint: "Install OpenCode CLI or configure the OpenCode server." },
    auth: { mode: "none", env_vars: [], message: "" },
    supported_features: ["fork", "revert", "diff", "todo_update", "messages", "command", "permission_response", "agents", "skills", "session_status", "reasoning", "file_change", "set_model"],
  },
  {
    key: "cursor",
    label: "Cursor Agent",
    adapter_family: "cli",
    default_provider: "cursor",
    compatible_providers: ["cursor"],
    default_model: "claude-sonnet-4-20250514",
    model_options: ["claude-sonnet-4-20250514", "claude-opus-4-20250514", "gpt-4o", "gemini-2.5-pro", "cursor-small"],
    strict_model_options: true,
    command: { default_command: "cursor-agent", env_var: "CURSOR_RUNTIME_COMMAND", install_hint: "Install Cursor Agent CLI from Cursor or npm." },
    auth: { mode: "env_any", env_vars: ["CURSOR_API_KEY"], message: "Cursor Agent authentication is unavailable." },
    supported_features: ["progress", "reasoning"],
  },
  {
    key: "gemini",
    label: "Gemini CLI",
    adapter_family: "cli",
    default_provider: "google",
    compatible_providers: ["google", "vertex"],
    default_model: "gemini-2.5-pro",
    model_options: ["gemini-3.1-pro-preview", "gemini-3-flash-preview", "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite"],
    strict_model_options: true,
    command: { default_command: "gemini", env_var: "GEMINI_RUNTIME_COMMAND", install_hint: "Install Gemini CLI with npm." },
    auth: { mode: "env_any", env_vars: ["GEMINI_API_KEY", "GOOGLE_API_KEY"], message: "Gemini CLI authentication is unavailable." },
    supported_features: ["reasoning", "plan_mode", "progress"],
  },
  {
    key: "qoder",
    label: "Qoder CLI",
    adapter_family: "cli",
    default_provider: "qoder",
    compatible_providers: ["qoder"],
    default_model: "auto",
    model_options: ["auto", "ultimate", "performance", "efficient", "lite"],
    strict_model_options: true,
    command: { default_command: "qodercli", env_var: "QODER_RUNTIME_COMMAND", install_hint: "Install Qoder CLI from qoder.com." },
    auth: { mode: "none", env_vars: [], message: "" },
    supported_features: ["progress"],
  },
  {
    key: "iflow",
    label: "iFlow CLI",
    adapter_family: "cli",
    default_provider: "iflow",
    compatible_providers: ["iflow"],
    default_model: "Qwen3-Coder",
    model_options: ["Qwen3-Coder", "Kimi-K2.5", "DeepSeek-v3"],
    strict_model_options: true,
    command: { default_command: "iflow", env_var: "IFLOW_RUNTIME_COMMAND", install_hint: "Install iFlow CLI with npm." },
    auth: { mode: "env_any", env_vars: ["IFLOW_API_KEY", "IFLOW_apiKey"], message: "iFlow CLI authentication is unavailable." },
    supported_features: ["plan_mode", "auto_edit", "progress"],
  },
];

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };
  return (
    <Button type="button" variant="ghost" size="icon" className="size-7" onClick={handleCopy}>
      {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
    </Button>
  );
}

function SecretInput({ value, onChange, placeholder }: { value: string; onChange: (v: string) => void; placeholder?: string }) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="flex gap-1.5">
      <Input
        type={visible ? "text" : "password"}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="flex-1"
      />
      <Button type="button" variant="ghost" size="icon" className="size-9 shrink-0" onClick={() => setVisible(!visible)}>
        {visible ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
      </Button>
    </div>
  );
}

function FeatureBadges({ features }: { features: string[] }) {
  const t = useTranslations("settings.runtimeConfig");
  if (features.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("noFeatures")}</p>;
  }
  return (
    <div className="flex flex-wrap gap-1.5">
      {features.map((f) => (
        <Badge key={f} variant="outline" className="text-xs font-normal">
          {f.replace(/_/g, " ")}
        </Badge>
      ))}
    </div>
  );
}

function ExtraEnvEditor({ runtimeKey }: { runtimeKey: string }) {
  const t = useTranslations("settings.runtimeConfig");
  const config = useRuntimeConfigStore((s) => s.getRuntime(runtimeKey));
  const setExtraEnv = useRuntimeConfigStore((s) => s.setRuntimeExtraEnv);
  const removeExtraEnv = useRuntimeConfigStore((s) => s.removeRuntimeExtraEnv);
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");

  const entries = Object.entries(config.extraEnv);

  const handleAdd = () => {
    const trimmedKey = newKey.trim();
    if (!trimmedKey) return;
    setExtraEnv(runtimeKey, trimmedKey, newValue);
    setNewKey("");
    setNewValue("");
  };

  return (
    <div className="space-y-3">
      {entries.map(([k, v]) => (
        <div key={k} className="flex items-center gap-2">
          <code className="min-w-[120px] shrink-0 rounded bg-muted px-2 py-1 text-xs">{k}</code>
          <Input
            value={v}
            onChange={(e) => setExtraEnv(runtimeKey, k, e.target.value)}
            className="flex-1 text-sm"
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-8 shrink-0 text-destructive hover:text-destructive"
            onClick={() => removeExtraEnv(runtimeKey, k)}
          >
            <Trash2 className="size-3.5" />
          </Button>
        </div>
      ))}
      <div className="flex items-end gap-2">
        <div className="flex-1 space-y-1">
          <Label className="text-xs">{t("envVarKey")}</Label>
          <Input
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            placeholder="MY_ENV_VAR"
            className="text-sm"
          />
        </div>
        <div className="flex-1 space-y-1">
          <Label className="text-xs">{t("envVarValue")}</Label>
          <Input
            value={newValue}
            onChange={(e) => setNewValue(e.target.value)}
            placeholder="value"
            className="text-sm"
          />
        </div>
        <Button type="button" variant="outline" size="sm" className="shrink-0" onClick={handleAdd} disabled={!newKey.trim()}>
          <Plus className="mr-1 size-3.5" />
          {t("addEnvVar")}
        </Button>
      </div>
    </div>
  );
}

export function SectionRuntimeDetail({ runtimeKey }: { runtimeKey: string }) {
  const t = useTranslations("settings.runtimeConfig");
  const profile = RUNTIME_PROFILES.find((p) => p.key === runtimeKey);
  const config = useRuntimeConfigStore((s) => s.getRuntime(runtimeKey));
  const setField = useRuntimeConfigStore((s) => s.setRuntimeField);

  if (!profile) {
    return <p className="text-sm text-muted-foreground">Unknown runtime: {runtimeKey}</p>;
  }

  const authRequired = profile.auth?.mode === "env_any" && (profile.auth.env_vars?.length ?? 0) > 0;
  const isOpenCode = runtimeKey === "opencode";

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">{profile.label}</h2>
          <p className="text-sm text-muted-foreground">
            {profile.adapter_family === "dedicated" ? t("adapterDedicated") : t("adapterCli")}
            {" \u00b7 "}
            {profile.default_provider}
            {profile.default_model && ` \u00b7 ${profile.default_model}`}
          </p>
        </div>
        <Badge variant="outline" className="text-xs">
          {profile.adapter_family === "dedicated" ? t("adapterDedicated") : t("adapterCli")}
        </Badge>
      </div>

      {/* Overview card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("runtimeOverview")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1">
              <Label className="text-xs text-muted-foreground">{t("defaultProvider")}</Label>
              <p className="text-sm font-medium">{profile.default_provider}</p>
            </div>
            <div className="space-y-1">
              <Label className="text-xs text-muted-foreground">{t("defaultModel")}</Label>
              <p className="text-sm font-medium">{profile.default_model ?? "—"}</p>
            </div>
            <div className="space-y-1 sm:col-span-2">
              <Label className="text-xs text-muted-foreground">{t("modelOptions")}</Label>
              <div className="flex flex-wrap gap-1.5">
                {(profile.model_options ?? []).map((m) => (
                  <Badge key={m} variant="secondary" className="text-xs font-mono">
                    {m}
                  </Badge>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">
                {profile.strict_model_options ? t("strictModelsDesc") : t("flexibleModelsDesc")}
              </p>
            </div>
            {profile.compatible_providers.length > 1 && (
              <div className="space-y-1 sm:col-span-2">
                <Label className="text-xs text-muted-foreground">Compatible Providers</Label>
                <div className="flex gap-1.5">
                  {profile.compatible_providers.map((p) => (
                    <Badge key={p} variant="outline" className="text-xs">{p}</Badge>
                  ))}
                </div>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Configuration card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("configSection")}</CardTitle>
          <CardDescription>{t("savedLocally")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          {/* Auth / API Key */}
          <div>
            <div className="mb-3 flex items-center gap-2">
              <h3 className="text-sm font-medium">{t("authSection")}</h3>
              <Badge variant={authRequired ? "default" : "secondary"} className="text-xs">
                {authRequired ? t("authRequired") : t("authNotRequired")}
              </Badge>
            </div>

            {authRequired ? (
              <div className="space-y-3">
                <div className="flex items-start gap-2 rounded-md border border-amber-500/30 bg-amber-500/5 p-3 text-sm">
                  <Info className="mt-0.5 size-4 shrink-0 text-amber-600" />
                  <div>
                    <p className="font-medium">{t("authEnvVars")}: {profile.auth!.env_vars!.join(" / ")}</p>
                    <p className="text-muted-foreground">{t("authEnvVarsDesc")}</p>
                    {profile.auth!.message && (
                      <p className="mt-1 text-xs text-muted-foreground">{profile.auth!.message}</p>
                    )}
                  </div>
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label className="text-sm">{t("apiKey")}</Label>
                  <SecretInput
                    value={config.apiKey}
                    onChange={(v) => setField(runtimeKey, "apiKey", v)}
                    placeholder={t("apiKeyPlaceholder")}
                  />
                  <p className="text-xs text-muted-foreground">{t("apiKeyDesc")}</p>
                </div>
              </div>
            ) : (
              <div className="flex items-center gap-2 rounded-md border bg-muted/50 p-3 text-sm text-muted-foreground">
                <CheckCircle2 className="size-4 shrink-0 text-emerald-500" />
                {t("noAuthNeeded")}
              </div>
            )}
          </div>

          <Separator />

          {/* CLI command */}
          {profile.command && (
            <div className="space-y-3">
              <div className="flex flex-col gap-1.5">
                <Label className="text-sm">{t("cliCommand")}</Label>
                <div className="flex items-center gap-2">
                  <Input
                    value={config.commandPath}
                    onChange={(e) => setField(runtimeKey, "commandPath", e.target.value)}
                    placeholder={profile.command.default_command ?? t("cliCommandPlaceholder")}
                    className="flex-1"
                  />
                  {profile.command.default_command && (
                    <code className="shrink-0 rounded bg-muted px-2 py-1 text-xs text-muted-foreground">
                      default: {profile.command.default_command}
                    </code>
                  )}
                </div>
                <p className="text-xs text-muted-foreground">{t("cliCommandDesc")}</p>
              </div>
              {profile.command.env_var && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span>{t("cliEnvVar")}:</span>
                  <code className="rounded bg-muted px-1.5 py-0.5">{profile.command.env_var}</code>
                  <CopyButton text={profile.command.env_var} />
                </div>
              )}
              {profile.command.install_hint && (
                <div className="flex items-start gap-2 rounded-md border bg-muted/50 p-3 text-xs text-muted-foreground">
                  <Info className="mt-0.5 size-3.5 shrink-0" />
                  <span>{t("installHint")}: {profile.command.install_hint}</span>
                </div>
              )}
            </div>
          )}

          {/* Server URL (OpenCode) */}
          {isOpenCode && (
            <>
              <Separator />
              <div className="flex flex-col gap-1.5">
                <Label className="text-sm">{t("serverUrl")}</Label>
                <Input
                  value={config.serverUrl}
                  onChange={(e) => setField(runtimeKey, "serverUrl", e.target.value)}
                  placeholder={t("serverUrlPlaceholder")}
                />
                <p className="text-xs text-muted-foreground">{t("serverUrlDesc")}</p>
              </div>
            </>
          )}

          <Separator />

          {/* Extra env vars */}
          <div>
            <h3 className="mb-3 text-sm font-medium">{t("envVarsSection")}</h3>
            <ExtraEnvEditor runtimeKey={runtimeKey} />
          </div>
        </CardContent>
      </Card>

      {/* Features card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("featuresSection")}</CardTitle>
        </CardHeader>
        <CardContent>
          <FeatureBadges features={profile.supported_features} />
        </CardContent>
      </Card>
    </div>
  );
}
