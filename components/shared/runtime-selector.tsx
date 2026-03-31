"use client";

import { useEffect, useMemo } from "react";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {
  CodingAgentCatalog,
  CodingAgentRuntimeOption,
  CodingAgentSelection,
} from "@/lib/stores/project-store";

const EMPTY_RUNTIME_OPTIONS: CodingAgentRuntimeOption[] = [];

interface RuntimeSelectorProps {
  catalog: CodingAgentCatalog | null | undefined;
  value: CodingAgentSelection;
  onChange: (next: CodingAgentSelection) => void;
  disabled?: boolean;
  idPrefix?: string;
}

function findRuntimeOption(
  catalog: CodingAgentCatalog | null | undefined,
  runtime: string,
): CodingAgentRuntimeOption | undefined {
  if (!catalog) {
    return undefined;
  }
  return catalog.runtimes.find((option) => option.runtime === runtime) ?? catalog.runtimes[0];
}

export function RuntimeSelector({
  catalog,
  value,
  onChange,
  disabled = false,
  idPrefix = "runtime-selector",
}: RuntimeSelectorProps) {
  const runtimeOptions = catalog?.runtimes ?? EMPTY_RUNTIME_OPTIONS;
  const selectedRuntime = useMemo(
    () => findRuntimeOption(catalog, value.runtime),
    [catalog, value.runtime],
  );
  const runtimeDiagnostics = selectedRuntime?.diagnostics ?? [];
  const modelOptions = useMemo(
    () =>
      selectedRuntime?.modelOptions && selectedRuntime.modelOptions.length > 0
        ? selectedRuntime.modelOptions
        : selectedRuntime?.defaultModel
          ? [selectedRuntime.defaultModel]
          : [],
    [selectedRuntime],
  );
  const catalogDiagnostics = useMemo(
    () =>
      runtimeOptions.flatMap((option) =>
        option.diagnostics.map((diagnostic) => ({
          runtime: option.label,
          code: diagnostic.code,
          message: diagnostic.message,
        })),
      ),
    [runtimeOptions],
  );
  const hasBlockingDiagnostics = runtimeDiagnostics.some((item) => item.blocking);

  useEffect(() => {
    if (!catalog || runtimeOptions.length === 0) {
      return;
    }

    const fallback = selectedRuntime ?? runtimeOptions[0];
    const nextRuntime = value.runtime || fallback?.runtime || "";
    const nextProvider =
      selectedRuntime?.compatibleProviders.includes(value.provider)
        ? value.provider
        : fallback?.defaultProvider || "";
    const nextModel =
      modelOptions.includes(value.model)
        ? value.model
        : modelOptions[0] || fallback?.defaultModel || "";

    if (
      nextRuntime !== value.runtime ||
      nextProvider !== value.provider ||
      nextModel !== value.model
    ) {
      onChange({
        runtime: nextRuntime,
        provider: nextProvider,
        model: nextModel,
      });
    }
  }, [catalog, modelOptions, onChange, runtimeOptions, selectedRuntime, value.model, value.provider, value.runtime]);

  const handleRuntimeChange = (runtime: string) => {
    const nextRuntime = runtimeOptions.find((option) => option.runtime === runtime);
    onChange({
      runtime,
      provider: nextRuntime?.defaultProvider ?? "",
      model:
        nextRuntime?.modelOptions && nextRuntime.modelOptions.length > 0
          ? nextRuntime.modelOptions[0] ?? ""
          : nextRuntime?.defaultModel ?? "",
    });
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor={`${idPrefix}-runtime`}>Runtime</Label>
        <Select value={value.runtime} onValueChange={handleRuntimeChange} disabled={disabled}>
          <SelectTrigger id={`${idPrefix}-runtime`}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {runtimeOptions.map((option) => (
              <SelectItem key={option.runtime} value={option.runtime} disabled={!option.available}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor={`${idPrefix}-provider`}>Provider</Label>
        <Select
          value={value.provider}
          onValueChange={(provider) => onChange({ ...value, provider })}
          disabled={disabled || !selectedRuntime}
        >
          <SelectTrigger id={`${idPrefix}-provider`}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(selectedRuntime?.compatibleProviders ?? []).map((option) => (
              <SelectItem key={option} value={option}>
                {option}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor={`${idPrefix}-model`}>Model</Label>
        <Select
          value={value.model}
          onValueChange={(model) => onChange({ ...value, model })}
          disabled={disabled || !selectedRuntime}
        >
          <SelectTrigger id={`${idPrefix}-model`}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {modelOptions.map((option) => (
              <SelectItem key={option} value={option}>
                {option}
              </SelectItem>
            ))}
            {value.model && !modelOptions.includes(value.model) ? (
              <SelectItem value={value.model}>{value.model}</SelectItem>
            ) : null}
          </SelectContent>
        </Select>
      </div>

      {runtimeDiagnostics.length > 0 ? (
        <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
          {runtimeDiagnostics.map((diagnostic) => (
            <p key={`${diagnostic.code}-${diagnostic.message}`}>{diagnostic.message}</p>
          ))}
        </div>
      ) : null}

      {catalogDiagnostics.length > 0 ? (
        <div className="rounded-md border p-3 text-sm text-muted-foreground">
          {catalogDiagnostics.map((diagnostic) => (
            <p key={`${diagnostic.runtime}-${diagnostic.code}`}>
              {diagnostic.runtime}: {diagnostic.message}
            </p>
          ))}
        </div>
      ) : null}

      {hasBlockingDiagnostics && selectedRuntime ? (
        <p className="text-xs text-amber-700 dark:text-amber-300">
          {selectedRuntime.label} is currently unavailable.
        </p>
      ) : null}
    </div>
  );
}
