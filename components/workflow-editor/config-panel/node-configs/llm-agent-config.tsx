"use client";

import React from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";

// ── Constants ─────────────────────────────────────────────────────────────────

const RUNTIMES = [
  "claude_code",
  "codex",
  "cursor",
  "gemini",
  "opencode",
  "qoder",
] as const;

const PROVIDERS = ["anthropic", "openai", "google"] as const;

const MODELS_BY_PROVIDER: Record<string, string[]> = {
  anthropic: ["claude-opus-4-6", "claude-sonnet-4-6", "claude-haiku-4-5"],
  openai: ["gpt-4o", "gpt-4o-mini", "o3"],
  google: ["gemini-2.5-pro", "gemini-2.5-flash"],
};

// ── Types ─────────────────────────────────────────────────────────────────────

interface LlmAgentConfigProps {
  config: Record<string, unknown>;
  onChange: (config: Record<string, unknown>) => void;
}

// ── Component ─────────────────────────────────────────────────────────────────

export function LlmAgentConfig({ config, onChange }: LlmAgentConfigProps) {
  const runtime = (config.runtime as string | undefined) ?? "";
  const provider = (config.provider as string | undefined) ?? "";
  const model = (config.model as string | undefined) ?? "";
  const budgetUsd = (config.budgetUsd as string | number | undefined) ?? "";
  const prompt = (config.prompt as string | undefined) ?? "";
  const systemPrompt = (config.systemPrompt as string | undefined) ?? "";

  const availableModels = MODELS_BY_PROVIDER[provider] ?? [];

  function update(partial: Record<string, unknown>) {
    onChange({ ...config, ...partial });
  }

  function handleProviderChange(value: string) {
    const models = MODELS_BY_PROVIDER[value] ?? [];
    update({ provider: value, model: models[0] ?? "" });
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Runtime */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Runtime</Label>
        <Select value={runtime} onValueChange={(v) => update({ runtime: v })}>
          <SelectTrigger>
            <SelectValue placeholder="Select runtime…" />
          </SelectTrigger>
          <SelectContent>
            {RUNTIMES.map((r) => (
              <SelectItem key={r} value={r}>
                {r}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Provider */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Provider</Label>
        <Select value={provider} onValueChange={handleProviderChange}>
          <SelectTrigger>
            <SelectValue placeholder="Select provider…" />
          </SelectTrigger>
          <SelectContent>
            {PROVIDERS.map((p) => (
              <SelectItem key={p} value={p}>
                {p}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Model */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Model</Label>
        <Select
          value={model}
          onValueChange={(v) => update({ model: v })}
          disabled={availableModels.length === 0}
        >
          <SelectTrigger>
            <SelectValue placeholder="Select model…" />
          </SelectTrigger>
          <SelectContent>
            {availableModels.map((m) => (
              <SelectItem key={m} value={m}>
                {m}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Budget USD */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Budget (USD)</Label>
        <Input
          type="number"
          min={0}
          step={0.01}
          placeholder="e.g. 1.00"
          value={String(budgetUsd)}
          onChange={(e) => update({ budgetUsd: e.target.value })}
        />
      </div>

      {/* Prompt */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Prompt</Label>
        <Textarea
          rows={4}
          placeholder="User prompt template…"
          value={prompt}
          onChange={(e) => update({ prompt: e.target.value })}
        />
      </div>

      {/* System Prompt */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">System Prompt</Label>
        <Textarea
          rows={4}
          placeholder="System prompt template…"
          value={systemPrompt}
          onChange={(e) => update({ systemPrompt: e.target.value })}
        />
      </div>
    </div>
  );
}
