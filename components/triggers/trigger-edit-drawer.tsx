"use client";

/**
 * TriggerEditDrawer — shadcn Sheet-based create/edit form for a single
 * manual trigger. Opens from the TriggerListTable; submits via the
 * employee-trigger-store. The drawer is intentionally flat (no tab
 * container) because the branch per source (`im` vs `schedule`) is the
 * only user decision; everything else is a straight-line form.
 */
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import {
  useEmployeeTriggerStore,
  type CreateTriggerInput,
  type PatchTriggerInput,
} from "@/lib/stores/employee-trigger-store";
import type {
  TriggerSource,
  WorkflowTrigger,
} from "@/lib/stores/workflow-trigger-store";

interface Props {
  open: boolean;
  employeeId: string;
  // When editing an existing row; when undefined the drawer is in "create"
  // mode and the form resets to blank defaults.
  trigger?: WorkflowTrigger | null;
  onClose: () => void;
}

interface FormState {
  workflowId: string;
  source: TriggerSource;
  displayName: string;
  description: string;
  // IM branch
  imPlatform: string;
  imCommand: string;
  imMatchRegex: string;
  imChatAllowlist: string;
  // Schedule branch
  cron: string;
  timezone: string;
  overlapPolicy: string;
  // Shared
  inputMappingJSON: string;
  enabled: boolean;
}

const defaultForm: FormState = {
  workflowId: "",
  source: "im",
  displayName: "",
  description: "",
  imPlatform: "feishu",
  imCommand: "",
  imMatchRegex: "",
  imChatAllowlist: "",
  cron: "",
  timezone: "UTC",
  overlapPolicy: "skip_if_running",
  inputMappingJSON: "{}",
  enabled: true,
};

export function TriggerEditDrawer({ open, employeeId, trigger, onClose }: Props) {
  const createTrigger = useEmployeeTriggerStore((s) => s.createTrigger);
  const patchTrigger = useEmployeeTriggerStore((s) => s.patchTrigger);
  const fetchByEmployee = useEmployeeTriggerStore((s) => s.fetchByEmployee);

  const isEdit = trigger != null;
  const [form, setForm] = useState<FormState>(defaultForm);
  const [jsonErr, setJsonErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    if (trigger) {
      setForm(hydrateForm(trigger));
    } else {
      setForm(defaultForm);
    }
    setJsonErr(null);
  }, [open, trigger]);

  const update = <K extends keyof FormState>(k: K, v: FormState[K]) =>
    setForm((s) => ({ ...s, [k]: v }));

  const configPayload = useMemo((): Record<string, unknown> => {
    if (form.source === "im") {
      const cfg: Record<string, unknown> = { platform: form.imPlatform };
      if (form.imCommand) cfg.command = form.imCommand;
      if (form.imMatchRegex) cfg.match_regex = form.imMatchRegex;
      if (form.imChatAllowlist.trim()) {
        cfg.chat_allowlist = form.imChatAllowlist
          .split(/\r?\n/)
          .map((s) => s.trim())
          .filter(Boolean);
      }
      return cfg;
    }
    return {
      cron: form.cron,
      timezone: form.timezone,
      overlap_policy: form.overlapPolicy,
    };
  }, [form]);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    let mapping: Record<string, unknown>;
    try {
      mapping = JSON.parse(form.inputMappingJSON || "{}");
    } catch (err) {
      setJsonErr(`input_mapping JSON 不合法: ${(err as Error).message}`);
      return;
    }
    setJsonErr(null);
    setSaving(true);
    try {
      if (isEdit && trigger) {
        const patch: PatchTriggerInput = {
          config: configPayload,
          inputMapping: mapping,
          displayName: form.displayName,
          description: form.description,
          enabled: form.enabled,
        };
        const ok = await patchTrigger(trigger.id, patch);
        if (ok) {
          await fetchByEmployee(employeeId);
          onClose();
        }
      } else {
        const input: CreateTriggerInput = {
          workflowId: form.workflowId,
          source: form.source,
          config: configPayload,
          inputMapping: mapping,
          actingEmployeeId: employeeId,
          displayName: form.displayName,
          description: form.description,
        };
        const ok = await createTrigger(input);
        if (ok) onClose();
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{isEdit ? "编辑触发器" : "新建触发器"}</SheetTitle>
          <SheetDescription>
            派发运行的数字员工被默认绑定为当前员工 (`acting_employee_id`)。
          </SheetDescription>
        </SheetHeader>

        <form onSubmit={onSubmit} className="space-y-4 py-4 px-4">
          {!isEdit ? (
            <div className="space-y-2">
              <Label htmlFor="trigger-wf">目标工作流 ID</Label>
              <Input
                id="trigger-wf"
                value={form.workflowId}
                onChange={(e) => update("workflowId", e.target.value)}
                placeholder="UUID"
                required
              />
            </div>
          ) : null}

          <div className="space-y-2">
            <Label htmlFor="trigger-name">显示名称</Label>
            <Input
              id="trigger-name"
              value={form.displayName}
              onChange={(e) => update("displayName", e.target.value)}
              placeholder="例如：PR review 命令"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="trigger-desc">描述 (可选)</Label>
            <Input
              id="trigger-desc"
              value={form.description}
              onChange={(e) => update("description", e.target.value)}
            />
          </div>

          {!isEdit ? (
            <div className="space-y-2">
              <Label>来源</Label>
              <div className="flex gap-3 text-sm">
                <label className="flex items-center gap-1">
                  <input
                    type="radio"
                    name="source"
                    value="im"
                    checked={form.source === "im"}
                    onChange={() => update("source", "im")}
                  />
                  IM 命令
                </label>
                <label className="flex items-center gap-1">
                  <input
                    type="radio"
                    name="source"
                    value="schedule"
                    checked={form.source === "schedule"}
                    onChange={() => update("source", "schedule")}
                  />
                  定时
                </label>
              </div>
            </div>
          ) : null}

          {form.source === "im" ? (
            <div className="space-y-3">
              <div className="space-y-2">
                <Label>平台</Label>
                <Select
                  value={form.imPlatform}
                  onValueChange={(v) => update("imPlatform", v)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="feishu">Feishu</SelectItem>
                    <SelectItem value="slack">Slack</SelectItem>
                    <SelectItem value="dingtalk">DingTalk</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>命令</Label>
                <Input
                  value={form.imCommand}
                  onChange={(e) => update("imCommand", e.target.value)}
                  placeholder="/review"
                />
              </div>
              <div className="space-y-2">
                <Label>match_regex (可选)</Label>
                <Input
                  value={form.imMatchRegex}
                  onChange={(e) => update("imMatchRegex", e.target.value)}
                  placeholder="^/review\\s+https?://.+"
                />
              </div>
              <div className="space-y-2">
                <Label>chat_allowlist (每行一个 chat id)</Label>
                <Textarea
                  value={form.imChatAllowlist}
                  onChange={(e) => update("imChatAllowlist", e.target.value)}
                  rows={3}
                />
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              <div className="space-y-2">
                <Label>cron 表达式</Label>
                <Input
                  value={form.cron}
                  onChange={(e) => update("cron", e.target.value)}
                  placeholder="0 9 * * 1-5"
                />
              </div>
              <div className="space-y-2">
                <Label>时区</Label>
                <Input
                  value={form.timezone}
                  onChange={(e) => update("timezone", e.target.value)}
                  placeholder="UTC"
                />
              </div>
              <div className="space-y-2">
                <Label>重叠策略</Label>
                <Select
                  value={form.overlapPolicy}
                  onValueChange={(v) => update("overlapPolicy", v)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="skip_if_running">skip_if_running</SelectItem>
                    <SelectItem value="allow_parallel">allow_parallel</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          )}

          <div className="space-y-2">
            <Label>input_mapping (JSON)</Label>
            <Textarea
              value={form.inputMappingJSON}
              onChange={(e) => update("inputMappingJSON", e.target.value)}
              rows={6}
              className="font-mono text-xs"
            />
            {jsonErr ? <p className="text-xs text-red-500">{jsonErr}</p> : null}
          </div>

          {isEdit ? (
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="trigger-enabled"
                checked={form.enabled}
                onChange={(e) => update("enabled", e.target.checked)}
              />
              <Label htmlFor="trigger-enabled">启用</Label>
            </div>
          ) : null}

          <SheetFooter className="flex flex-row justify-end gap-2 pt-2">
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? "保存中…" : isEdit ? "保存" : "创建"}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

// Best-effort inverse of configPayload: populates the form fields from an
// existing trigger's stored JSON. Unknown keys are ignored — the JSON is
// round-tripped through the input_mapping textarea so nothing is lost.
function hydrateForm(t: WorkflowTrigger): FormState {
  const cfg = (t.config ?? {}) as Record<string, unknown>;
  const base: FormState = {
    ...defaultForm,
    workflowId: t.workflowId ?? "",
    source: t.source,
    displayName: t.displayName ?? "",
    description: t.description ?? "",
    enabled: t.enabled,
    inputMappingJSON: JSON.stringify(t.inputMapping ?? {}, null, 2),
  };
  if (t.source === "im") {
    base.imPlatform = (cfg.platform as string) ?? "feishu";
    base.imCommand = (cfg.command as string) ?? "";
    base.imMatchRegex = (cfg.match_regex as string) ?? "";
    base.imChatAllowlist = Array.isArray(cfg.chat_allowlist)
      ? (cfg.chat_allowlist as string[]).join("\n")
      : "";
  } else if (t.source === "schedule") {
    base.cron = (cfg.cron as string) ?? "";
    base.timezone = (cfg.timezone as string) ?? "UTC";
    base.overlapPolicy = (cfg.overlap_policy as string) ?? "skip_if_running";
  }
  return base;
}
