"use client";

import { useMemo } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";

type Target = "reply_to_trigger" | "explicit";
type ActionType = "url" | "callback";

interface CardAction {
  id: string;
  label: string;
  style?: "primary" | "danger" | "default";
  type: ActionType;
  url?: string;
  payload?: Record<string, unknown>;
}

interface CardConfig {
  title?: string;
  status?: "success" | "failed" | "running" | "pending" | "info";
  summary?: string;
  fields?: Array<{ label: string; value: string }>;
  actions?: CardAction[];
  footer?: string;
}

interface Props {
  config: Record<string, unknown>;
  onChange: (c: Record<string, unknown>) => void;
}

const DATASTORE_HELP = "Templating: {{$dataStore.<nodeId>.<field>}} resolves at execution time.";

export function IMSendConfig({ config, onChange }: Props) {
  const target = (config.target as Target | undefined) ?? "reply_to_trigger";
  const explicit = (config.explicit_target as Record<string, string> | undefined) ?? {};
  const card = useMemo<CardConfig>(() => (config.card as CardConfig | undefined) ?? {}, [config.card]);

  function update(patch: Record<string, unknown>) {
    onChange({ ...config, ...patch });
  }
  function updateCard(patch: Partial<CardConfig>) {
    update({ card: { ...card, ...patch } });
  }
  function updateExplicit(patch: Record<string, string>) {
    update({ explicit_target: { ...explicit, ...patch } });
  }

  const actions = card.actions ?? [];
  const fields = card.fields ?? [];

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Target</Label>
        <Select value={target} onValueChange={(v) => update({ target: v })}>
          <SelectTrigger aria-label="Target"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="reply_to_trigger">Reply to triggering message</SelectItem>
            <SelectItem value="explicit">Explicit chat / thread</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {target === "explicit" && (
        <div className="grid grid-cols-2 gap-2">
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Provider</Label>
            <Input value={explicit.provider ?? ""}
              onChange={(e) => updateExplicit({ provider: e.target.value })} placeholder="feishu" />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Chat ID</Label>
            <Input value={explicit.chat_id ?? ""}
              onChange={(e) => updateExplicit({ chat_id: e.target.value })} placeholder="oc_xxx" />
          </div>
          <div className="flex flex-col gap-1.5 col-span-2">
            <Label className="text-xs">Thread ID (optional)</Label>
            <Input value={explicit.thread_id ?? ""}
              onChange={(e) => updateExplicit({ thread_id: e.target.value })} />
          </div>
        </div>
      )}

      <div className="border-t pt-3 flex flex-col gap-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Card</p>
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">Title</Label>
          <Input value={card.title ?? ""} onChange={(e) => updateCard({ title: e.target.value })} />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">Status</Label>
          <Select value={card.status ?? "info"}
            onValueChange={(v) => updateCard({ status: v as CardConfig["status"] })}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              {["info", "success", "failed", "running", "pending"].map((s) => (
                <SelectItem key={s} value={s}>{s}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">Summary</Label>
          <Textarea rows={3} value={card.summary ?? ""}
            onChange={(e) => updateCard({ summary: e.target.value })} />
          <p className="text-[11px] text-muted-foreground">{DATASTORE_HELP}</p>
        </div>

        {/* Fields editor */}
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">Fields</Label>
          {fields.map((f, i) => (
            <div key={i} className="flex gap-1.5">
              <Input className="flex-1" value={f.label} placeholder="Label"
                onChange={(e) => {
                  const next = fields.slice(); next[i] = { ...f, label: e.target.value };
                  updateCard({ fields: next });
                }} />
              <Input className="flex-1" value={f.value} placeholder="Value"
                onChange={(e) => {
                  const next = fields.slice(); next[i] = { ...f, value: e.target.value };
                  updateCard({ fields: next });
                }} />
              <Button variant="ghost" size="icon"
                onClick={() => updateCard({ fields: fields.filter((_, idx) => idx !== i) })}>
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          ))}
          <Button variant="ghost" size="sm" className="self-start"
            onClick={() => updateCard({ fields: [...fields, { label: "", value: "" }] })}>
            <Plus className="mr-1 h-3.5 w-3.5" /> Add field
          </Button>
        </div>

        {/* Actions editor */}
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">Actions (buttons)</Label>
          {actions.map((a, i) => (
            <div key={i} className="border rounded p-2 flex flex-col gap-1.5">
              <div className="flex gap-1.5">
                <Input className="flex-1" placeholder="Action id (e.g. approve)" value={a.id}
                  onChange={(e) => {
                    const next = actions.slice(); next[i] = { ...a, id: e.target.value };
                    updateCard({ actions: next });
                  }} />
                <Input className="flex-1" placeholder="Label" value={a.label}
                  onChange={(e) => {
                    const next = actions.slice(); next[i] = { ...a, label: e.target.value };
                    updateCard({ actions: next });
                  }} />
                <Button variant="ghost" size="icon"
                  onClick={() => updateCard({ actions: actions.filter((_, idx) => idx !== i) })}>
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
              <div className="flex gap-1.5">
                <Select value={a.type} onValueChange={(v) => {
                  const next = actions.slice(); next[i] = { ...a, type: v as ActionType };
                  updateCard({ actions: next });
                }}>
                  <SelectTrigger className="flex-1"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="callback">Callback</SelectItem>
                    <SelectItem value="url">URL</SelectItem>
                  </SelectContent>
                </Select>
                {a.type === "url" ? (
                  <Input className="flex-[2]" placeholder="https://..." value={a.url ?? ""}
                    onChange={(e) => {
                      const next = actions.slice(); next[i] = { ...a, url: e.target.value };
                      updateCard({ actions: next });
                    }} />
                ) : (
                  <Input className="flex-[2]" placeholder='Payload JSON: {"k":"v"}'
                    value={a.payload ? JSON.stringify(a.payload) : ""}
                    onChange={(e) => {
                      const next = actions.slice();
                      try {
                        next[i] = { ...a, payload: JSON.parse(e.target.value || "{}") };
                      } catch {
                        return;
                      }
                      updateCard({ actions: next });
                    }} />
                )}
              </div>
            </div>
          ))}
          <Button variant="ghost" size="sm" className="self-start"
            onClick={() => updateCard({
              actions: [...actions, { id: "", label: "", type: "callback", payload: {} }],
            })}>
            <Plus className="mr-1 h-3.5 w-3.5" /> Add action
          </Button>
        </div>

        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">Footer (optional)</Label>
          <Input value={card.footer ?? ""} onChange={(e) => updateCard({ footer: e.target.value })} />
        </div>
      </div>
    </div>
  );
}
