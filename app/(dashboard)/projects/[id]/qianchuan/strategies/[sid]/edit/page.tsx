"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
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

function statusBadge(status: StrategyStatus) {
  switch (status) {
    case "draft":
      return <Badge variant="secondary">草稿</Badge>;
    case "published":
      return <Badge>已发布</Badge>;
    case "archived":
      return <Badge variant="outline">已归档</Badge>;
  }
}

export default function QianchuanStrategyEditPage() {
  const params = useParams<{ id: string; sid: string }>();
  const router = useRouter();
  const projectId = params?.id ?? "";
  const sid = params?.sid ?? "";
  const isNew = sid === "new";

  useBreadcrumbs([
    { label: "Projects", href: "/projects" },
    { label: projectId, href: `/projects/${projectId}` },
    { label: "Qianchuan Strategies", href: `/projects/${projectId}/qianchuan/strategies` },
    { label: isNew ? "新建" : sid },
  ]);

  const { selected, lastError, lastTestResult, fetchOne, update, publish, archive, testRun, clearError } =
    useQianchuanStrategiesStore();

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
      setSnapshotParseError(`JSON 解析失败: ${(err as Error).message}`);
      return;
    }
    setSnapshotParseError(null);
    clearError();
    await testRun(selected.id, parsed);
  };

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader
        title={selected?.name ?? "新建策略"}
        description={
          selected
            ? `版本 v${selected.version} · ${selected.isSystem ? "系统策略" : "项目策略"}`
            : "撰写 YAML 后保存为草稿。"
        }
        actions={
          <div className="flex gap-2 items-center">
            {selected ? statusBadge(selected.status) : null}
            {selected && !selected.isSystem && selected.status === "draft" ? (
              <Button onClick={onPublish} size="sm">
                <Send className="mr-1 size-4" />
                发布
              </Button>
            ) : null}
            {selected && !selected.isSystem && selected.status === "published" ? (
              <Button onClick={onArchive} size="sm" variant="outline">
                <Archive className="mr-1 size-4" />
                归档
              </Button>
            ) : null}
            {!isReadOnly ? (
              <Button onClick={onSave} size="sm" variant="default">
                <Save className="mr-1 size-4" />
                保存
              </Button>
            ) : null}
          </div>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-[var(--space-section-gap)]">
        <SectionCard title="YAML 编辑器" description="保存时服务端校验，错误会标记在对应行。">
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

        <SectionCard title="测试面板" description="提供快照 JSON，预览策略会触发哪些动作（dry-run，不写库）。">
          <div className="flex flex-col gap-3 p-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="snapshot">snapshot 快照 (JSON)</Label>
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
              运行
            </Button>

            {lastTestResult ? (
              <div className="flex flex-col gap-2 mt-2">
                <p className="text-xs text-muted-foreground">
                  触发规则: {(lastTestResult.fired_rules ?? []).join(", ") || "(无)"}
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
