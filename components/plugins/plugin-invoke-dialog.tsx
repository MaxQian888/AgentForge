"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { usePluginStore } from "@/lib/stores/plugin-store";
import type { PluginRecord } from "@/lib/stores/plugin-store";

interface InvokeFormProps {
  plugin: PluginRecord;
  onClose: () => void;
}

function InvokeForm({ plugin, onClose }: InvokeFormProps) {
  const [operation, setOperation] = useState("");
  const [payloadText, setPayloadText] = useState("{}");
  const [parseError, setParseError] = useState<string | null>(null);
  const [result, setResult] = useState<Record<string, unknown> | null>(null);
  const [invokeError, setInvokeError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const invokePlugin = usePluginStore((s) => s.invokePlugin);

  const handleSubmit = async () => {
    setParseError(null);
    setInvokeError(null);
    setResult(null);

    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(payloadText) as Record<string, unknown>;
    } catch {
      setParseError("Invalid JSON payload");
      return;
    }

    if (!operation.trim()) {
      setInvokeError("Operation name is required");
      return;
    }

    setSubmitting(true);
    try {
      const res = await invokePlugin(
        plugin.metadata.id,
        operation.trim(),
        parsed,
      );
      setResult(res);
    } catch (err) {
      setInvokeError(
        err instanceof Error ? err.message : "Invocation failed",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <div className="grid gap-3 py-4">
        <div className="grid gap-1.5">
          <Label htmlFor="invoke-operation">Operation</Label>
          <Input
            id="invoke-operation"
            placeholder="e.g. run, execute, ping"
            value={operation}
            onChange={(e) => setOperation(e.target.value)}
          />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="invoke-payload">Payload (JSON)</Label>
          <textarea
            id="invoke-payload"
            className="flex min-h-[140px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 font-mono"
            value={payloadText}
            onChange={(e) => {
              setPayloadText(e.target.value);
              setParseError(null);
            }}
          />
          {parseError ? (
            <p className="text-sm text-destructive">{parseError}</p>
          ) : null}
        </div>
        {invokeError ? (
          <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {invokeError}
          </div>
        ) : null}
        {result !== null ? (
          <div className="grid gap-1.5">
            <Label>Result</Label>
            <pre className="max-h-[200px] overflow-auto rounded-md border border-border/60 bg-muted/30 p-3 text-xs font-mono">
              {JSON.stringify(result, null, 2)}
            </pre>
          </div>
        ) : null}
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onClose}>
          Close
        </Button>
        <Button onClick={() => void handleSubmit()} disabled={submitting}>
          {submitting ? "Invoking..." : "Submit"}
        </Button>
      </DialogFooter>
    </>
  );
}

interface PluginInvokeDialogProps {
  plugin: PluginRecord | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function PluginInvokeDialog({
  plugin,
  open,
  onOpenChange,
}: PluginInvokeDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            Invoke {plugin?.metadata.name ?? "Plugin"}
          </DialogTitle>
          <DialogDescription>
            Send an operation request to the plugin runtime. Provide an
            operation name and an optional JSON payload.
          </DialogDescription>
        </DialogHeader>
        {plugin ? (
          <InvokeForm
            key={plugin.metadata.id}
            plugin={plugin}
            onClose={() => onOpenChange(false)}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
