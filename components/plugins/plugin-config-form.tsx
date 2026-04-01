"use client";

import { useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { DialogFooter } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import type { PluginRecord } from "@/lib/stores/plugin-store";

/* ── Schema types (subset of JSON Schema) ── */

interface SchemaProperty {
  type?: string;
  description?: string;
  default?: unknown;
  enum?: string[];
}

interface ConfigSchema {
  type?: string;
  properties?: Record<string, SchemaProperty>;
  required?: string[];
}

/* ── Helpers ── */

function toTitleCase(key: string): string {
  return key
    .replace(/([A-Z])/g, " $1")
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase())
    .trim();
}

const SECRET_PATTERN = /secret|password|token|key/i;

function isSecretField(key: string): boolean {
  return SECRET_PATTERN.test(key);
}

/* ── Props ── */

interface PluginConfigFormProps {
  plugin: PluginRecord;
  onSave: (config: Record<string, unknown>) => void;
  onCancel: () => void;
}

/* ── Schema-driven field ── */

interface SchemaFieldProps {
  fieldKey: string;
  schema: SchemaProperty;
  value: unknown;
  onChange: (value: unknown) => void;
  onReset: (() => void) | null;
}

function SchemaField({
  fieldKey,
  schema,
  value,
  onChange,
  onReset,
}: SchemaFieldProps) {
  const label = toTitleCase(fieldKey);
  const fieldId = `config-field-${fieldKey}`;
  const fieldType = schema.type ?? "string";

  return (
    <div className="grid gap-1.5">
      <div className="flex items-center justify-between">
        <Label htmlFor={fieldId}>{label}</Label>
        {onReset ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-xs text-muted-foreground"
            onClick={onReset}
          >
            Reset to default
          </Button>
        ) : null}
      </div>

      {/* Boolean */}
      {fieldType === "boolean" ? (
        <label
          htmlFor={fieldId}
          className="flex items-center gap-2 text-sm cursor-pointer"
        >
          <input
            id={fieldId}
            type="checkbox"
            checked={Boolean(value)}
            onChange={(e) => onChange(e.target.checked)}
            className="h-4 w-4 rounded border border-input"
          />
          <span className="text-muted-foreground">
            {schema.description ?? label}
          </span>
        </label>
      ) : /* Enum select */
      schema.enum ? (
        <Select
          value={String(value ?? "") || "__none__"}
          onValueChange={(val) => onChange(val === "__none__" ? "" : val)}
        >
          <SelectTrigger id={fieldId} className="w-full">
            <SelectValue placeholder="— Select —" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__none__">— Select —</SelectItem>
            {schema.enum.map((opt) => (
              <SelectItem key={opt} value={opt}>
                {opt}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : /* Number / Integer */
      fieldType === "number" || fieldType === "integer" ? (
        <Input
          id={fieldId}
          type="number"
          value={value === undefined || value === null ? "" : String(value)}
          onChange={(e) => {
            const raw = e.target.value;
            if (raw === "") {
              onChange(undefined);
            } else {
              onChange(
                fieldType === "integer"
                  ? parseInt(raw, 10)
                  : parseFloat(raw)
              );
            }
          }}
        />
      ) : /* String (secret or plain) */
      (
        <Input
          id={fieldId}
          type={isSecretField(fieldKey) ? "password" : "text"}
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
        />
      )}

      {/* Description (shown below the field, except for boolean which shows inline) */}
      {schema.description && fieldType !== "boolean" ? (
        <p className="text-xs text-muted-foreground">{schema.description}</p>
      ) : null}

      {/* Default value hint */}
      {schema.default !== undefined ? (
        <p className="text-xs text-muted-foreground/70">
          Default: {JSON.stringify(schema.default)}
        </p>
      ) : null}
    </div>
  );
}

/* ── JSON fallback textarea ── */

interface JsonFallbackProps {
  configText: string;
  setConfigText: (text: string) => void;
  parseError: string | null;
  setParseError: (err: string | null) => void;
  onSave: (config: Record<string, unknown>) => void;
  onCancel: () => void;
}

function JsonFallback({
  configText,
  setConfigText,
  parseError,
  setParseError,
  onSave,
  onCancel,
}: JsonFallbackProps) {
  const handleSave = () => {
    try {
      const parsed = JSON.parse(configText) as Record<string, unknown>;
      setParseError(null);
      onSave(parsed);
    } catch {
      setParseError("Invalid JSON");
    }
  };

  return (
    <>
      <div className="grid gap-3 py-4">
        <Label htmlFor="config-json">Configuration (JSON)</Label>
        <Textarea
          id="config-json"
          className={cn(
            "flex min-h-[200px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm",
            "ring-offset-background placeholder:text-muted-foreground",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
            "disabled:cursor-not-allowed disabled:opacity-50 font-mono"
          )}
          value={configText}
          onChange={(e) => {
            setConfigText(e.target.value);
            setParseError(null);
          }}
        />
        {parseError ? (
          <p className="text-sm text-destructive">{parseError}</p>
        ) : null}
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button onClick={handleSave}>Save</Button>
      </DialogFooter>
    </>
  );
}

/* ── Main component ── */

export function PluginConfigForm({
  plugin,
  onSave,
  onCancel,
}: PluginConfigFormProps) {
  const schema = plugin.spec.extra?.configSchema as ConfigSchema | undefined;
  const hasSchemaFields =
    schema?.properties && Object.keys(schema.properties).length > 0;

  // Schema-driven state
  const [formValues, setFormValues] = useState<Record<string, unknown>>(
    () => ({ ...(plugin.spec.config ?? {}) })
  );

  // JSON fallback state
  const [configText, setConfigText] = useState(() =>
    JSON.stringify(plugin.spec.config ?? {}, null, 2)
  );
  const [parseError, setParseError] = useState<string | null>(null);

  const updateField = useCallback((key: string, value: unknown) => {
    setFormValues((prev) => ({ ...prev, [key]: value }));
  }, []);

  const resetField = useCallback(
    (key: string, defaultValue: unknown) => {
      setFormValues((prev) => ({ ...prev, [key]: defaultValue }));
    },
    []
  );

  // No schema: show JSON textarea fallback
  if (!hasSchemaFields) {
    return (
      <JsonFallback
        configText={configText}
        setConfigText={setConfigText}
        parseError={parseError}
        setParseError={setParseError}
        onSave={onSave}
        onCancel={onCancel}
      />
    );
  }

  const properties = schema!.properties!;

  const handleSchemaSave = () => {
    onSave({ ...formValues });
  };

  return (
    <>
      <div className="grid gap-4 py-4 max-h-[60vh] overflow-y-auto pr-1">
        {Object.entries(properties).map(([key, prop]) => (
          <SchemaField
            key={key}
            fieldKey={key}
            schema={prop}
            value={formValues[key]}
            onChange={(val) => updateField(key, val)}
            onReset={
              prop.default !== undefined
                ? () => resetField(key, prop.default)
                : null
            }
          />
        ))}
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button onClick={handleSchemaSave}>Save</Button>
      </DialogFooter>
    </>
  );
}
