"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAgentStore, type Agent } from "@/lib/stores/agent-store";
import { useReviewStore, type ReviewDTO } from "@/lib/stores/review-store";
import { useMemberStore } from "@/lib/stores/member-store";
import { useSavedViewStore, type SavedView } from "@/lib/stores/saved-view-store";
import type { LiveArtifactKind } from "./types";

// ---------------------------------------------------------------------------
// Shared spec type emitted by every insertion dialog
// ---------------------------------------------------------------------------

export interface LiveArtifactInsertSpec {
  live_kind: LiveArtifactKind;
  target_ref: unknown;
  view_opts: unknown;
}

export type LiveArtifactOnInsert = (spec: LiveArtifactInsertSpec) => void;

const RUNTIME_OPTIONS = [
  "claude_code",
  "codex",
  "cursor",
  "opencode",
  "gemini",
  "qoder",
  "iflow",
] as const;

const PROVIDER_OPTIONS = [
  "anthropic",
  "openai",
  "google",
  "cursor",
  "qwen",
  "iflow",
] as const;

const GROUP_BY_OPTIONS: Array<"" | "runtime" | "provider" | "member"> = [
  "",
  "runtime",
  "provider",
  "member",
];

function shortId(id: string | null | undefined): string {
  if (!id) return "";
  return id.length > 8 ? id.slice(0, 8) : id;
}

function formatDate(value: string | null | undefined): string {
  if (!value) return "";
  try {
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return value;
    return d.toISOString().replace("T", " ").slice(0, 16);
  } catch {
    return value;
  }
}

// ---------------------------------------------------------------------------
// AgentRunPickerDialog
// ---------------------------------------------------------------------------

export interface AgentRunPickerDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId?: string | null;
  onInsert: LiveArtifactOnInsert;
  /** Override agent list for tests / storybook. */
  agentsOverride?: Agent[];
}

export function AgentRunPickerDialog(props: AgentRunPickerDialogProps) {
  // Mounting the form body only when `open` is true ensures the form state is
  // freshly initialized each time the user opens the dialog. This avoids
  // resetting state synchronously inside a useEffect, which React 19
  // discourages (react-hooks/set-state-in-effect).
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        {props.open ? <AgentRunPickerBody {...props} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function AgentRunPickerBody({
  onOpenChange,
  onInsert,
  agentsOverride,
}: AgentRunPickerDialogProps) {
  const t = useTranslations("docs");
  const storeAgents = useAgentStore((state) => state.agents);
  const fetchAgents = useAgentStore((state) => state.fetchAgents);
  const [search, setSearch] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  useEffect(() => {
    if (agentsOverride) return;
    if (storeAgents.length === 0) {
      void fetchAgents();
    }
  }, [storeAgents.length, fetchAgents, agentsOverride]);

  const available = agentsOverride ?? storeAgents;

  const filteredRuns = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return available.slice(0, 20);
    return available
      .filter((agent) => {
        const haystack = [agent.id, agent.taskTitle, agent.taskId, agent.status, agent.runtime]
          .join(" ")
          .toLowerCase();
        return haystack.includes(term);
      })
      .slice(0, 20);
  }, [available, search]);

  const confirmDisabled = !selectedId;

  const handleConfirm = () => {
    if (!selectedId) return;
    onInsert({
      live_kind: "agent_run",
      target_ref: { kind: "agent_run", id: selectedId },
      view_opts: { show_log_lines: 10, show_steps: true },
    });
    onOpenChange(false);
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("liveArtifact.dialogs.agentRun.title")}</DialogTitle>
        <DialogDescription>
          {t("liveArtifact.dialogs.agentRun.desc")}
        </DialogDescription>
      </DialogHeader>
      <div className="grid gap-3">
        <Input
          aria-label={t("liveArtifact.dialogs.agentRun.searchAriaLabel")}
          placeholder={t("liveArtifact.dialogs.agentRun.searchPlaceholder")}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <div
          role="listbox"
          aria-label={t("liveArtifact.dialogs.agentRun.listAriaLabel")}
          className="max-h-72 space-y-2 overflow-y-auto"
        >
          {filteredRuns.map((agent) => {
            const isSelected = agent.id === selectedId;
            return (
              <button
                key={agent.id}
                type="button"
                role="option"
                aria-selected={isSelected}
                onClick={() => setSelectedId(agent.id)}
                className={
                  "flex w-full flex-col gap-1 rounded-lg border px-3 py-2 text-left text-sm hover:bg-accent/40 " +
                  (isSelected ? "border-primary bg-accent/30" : "border-border/60")
                }
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="font-mono text-xs">{shortId(agent.id)}</span>
                  <span className="rounded-full border px-2 py-0.5 text-xs">
                    {agent.status}
                  </span>
                </div>
                <div className="text-xs text-muted-foreground">
                  {agent.taskTitle || agent.taskId} · {t("liveArtifact.dialogs.agentRun.started")} {formatDate(agent.startedAt)} · {t("liveArtifact.dialogs.agentRun.cost", { amount: Number(agent.cost ?? 0).toFixed(4) })}
                </div>
              </button>
            );
          })}
          {filteredRuns.length === 0 ? (
            <div className="rounded-lg border border-dashed px-3 py-4 text-center text-sm text-muted-foreground">
              {t("liveArtifact.dialogs.agentRun.empty")}
            </div>
          ) : null}
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" onClick={() => onOpenChange(false)}>
          {t("liveArtifact.dialogs.cancel")}
        </Button>
        <Button onClick={handleConfirm} disabled={confirmDisabled}>
          {t("liveArtifact.dialogs.insert")}
        </Button>
      </DialogFooter>
    </>
  );
}

// ---------------------------------------------------------------------------
// CostSummaryFilterDialog
// ---------------------------------------------------------------------------

export interface CostSummaryFilterDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId?: string | null;
  onInsert: LiveArtifactOnInsert;
}

export function CostSummaryFilterDialog(props: CostSummaryFilterDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        {props.open ? <CostSummaryFilterBody {...props} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function CostSummaryFilterBody({
  onOpenChange,
  projectId,
  onInsert,
}: CostSummaryFilterDialogProps) {
  const t = useTranslations("docs");
  const membersByProject = useMemberStore((state) => state.membersByProject);
  const fetchMembers = useMemberStore((state) => state.fetchMembers);

  const [rangeStart, setRangeStart] = useState("");
  const [rangeEnd, setRangeEnd] = useState("");
  const [runtime, setRuntime] = useState("");
  const [provider, setProvider] = useState("");
  const [memberId, setMemberId] = useState("");
  const [groupBy, setGroupBy] = useState<"" | "runtime" | "provider" | "member">("");
  const [topN, setTopN] = useState<number>(5);

  useEffect(() => {
    if (!projectId) return;
    if (!membersByProject[projectId]) {
      void fetchMembers(projectId);
    }
  }, [projectId, membersByProject, fetchMembers]);

  const members = projectId ? membersByProject[projectId] ?? [] : [];
  const confirmDisabled = !rangeStart || !rangeEnd;

  const handleConfirm = () => {
    if (confirmDisabled) return;
    const filter: Record<string, unknown> = {
      range_start: rangeStart,
      range_end: rangeEnd,
    };
    if (runtime) filter.runtime = runtime;
    if (provider) filter.provider = provider;
    if (memberId) filter.member_id = memberId;

    const viewOpts: Record<string, unknown> = { top_n: topN };
    if (groupBy) viewOpts.group_by = groupBy;

    onInsert({
      live_kind: "cost_summary",
      target_ref: { kind: "cost_summary", filter },
      view_opts: viewOpts,
    });
    onOpenChange(false);
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("liveArtifact.dialogs.costSummary.title")}</DialogTitle>
        <DialogDescription>
          {t("liveArtifact.dialogs.costSummary.desc")}
        </DialogDescription>
      </DialogHeader>
      <div className="grid gap-3">
        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-1">
            <Label htmlFor="cost-range-start">{t("liveArtifact.dialogs.costSummary.startDate")}</Label>
            <Input
              id="cost-range-start"
              type="date"
              value={rangeStart}
              onChange={(event) => setRangeStart(event.target.value)}
            />
          </div>
          <div className="grid gap-1">
            <Label htmlFor="cost-range-end">{t("liveArtifact.dialogs.costSummary.endDate")}</Label>
            <Input
              id="cost-range-end"
              type="date"
              value={rangeEnd}
              onChange={(event) => setRangeEnd(event.target.value)}
            />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-1">
            <Label htmlFor="cost-runtime">{t("liveArtifact.dialogs.costSummary.runtime")}</Label>
            <select
              id="cost-runtime"
              className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
              value={runtime}
              onChange={(event) => setRuntime(event.target.value)}
            >
              <option value="">{t("liveArtifact.dialogs.costSummary.anyRuntime")}</option>
              {RUNTIME_OPTIONS.map((opt) => (
                <option key={opt} value={opt}>
                  {opt}
                </option>
              ))}
            </select>
          </div>
          <div className="grid gap-1">
            <Label htmlFor="cost-provider">{t("liveArtifact.dialogs.costSummary.provider")}</Label>
            <select
              id="cost-provider"
              className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
              value={provider}
              onChange={(event) => setProvider(event.target.value)}
            >
              <option value="">{t("liveArtifact.dialogs.costSummary.anyProvider")}</option>
              {PROVIDER_OPTIONS.map((opt) => (
                <option key={opt} value={opt}>
                  {opt}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-1">
            <Label htmlFor="cost-member">{t("liveArtifact.dialogs.costSummary.member")}</Label>
            <select
              id="cost-member"
              className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
              value={memberId}
              onChange={(event) => setMemberId(event.target.value)}
            >
              <option value="">{t("liveArtifact.dialogs.costSummary.anyMember")}</option>
              {members.map((member) => (
                <option key={member.id} value={member.id}>
                  {member.name ?? member.id}
                </option>
              ))}
            </select>
          </div>
          <div className="grid gap-1">
            <Label htmlFor="cost-group-by">{t("liveArtifact.dialogs.costSummary.groupBy")}</Label>
            <select
              id="cost-group-by"
              className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
              value={groupBy}
              onChange={(event) =>
                setGroupBy(event.target.value as "" | "runtime" | "provider" | "member")
              }
            >
              {GROUP_BY_OPTIONS.map((opt) => (
                <option key={opt || "none"} value={opt}>
                  {opt || t("liveArtifact.dialogs.costSummary.noGrouping")}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="grid gap-1">
          <Label htmlFor="cost-top-n">{t("liveArtifact.dialogs.costSummary.topN")}</Label>
          <Input
            id="cost-top-n"
            type="number"
            min={1}
            max={50}
            value={topN}
            onChange={(event) => {
              const parsed = Number(event.target.value);
              setTopN(Number.isFinite(parsed) && parsed > 0 ? parsed : 5);
            }}
          />
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" onClick={() => onOpenChange(false)}>
          {t("liveArtifact.dialogs.cancel")}
        </Button>
        <Button onClick={handleConfirm} disabled={confirmDisabled}>
          {t("liveArtifact.dialogs.insert")}
        </Button>
      </DialogFooter>
    </>
  );
}

// ---------------------------------------------------------------------------
// ReviewPickerDialog
// ---------------------------------------------------------------------------

export interface ReviewPickerDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId?: string | null;
  onInsert: LiveArtifactOnInsert;
  /** Override review list for tests. */
  reviewsOverride?: ReviewDTO[];
}

export function ReviewPickerDialog(props: ReviewPickerDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        {props.open ? <ReviewPickerBody {...props} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function ReviewPickerBody({
  onOpenChange,
  onInsert,
  reviewsOverride,
}: ReviewPickerDialogProps) {
  const t = useTranslations("docs");
  const allReviews = useReviewStore((state) => state.allReviews);
  const fetchAllReviews = useReviewStore((state) => state.fetchAllReviews);
  const [search, setSearch] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  useEffect(() => {
    if (reviewsOverride) return;
    if (allReviews.length === 0) {
      void fetchAllReviews();
    }
  }, [allReviews.length, fetchAllReviews, reviewsOverride]);

  const available = reviewsOverride ?? allReviews;

  const filteredReviews = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return available.slice(0, 50);
    return available
      .filter((review) => {
        const haystack = [review.id, review.taskId, review.status, review.riskLevel, review.summary]
          .join(" ")
          .toLowerCase();
        return haystack.includes(term);
      })
      .slice(0, 50);
  }, [available, search]);

  const confirmDisabled = !selectedId;

  const handleConfirm = () => {
    if (!selectedId) return;
    onInsert({
      live_kind: "review",
      target_ref: { kind: "review", id: selectedId },
      view_opts: { show_findings_preview: true },
    });
    onOpenChange(false);
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("liveArtifact.dialogs.review.title")}</DialogTitle>
        <DialogDescription>
          {t("liveArtifact.dialogs.review.desc")}
        </DialogDescription>
      </DialogHeader>
      <div className="grid gap-3">
        <Input
          aria-label={t("liveArtifact.dialogs.review.searchAriaLabel")}
          placeholder={t("liveArtifact.dialogs.review.searchPlaceholder")}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <div
          role="listbox"
          aria-label={t("liveArtifact.dialogs.review.listAriaLabel")}
          className="max-h-72 space-y-2 overflow-y-auto"
        >
          {filteredReviews.map((review) => {
            const isSelected = review.id === selectedId;
            return (
              <button
                key={review.id}
                type="button"
                role="option"
                aria-selected={isSelected}
                onClick={() => setSelectedId(review.id)}
                className={
                  "flex w-full flex-col gap-1 rounded-lg border px-3 py-2 text-left text-sm hover:bg-accent/40 " +
                  (isSelected ? "border-primary bg-accent/30" : "border-border/60")
                }
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="font-mono text-xs">{shortId(review.id)}</span>
                  <span className="rounded-full border px-2 py-0.5 text-xs">
                    {review.status}
                  </span>
                </div>
                <div className="text-xs text-muted-foreground">
                  {t("liveArtifact.dialogs.review.risk", { level: review.riskLevel || t("liveArtifact.dialogs.review.unknown") })} · task {shortId(review.taskId)}
                  {review.summary ? ` · ${review.summary.slice(0, 60)}` : ""}
                </div>
              </button>
            );
          })}
          {filteredReviews.length === 0 ? (
            <div className="rounded-lg border border-dashed px-3 py-4 text-center text-sm text-muted-foreground">
              {t("liveArtifact.dialogs.review.empty")}
            </div>
          ) : null}
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" onClick={() => onOpenChange(false)}>
          {t("liveArtifact.dialogs.cancel")}
        </Button>
        <Button onClick={handleConfirm} disabled={confirmDisabled}>
          {t("liveArtifact.dialogs.insert")}
        </Button>
      </DialogFooter>
    </>
  );
}

// ---------------------------------------------------------------------------
// TaskGroupFilterDialog
// ---------------------------------------------------------------------------

export interface TaskGroupFilterDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId?: string | null;
  onInsert: LiveArtifactOnInsert;
  /** Override saved views for tests. */
  savedViewsOverride?: SavedView[];
}

export function TaskGroupFilterDialog(props: TaskGroupFilterDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        {props.open ? <TaskGroupFilterBody {...props} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function TaskGroupFilterBody({
  onOpenChange,
  projectId,
  onInsert,
  savedViewsOverride,
}: TaskGroupFilterDialogProps) {
  const t = useTranslations("docs");
  const viewsByProject = useSavedViewStore((state) => state.viewsByProject);
  const fetchViews = useSavedViewStore((state) => state.fetchViews);

  const [mode, setMode] = useState<"saved_view" | "inline">("saved_view");
  const [savedViewId, setSavedViewId] = useState("");
  const [status, setStatus] = useState("");
  const [assignee, setAssignee] = useState("");
  const [tag, setTag] = useState("");
  const [sprintId, setSprintId] = useState("");
  const [milestoneId, setMilestoneId] = useState("");
  const [pageSize, setPageSize] = useState(50);

  useEffect(() => {
    if (savedViewsOverride) return;
    if (!projectId) return;
    if (!viewsByProject[projectId]) {
      void fetchViews(projectId);
    }
  }, [projectId, viewsByProject, fetchViews, savedViewsOverride]);

  const savedViews = savedViewsOverride ?? (projectId ? viewsByProject[projectId] ?? [] : []);

  const confirmDisabled =
    mode === "saved_view"
      ? !savedViewId
      : !status && !assignee && !tag && !sprintId && !milestoneId;

  const handleConfirm = () => {
    if (confirmDisabled) return;
    let filter: Record<string, unknown>;
    if (mode === "saved_view") {
      filter = { saved_view_id: savedViewId };
    } else {
      const inline: Record<string, unknown> = {};
      if (status) inline.status = status;
      if (assignee) inline.assignee = assignee;
      if (tag) inline.tag = tag;
      if (sprintId) inline.sprint_id = sprintId;
      if (milestoneId) inline.milestone_id = milestoneId;
      filter = inline;
    }
    onInsert({
      live_kind: "task_group",
      target_ref: { kind: "task_group", filter },
      view_opts: { page_size: pageSize, sort: "updated_at_desc" },
    });
    onOpenChange(false);
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("liveArtifact.dialogs.taskGroup.title")}</DialogTitle>
        <DialogDescription>
          {t("liveArtifact.dialogs.taskGroup.desc")}
        </DialogDescription>
      </DialogHeader>
      <div className="grid gap-3">
        <div
          role="tablist"
          aria-label={t("liveArtifact.dialogs.taskGroup.filterModeAriaLabel")}
          className="grid grid-cols-2 gap-1 rounded-md border border-border/60 p-1"
        >
          <button
            type="button"
            role="tab"
            aria-selected={mode === "saved_view"}
            onClick={() => setMode("saved_view")}
            className={
              "rounded-md px-3 py-1 text-sm " +
              (mode === "saved_view"
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground")
            }
          >
            {t("liveArtifact.dialogs.taskGroup.savedViewTab")}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === "inline"}
            onClick={() => setMode("inline")}
            className={
              "rounded-md px-3 py-1 text-sm " +
              (mode === "inline"
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground")
            }
          >
            {t("liveArtifact.dialogs.taskGroup.inlineFilterTab")}
          </button>
        </div>

        {mode === "saved_view" ? (
          <div className="grid gap-1">
            <Label htmlFor="task-group-saved-view">{t("liveArtifact.dialogs.taskGroup.savedViewLabel")}</Label>
            <select
              id="task-group-saved-view"
              className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
              value={savedViewId}
              onChange={(event) => setSavedViewId(event.target.value)}
            >
              <option value="">{t("liveArtifact.dialogs.taskGroup.selectSavedView")}</option>
              {savedViews.map((view) => (
                <option key={view.id} value={view.id}>
                  {view.name}
                </option>
              ))}
            </select>
            {savedViews.length === 0 ? (
              <div className="rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground">
                {t("liveArtifact.dialogs.taskGroup.noSavedViews")}
              </div>
            ) : null}
          </div>
        ) : (
          <div className="grid gap-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-1">
                <Label htmlFor="task-group-status">{t("liveArtifact.dialogs.taskGroup.status")}</Label>
                <Input
                  id="task-group-status"
                  placeholder={t("liveArtifact.dialogs.taskGroup.statusPlaceholder")}
                  value={status}
                  onChange={(event) => setStatus(event.target.value)}
                />
              </div>
              <div className="grid gap-1">
                <Label htmlFor="task-group-assignee">{t("liveArtifact.dialogs.taskGroup.assignee")}</Label>
                <Input
                  id="task-group-assignee"
                  placeholder={t("liveArtifact.dialogs.taskGroup.assigneePlaceholder")}
                  value={assignee}
                  onChange={(event) => setAssignee(event.target.value)}
                />
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div className="grid gap-1">
                <Label htmlFor="task-group-tag">{t("liveArtifact.dialogs.taskGroup.tag")}</Label>
                <Input
                  id="task-group-tag"
                  value={tag}
                  onChange={(event) => setTag(event.target.value)}
                />
              </div>
              <div className="grid gap-1">
                <Label htmlFor="task-group-sprint">{t("liveArtifact.dialogs.taskGroup.sprint")}</Label>
                <Input
                  id="task-group-sprint"
                  value={sprintId}
                  onChange={(event) => setSprintId(event.target.value)}
                />
              </div>
              <div className="grid gap-1">
                <Label htmlFor="task-group-milestone">{t("liveArtifact.dialogs.taskGroup.milestone")}</Label>
                <Input
                  id="task-group-milestone"
                  value={milestoneId}
                  onChange={(event) => setMilestoneId(event.target.value)}
                />
              </div>
            </div>
          </div>
        )}

        <div className="grid gap-1">
          <Label htmlFor="task-group-page-size">{t("liveArtifact.dialogs.taskGroup.rowsPerPage")}</Label>
          <Input
            id="task-group-page-size"
            type="number"
            min={5}
            max={500}
            value={pageSize}
            onChange={(event) => {
              const parsed = Number(event.target.value);
              setPageSize(Number.isFinite(parsed) && parsed > 0 ? parsed : 50);
            }}
          />
        </div>
      </div>
      <DialogFooter>
        <Button variant="ghost" onClick={() => onOpenChange(false)}>
          {t("liveArtifact.dialogs.cancel")}
        </Button>
        <Button onClick={handleConfirm} disabled={confirmDisabled}>
          {t("liveArtifact.dialogs.insert")}
        </Button>
      </DialogFooter>
    </>
  );
}
