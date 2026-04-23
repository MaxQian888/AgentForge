"use client";

import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";

type KV = { k: string; v: string };
type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

interface Props {
  config: Record<string, unknown>;
  onChange: (c: Record<string, unknown>) => void;
}

function fromObject(o: unknown): KV[] {
  if (!o || typeof o !== "object") return [];
  return Object.entries(o as Record<string, string>).map(([k, v]) => ({ k, v }));
}
function toObject(rows: KV[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const r of rows) if (r.k.trim()) out[r.k] = r.v;
  return out;
}

export function HTTPCallConfig({ config, onChange }: Props) {
  const t = useTranslations("workflow");
  const method = (config.method as Method | undefined) ?? "GET";
  const url = (config.url as string | undefined) ?? "";
  const body = (config.body as string | undefined) ?? "";
  const timeout = (config.timeout_seconds as number | undefined) ?? 30;
  const treatAsSuccess = ((config.treat_as_success as number[] | undefined) ?? []).join(",");

  const [headers, setHeaders] = useState<KV[]>(fromObject(config.headers));
  const [query, setQuery] = useState<KV[]>(fromObject(config.url_query));

  function update(patch: Record<string, unknown>) {
    onChange({ ...config, ...patch });
  }

  function updateHeaders(next: KV[]) {
    setHeaders(next);
    update({ headers: toObject(next) });
  }
  function updateQuery(next: KV[]) {
    setQuery(next);
    update({ url_query: toObject(next) });
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("nodeConfig.httpCall.method")}</Label>
        <Select value={method} onValueChange={(v) => update({ method: v })}>
          <SelectTrigger><SelectValue /></SelectTrigger>
          <SelectContent>
            {(["GET","POST","PUT","PATCH","DELETE"] as Method[]).map((m) => (
              <SelectItem key={m} value={m}>{m}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("nodeConfig.httpCall.url")}</Label>
        <Input value={url} onChange={(e) => update({ url: e.target.value })}
          placeholder={t("nodeConfig.httpCall.urlPlaceholder")} />
        <p className="text-[11px] text-muted-foreground">{t("nodeConfig.httpCall.secretsHelp")}</p>
      </div>

      <KVEditor label={t("nodeConfig.httpCall.headers")} rows={headers} onChange={updateHeaders}
        keyPlaceholder={t("nodeConfig.httpCall.headerKeyPlaceholder")} valuePlaceholder={t("nodeConfig.httpCall.headerValuePlaceholder")} />

      <KVEditor label={t("nodeConfig.httpCall.urlQuery")} rows={query} onChange={updateQuery}
        keyPlaceholder={t("nodeConfig.httpCall.queryKeyPlaceholder")} valuePlaceholder={t("nodeConfig.httpCall.queryValuePlaceholder")} />

      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("nodeConfig.httpCall.body")}</Label>
        <Textarea rows={5} value={body}
          onChange={(e) => update({ body: e.target.value })}
          placeholder={t("nodeConfig.httpCall.bodyPlaceholder")}
          className="font-mono text-xs" />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("nodeConfig.httpCall.timeout")}</Label>
        <Input type="number" value={timeout}
          onChange={(e) => update({ timeout_seconds: Number(e.target.value) })} />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("nodeConfig.httpCall.treatAsSuccess")}</Label>
        <Input value={treatAsSuccess}
          onChange={(e) => {
            const arr = e.target.value.split(",").map((s) => Number(s.trim())).filter((n) => !Number.isNaN(n));
            update({ treat_as_success: arr });
          }}
          placeholder={t("nodeConfig.httpCall.treatAsSuccessPlaceholder")} />
      </div>
    </div>
  );
}

function KVEditor({
  label, rows, onChange, keyPlaceholder, valuePlaceholder,
}: {
  label: string; rows: KV[]; onChange: (rows: KV[]) => void;
  keyPlaceholder: string; valuePlaceholder: string;
}) {
  const t = useTranslations("workflow");
  return (
    <div className="flex flex-col gap-1.5">
      <Label className="text-xs">{label}</Label>
      <div className="flex flex-col gap-1.5">
        {rows.map((r, i) => (
          <div key={i} className="flex gap-1.5">
            <Input className="flex-1" placeholder={keyPlaceholder} value={r.k}
              onChange={(e) => {
                const next = rows.slice();
                next[i] = { ...r, k: e.target.value };
                onChange(next);
              }} />
            <Input className="flex-1" placeholder={valuePlaceholder} value={r.v}
              onChange={(e) => {
                const next = rows.slice();
                next[i] = { ...r, v: e.target.value };
                onChange(next);
              }} />
            <Button variant="ghost" size="icon" onClick={() => onChange(rows.filter((_, idx) => idx !== i))}>
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        ))}
        <Button variant="ghost" size="sm" className="self-start"
          onClick={() => onChange([...rows, { k: "", v: "" }])}>
          <Plus className="mr-1 h-3.5 w-3.5" /> {t("nodeConfig.httpCall.addRow", { label: label.toLowerCase().replace(/s$/, "") })}
        </Button>
      </div>
    </div>
  );
}
