"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import dynamic from "next/dynamic";
import { Save, Send, Archive, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import {
  useQianchuanStrategiesStore,
  type StrategyStatus,
  type StrategyParseError,
} from "@/lib/stores/qianchuan-strategies-store";

// Lazy-load the Monaco editor; SSR is disabled because the Monaco runtime
// touches the DOM at import time.
const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

interface MonacoLikeEditor {
  getModel: () => unknown;
}
interface MonacoLikeNS {
  editor: { setModelMarkers: (model: unknown, owner: string, markers: unknown[]) => void };
}

export default function QianchuanStrategyEditPage() {
  const params = useParams<{ id: string; sid: string }>();
  const router = useRouter();
  const projectId = params?.id ?? "";
  const sid = params?.sid ?? "";
  const isNew = sid === "new";
  const t = useTranslations("qianchuan");

  useBreadcrumbs([
    { label: "Projects", href: "/projects" },
    { label: projectId, href: `/projects/${projectId}` },
    { label: t("title"), href: `/projects/${projectId}/qianchuan/strategies` },
    { label: isNew ? t("edit.newPlaceholder") : sid },
  ]);

  const { selected, lastError, lastTestResult, fetchOne, update, publish, archive, testRun, clearError } =
    useQianchuanStrategiesStore();

  function statusBadge(status: StrategyStatus) {
    const label = t(`status.${status}` as `status.${StrategyStatus}`);
    switch (status) {
      case "draft":
        return <Badge variant="secondary">{label}</Badge>;
      case "published":
        return <Badge>{label}</Badge>;
      case "archived":
        return <Badge variant="outline">{label}</Badge>;
    }
  }

  const [yamlSource, setYamlSource] = useState<string | null>(null);
  const [snapshotJSON, setSnapshotJSON] = useState("{\n  \"metrics\": { \"cost\": 0 }\n}");
  const [snapshotParseError, setSnapshotParseError] = useState<string | null>(null);
  const editorRef = useRef<MonacoLikeEditor | null>(null);
  const monacoRef = useRef<MonacoLikeNS | null>(null);

  useEffect(() => {
    if (!isNew && sid) {
      void fetchOne(sid);
    }
  }, [isNew, sid, fetchOne]);

  // Derive the editor's current value from local state (when the user has
  // typed) or the loaded strategy. This avoids a setState-in-effect loop.
  const effectiveYAML = yamlSource ?? selected?.yamlSource ?? "";

  // When a save returns a structured StrategyParseError, drop a Monaco
  // marker on the offending line.
  useEffect(() => {
    const editor = editorRef.current;
    const monaco = monacoRef.current;
    if (!editor || !monaco) return;
    if (lastError && typeof lastError === "object" && "line" in lastError) {
      const e = lastError as StrategyParseError;
      monaco.editor.setModelMarkers(editor.getModel(), "strategy", [
        {
          startLineNumber: e.line || 1,
          startColumn: e.col || 1,
          endLineNumber: e.line || 1,
          endColumn: (e.col || 1) + 10,
          message: `${e.field ? e.field + ": " : ""}${e.msg}`,
          severity: 8, // monaco.MarkerSeverity.Error
        },
      ]);
    } else if (editor && monaco) {
      monaco.editor.setModelMarkers(editor.getModel(), "strategy", []);
    }
  }, [lastError]);

  const isReadOnly = useMemo(() => {
    if (!selected) return false;
    return selected.isSystem || selected.status === "archived";
  }, [selected]);

  const onSave = async () => {
    if (!selected) return;
    await update(selected.id, effectiveYAML);
  };

  const onPublish = async () => {
    if (!selected) return;
    const result = await publish(selected.id);
    if (result) router.push(`/projects/${projectId}/qianchuan/strategies`);
  };

  const onArchive = async () => {
    if (!selected) return;
    await archive(selected.id);
  };

  const onRunTest = async () => {
    if (!selected) return;
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(snapshotJSON);
    } catch (err) {
      setSnapshotParseError(t("edit.snapshotJSONError", { message: (err as Error).message }));
      return;
    }
    setSnapshotParseError(null);
    clearError();
    await testRun(selected.id, parsed);
  };

  const description = selected
    ? `${t("edit.metaVersion", { version: selected.version })} · ${
        selected.isSystem ? t("edit.metaSystem") : t("edit.metaProject")
      }`
    : t("description");

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader
        title={selected?.name ?? t("edit.newPlaceholder")}
        description={description}
        actions={
          <div className="flex gap-2 items-center">
            {selected ? statusBadge(selected.status) : null}
            {selected && !selected.isSystem && selected.status === "draft" ? (
              <Button onClick={onPublish} size="sm">
                <Send className="mr-1 size-4" />
                {t("buttons.publish")}
              </Button>
            ) : null}
            {selected && !selected.isSystem && selected.status === "published" ? (
              <Button onClick={onArchive} size="sm" variant="outline">
                <Archive className="mr-1 size-4" />
                {t("buttons.archive")}
              </Button>
            ) : null}
            {!isReadOnly ? (
              <Button onClick={onSave} size="sm" variant="default">
                <Save className="mr-1 size-4" />
                {t("buttons.save")}
              </Button>
            ) : null}
          </div>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-[var(--space-section-gap)]">
        <SectionCard title={t("edit.editorTitle")} description={t("edit.editorDescription")}>
          <div className="h-[480px] border-t border-border">
            <MonacoEditor
              height="100%"
              language="yaml"
              value={effectiveYAML}
              onChange={(next) => setYamlSource(next ?? "")}
              options={{ readOnly: isReadOnly, minimap: { enabled: false } }}
              onMount={(editor, monaco) => {
                editorRef.current = editor as unknown as MonacoLikeEditor;
                monacoRef.current = monaco as unknown as MonacoLikeNS;
              }}
            />
          </div>
        </SectionCard>

        <SectionCard title={t("edit.testTitle")} description={t("edit.testDescription")}>
          <div className="flex flex-col gap-3 p-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="snapshot">{t("edit.snapshotLabel")}</Label>
              <Textarea
                id="snapshot"
                value={snapshotJSON}
                onChange={(e) => setSnapshotJSON(e.target.value)}
                rows={8}
                className="font-mono text-xs"
              />
              {snapshotParseError ? (
                <p className="text-xs text-destructive">{snapshotParseError}</p>
              ) : null}
            </div>
            <Button onClick={onRunTest} size="sm" variant="default" className="self-start">
              <Play className="mr-1 size-4" />
              {t("buttons.run")}
            </Button>

            {lastTestResult ? (
              <div className="flex flex-col gap-2 mt-2">
                <p className="text-xs text-muted-foreground">
                  {t("edit.firedRulesLabel")}:{" "}
                  {(lastTestResult.fired_rules ?? []).join(", ") || t("edit.firedRulesNone")}
                </p>
                <ul className="flex flex-col gap-2">
                  {(lastTestResult.actions ?? []).map((a, idx) => (
                    <li key={idx} className="rounded-md border border-border p-2 text-xs">
                      <div className="flex items-center gap-2">
                        <Badge variant="secondary">{a.type}</Badge>
                        <span className="font-mono text-muted-foreground">{a.rule}</span>
                        {a.ad_id ? <span className="text-muted-foreground">→ {a.ad_id}</span> : null}
                      </div>
                      {a.params ? (
                        <pre className="mt-1 whitespace-pre-wrap break-all">{JSON.stringify(a.params, null, 2)}</pre>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        </SectionCard>
      </div>
    </div>
  );
}
