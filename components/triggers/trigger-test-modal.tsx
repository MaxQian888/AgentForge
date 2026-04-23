"use client";

/**
 * TriggerTestModal — invokes POST /api/v1/triggers/:id/test with a sample
 * event payload and renders the dry-run result. Never dispatches a real
 * workflow execution and never mutates the store.
 */
import { useState } from "react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  useEmployeeTriggerStore,
  type DryRunResult,
} from "@/lib/stores/employee-trigger-store";

interface Props {
  open: boolean;
  triggerId?: string;
  // Default sample event the textarea is pre-filled with. The page passes
  // a platform-specific stub when the trigger source is known.
  initialSample?: string;
  onClose: () => void;
}

const FALLBACK_SAMPLE = JSON.stringify(
  {
    platform: "feishu",
    command: "/echo",
    content: "/echo hi",
    chat_id: "c-1",
    args: ["hi"],
  },
  null,
  2,
);

export function TriggerTestModal({ open, triggerId, initialSample, onClose }: Props) {
  const t = useTranslations("triggers.testModal");
  const testTrigger = useEmployeeTriggerStore((s) => s.testTrigger);
  const [sample, setSample] = useState(initialSample ?? FALLBACK_SAMPLE);
  const [parseErr, setParseErr] = useState<string | null>(null);
  const [result, setResult] = useState<DryRunResult | null>(null);
  const [running, setRunning] = useState(false);
  const [tab, setTab] = useState<"sample" | "result">("sample");

  const [prevTestKey, setPrevTestKey] = useState<string | symbol>(Symbol("init"));
  const testKey = `${open}:${initialSample ?? ""}`;
  if (prevTestKey !== testKey) {
    setPrevTestKey(testKey);
    if (open) {
      setSample(initialSample ?? FALLBACK_SAMPLE);
      setResult(null);
      setParseErr(null);
      setTab("sample");
    }
  }

  const onRun = async () => {
    if (!triggerId) return;
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(sample);
    } catch (err) {
      setParseErr(t("jsonError", { message: (err as Error).message }));
      return;
    }
    setParseErr(null);
    setRunning(true);
    try {
      const res = await testTrigger(triggerId, parsed);
      setResult(res);
      setTab("result");
    } finally {
      setRunning(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        <Tabs value={tab} onValueChange={(v) => setTab(v as "sample" | "result")}>
          <TabsList>
            <TabsTrigger value="sample">{t("tabSample")}</TabsTrigger>
            <TabsTrigger value="result" disabled={result === null}>
              {t("tabResult")}
            </TabsTrigger>
          </TabsList>
          <TabsContent value="sample">
            <Textarea
              value={sample}
              onChange={(e) => setSample(e.target.value)}
              rows={12}
              className="font-mono text-xs"
            />
            {parseErr ? <p className="text-xs text-red-500 mt-2">{parseErr}</p> : null}
          </TabsContent>
          <TabsContent value="result">
            {result ? <ResultView result={result} /> : null}
          </TabsContent>
        </Tabs>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t("close")}
          </Button>
          <Button onClick={onRun} disabled={!triggerId || running}>
            {running ? t("running") : t("runDryRun")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ResultView({ result }: { result: DryRunResult }) {
  const t = useTranslations("triggers.testModal");
  return (
    <div className="space-y-3 text-sm">
      <div className="flex items-center gap-2">
        <span>{t("matched")}</span>
        {result.matched ? (
          <Badge variant="default">{t("matchedYes")}</Badge>
        ) : (
          <Badge variant="secondary">{t("matchedNo")}</Badge>
        )}
        <span className="ml-3">{t("wouldDispatch")}</span>
        {result.would_dispatch ? (
          <Badge variant="default">{t("wouldDispatchYes")}</Badge>
        ) : (
          <Badge variant="outline">{t("wouldDispatchNo")}</Badge>
        )}
      </div>
      {result.skip_reason ? (
        <p className="text-xs text-amber-600 dark:text-amber-400">
          {t("skipReason")} <code>{result.skip_reason}</code>
        </p>
      ) : null}
      {result.rendered_input ? (
        <div>
          <p className="text-xs text-muted-foreground mb-1">{t("renderedInput")}</p>
          <pre className="text-xs bg-muted p-2 rounded overflow-auto">
            {JSON.stringify(result.rendered_input, null, 2)}
          </pre>
        </div>
      ) : null}
    </div>
  );
}
