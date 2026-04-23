"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { Plus, Users, Trash2, Search } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/shared/page-header";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import {
  applyRoleRegistryState,
  buildTeamAttentionGroups,
  getQuickLifecycleLabel,
  getQuickLifecycleTargetStatus,
  getTeamMemberAttentionCategories,
  type TeamAttentionCategory,
  type TeamMember,
} from "@/lib/dashboard/summary";
import type {
  BulkUpdateMembersResponse,
  CreateMemberInput,
  UpdateMemberInput,
} from "@/lib/stores/member-store";
import type { RoleManifest } from "@/lib/stores/role-store";
import type { AgentProfileDraft } from "@/lib/team/agent-profile";
import { useProjectRole } from "@/hooks/use-project-role";
import { getMemberStatusLabel, type MemberStatus } from "@/lib/team/member-status";
import { cn } from "@/lib/utils";

interface TeamProjectOption {
  id: string;
  name: string;
}

interface TeamManagementProps {
  projects: TeamProjectOption[];
  selectedProjectId: string | null;
  members: TeamMember[];
  loading: boolean;
  error: string | null;
  availableRoles: RoleManifest[];
  initialFocus?: "add-member" | TeamAttentionCategory | null;
  bulkUpdatePending?: boolean;
  bulkUpdateResult?: BulkUpdateMembersResponse | null;
  onRetry: () => void;
  onProjectChange: (projectId: string) => void;
  onCreateMember: (input: CreateMemberInput) => Promise<void>;
  onUpdateMember: (memberId: string, input: UpdateMemberInput) => Promise<void>;
  onDeleteMember?: (memberId: string) => Promise<void>;
  onBulkUpdateMembers?: (
    memberIds: string[],
    status: MemberStatus,
  ) => Promise<BulkUpdateMembersResponse>;
  onClearBulkUpdateResult?: () => void;
}

interface MemberFormState {
  name: string;
  type: "human" | "agent";
  role: string;
  status: MemberStatus;
  email: string;
  imPlatform: string;
  imUserId: string;
  skillsInput: string;
  agentProfile: AgentProfileDraft;
}

const EMPTY_AGENT_PROFILE: AgentProfileDraft = {
  roleId: "",
  runtime: "",
  provider: "",
  model: "",
  maxBudgetUsd: "",
  notes: "",
};

function parseCommaList(input: string): string[] {
  return input
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function hasAgentProfileInput(draft: AgentProfileDraft): boolean {
  return Object.values(draft).some((value) => value.trim().length > 0);
}

function buildInitialCreateForm(): MemberFormState {
  return {
    name: "",
    type: "human",
    role: "",
    status: "active",
    email: "",
    imPlatform: "",
    imUserId: "",
    skillsInput: "",
    agentProfile: EMPTY_AGENT_PROFILE,
  };
}

function buildEditForm(member: TeamMember): MemberFormState {
  return {
    name: member.name,
    type: member.type,
    role: member.role,
    status: member.status,
    email: member.email,
    imPlatform: member.imPlatform ?? "",
    imUserId: member.imUserId ?? "",
    skillsInput: member.skills.join(", "),
    agentProfile: {
      roleId: member.agentProfile?.roleId ?? "",
      runtime: member.agentProfile?.runtime ?? "",
      provider: member.agentProfile?.provider ?? "",
      model: member.agentProfile?.model ?? "",
      maxBudgetUsd:
        member.agentProfile?.maxBudgetUsd != null
          ? String(member.agentProfile.maxBudgetUsd)
          : "",
      notes: member.agentProfile?.notes ?? "",
    },
  };
}

function StatusSelect({
  id,
  label,
  value,
  onChange,
}: {
  id: string;
  label: string;
  value: MemberStatus;
  onChange: (value: MemberStatus) => void;
}) {
  const tc = useTranslations("common");
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={(nextValue) => onChange(nextValue as MemberStatus)}>
        <SelectTrigger id={id}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="active">{tc("status.active")}</SelectItem>
          <SelectItem value="inactive">{tc("status.inactive")}</SelectItem>
          <SelectItem value="suspended">{tc("status.suspended")}</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}

function RoleBindingSelect({
  id,
  label,
  value,
  availableRoles,
  onChange,
  className,
  ariaInvalid,
  placeholder,
}: {
  id: string;
  label: string;
  value: string;
  availableRoles: RoleManifest[];
  onChange: (value: string) => void;
  className?: string;
  ariaInvalid?: boolean;
  placeholder?: string;
}) {
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger
          id={id}
          className={className}
          aria-invalid={ariaInvalid}
        >
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__unbound__">{placeholder}</SelectItem>
          {availableRoles.map((role) => (
            <SelectItem key={role.metadata.id} value={role.metadata.id}>
              {role.metadata.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function AgentProfileFields({
  mode,
  availableRoles,
  value,
  onChange,
  highlightedFields = [],
  t,
}: {
  mode: "create" | "edit";
  availableRoles: RoleManifest[];
  value: AgentProfileDraft;
  onChange: (nextValue: AgentProfileDraft) => void;
  highlightedFields?: string[];
  t: ReturnType<typeof useTranslations>;
}) {
  const highlightedSet = new Set(highlightedFields);
  const highlightClass = (field: string) =>
    highlightedSet.has(field)
      ? "border-destructive ring-1 ring-destructive/40"
      : undefined;

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <RoleBindingSelect
        id={mode === "edit" ? "edit-bound-role" : "bound-role"}
        label={mode === "edit" ? t("management.boundRole") : t("management.boundRole")}
        value={value.roleId || "__unbound__"}
        availableRoles={availableRoles}
        className={highlightClass("roleId")}
        ariaInvalid={highlightedSet.has("roleId")}
        placeholder={t("management.unboundRole")}
        onChange={(roleId) =>
          onChange({ ...value, roleId: roleId === "__unbound__" ? "" : roleId })
        }
      />
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-agent-budget" : "agent-budget"}>
          {t("management.agentBudgetUsd")}
        </Label>
        <Input
          id={mode === "edit" ? "edit-agent-budget" : "agent-budget"}
          value={value.maxBudgetUsd}
          className={highlightClass("maxBudgetUsd")}
          aria-invalid={highlightedSet.has("maxBudgetUsd")}
          onChange={(event) =>
            onChange({ ...value, maxBudgetUsd: event.target.value })
          }
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-runtime" : "runtime"}>
          {t("management.runtime")}
        </Label>
        <Input
          id={mode === "edit" ? "edit-runtime" : "runtime"}
          value={value.runtime}
          className={highlightClass("runtime")}
          aria-invalid={highlightedSet.has("runtime")}
          onChange={(event) => onChange({ ...value, runtime: event.target.value })}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-provider" : "provider"}>
          {t("management.provider")}
        </Label>
        <Input
          id={mode === "edit" ? "edit-provider" : "provider"}
          value={value.provider}
          className={highlightClass("provider")}
          aria-invalid={highlightedSet.has("provider")}
          onChange={(event) => onChange({ ...value, provider: event.target.value })}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-model" : "model"}>
          {t("management.model")}
        </Label>
        <Input
          id={mode === "edit" ? "edit-model" : "model"}
          value={value.model}
          className={highlightClass("model")}
          aria-invalid={highlightedSet.has("model")}
          onChange={(event) => onChange({ ...value, model: event.target.value })}
        />
      </div>
      <div className="flex flex-col gap-2 md:col-span-2">
        <Label htmlFor={mode === "edit" ? "edit-agent-notes" : "agent-notes"}>
          {t("management.agentNotes")}
        </Label>
        <Textarea
          id={mode === "edit" ? "edit-agent-notes" : "agent-notes"}
          className="min-h-24 rounded-md border border-input bg-background px-3 py-2 text-sm"
          rows={4}
          value={value.notes}
          onChange={(event) => onChange({ ...value, notes: event.target.value })}
        />
      </div>
    </div>
  );
}

export function TeamManagement({
  projects,
  selectedProjectId,
  members,
  loading,
  error,
  availableRoles,
  initialFocus = null,
  bulkUpdatePending = false,
  bulkUpdateResult = null,
  onRetry,
  onProjectChange,
  onCreateMember,
  onUpdateMember,
  onDeleteMember,
  onBulkUpdateMembers,
  onClearBulkUpdateResult,
}: TeamManagementProps) {
  const t = useTranslations("teams");
  const tc = useTranslations("common");

  // Server-issued permissions for the active project. The hook returns
  // `false` for any action while permissions load — buttons stay disabled
  // rather than rendering optimistically. When no project is selected the
  // hook short-circuits and `can()` always returns false.
  const projectRole = useProjectRole(selectedProjectId ?? null);
  const canCreateMember = projectRole.can("member.create");
  const canUpdateMember = projectRole.can("member.update");
  const canDeleteMember = projectRole.can("member.delete");
  const canBulkUpdate = projectRole.can("member.bulk.update");

  const [showCreateForm, setShowCreateForm] = useState(false);
  const [createForm, setCreateForm] = useState<MemberFormState>(() =>
    buildInitialCreateForm(),
  );
  const [createError, setCreateError] = useState<string | null>(null);
  const [createHighlightedAgentFields, setCreateHighlightedAgentFields] = useState<string[]>([]);
  const [editingMemberId, setEditingMemberId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<MemberFormState | null>(null);
  const [editError, setEditError] = useState<string | null>(null);
  const [highlightedAgentFields, setHighlightedAgentFields] = useState<string[]>([]);
  const [deletingMemberId, setDeletingMemberId] = useState<string | null>(null);

  // Filter state
  const [searchQuery, setSearchQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [attentionFilter, setAttentionFilter] = useState<
    "all" | TeamAttentionCategory
  >("all");
  const [selectedMemberIds, setSelectedMemberIds] = useState<string[]>([]);
  const [pendingMemberIds, setPendingMemberIds] = useState<string[]>([]);
  const [quickActionError, setQuickActionError] = useState<string | null>(null);
  const [localBulkUpdateResult, setLocalBulkUpdateResult] =
    useState<BulkUpdateMembersResponse | null>(bulkUpdateResult);
  const previousProjectIdRef = useRef<string | null>(selectedProjectId);

  const selectedProjectName = useMemo(
    () =>
      projects.find((project) => project.id === selectedProjectId)?.name ?? t("management.title"),
    [projects, selectedProjectId, t],
  );

  const editingMember = useMemo(
    () => members.find((member) => member.id === editingMemberId) ?? null,
    [editingMemberId, members],
  );

  const deletingMember = useMemo(
    () => members.find((member) => member.id === deletingMemberId) ?? null,
    [deletingMemberId, members],
  );
  const governedMembers = useMemo(
    () => applyRoleRegistryState(members, availableRoles),
    [availableRoles, members],
  );
  const attentionGroups = useMemo(
    () => buildTeamAttentionGroups(governedMembers),
    [governedMembers],
  );

  useEffect(() => {
    if (previousProjectIdRef.current === selectedProjectId) {
      return;
    }
    previousProjectIdRef.current = selectedProjectId;
    setAttentionFilter("all");
    setSelectedMemberIds([]);
    setPendingMemberIds([]);
    setQuickActionError(null);
    setLocalBulkUpdateResult(null);
    onClearBulkUpdateResult?.();
  }, [selectedProjectId, onClearBulkUpdateResult]);

  useEffect(() => {
    setLocalBulkUpdateResult(bulkUpdateResult);
  }, [bulkUpdateResult]);

  useEffect(() => {
    if (!selectedProjectId || !initialFocus) {
      return;
    }

    if (initialFocus === "add-member") {
      setShowCreateForm(true);
      return;
    }

    setAttentionFilter(initialFocus);
  }, [initialFocus, selectedProjectId]);

  const filteredMembers = useMemo(() => {
    return governedMembers.filter((member) => {
      if (attentionFilter !== "all") {
        const categories = getTeamMemberAttentionCategories(member);
        if (!categories.includes(attentionFilter)) {
          return false;
        }
      }
      if (searchQuery) {
        const q = searchQuery.toLowerCase();
        const matchesSearch =
          member.name.toLowerCase().includes(q) ||
          member.email.toLowerCase().includes(q) ||
          member.role.toLowerCase().includes(q);
        if (!matchesSearch) return false;
      }
      if (typeFilter !== "all" && member.type !== typeFilter) return false;
      if (statusFilter !== "all" && member.status !== statusFilter) return false;
      return true;
    });
  }, [attentionFilter, governedMembers, searchQuery, typeFilter, statusFilter]);

  const openMemberEditor = (
    member: TeamMember,
    options?: { highlightedFields?: string[] }
  ) => {
    setEditingMemberId(member.id);
    setEditForm(buildEditForm(member));
    setHighlightedAgentFields(options?.highlightedFields ?? []);
    setEditError(null);
  };

  const clearEditState = () => {
    setEditingMemberId(null);
    setEditForm(null);
    setHighlightedAgentFields([]);
  };

  const setupRequiredFields = (member: TeamMember) =>
    (member.readinessMissing ?? []).filter((field) =>
      ["runtime", "provider", "model", "roleId"].includes(field)
    );

  const hasSetupRequired = (member: TeamMember) =>
    member.type === "agent" &&
    setupRequiredFields(member).some((field) =>
      ["runtime", "provider", "model"].includes(field)
    );

  const hasStaleRoleBinding = (member: TeamMember) =>
    member.type === "agent" && member.roleBindingState === "stale";

  const extractRoleFieldError = (error: unknown): string | null => {
    if (!error || typeof error !== "object") {
      return null;
    }
    const body = (error as { body?: { field?: string } }).body;
    return body?.field ?? null;
  };

  const extractErrorMessage = (error: unknown, fallback: string) =>
    error instanceof Error ? error.message : fallback;

  const handleAttentionFilter = (nextFilter: TeamAttentionCategory) => {
    setSearchQuery("");
    setTypeFilter("all");
    setStatusFilter("all");
    setAttentionFilter(nextFilter);
    setSelectedMemberIds([]);
    setQuickActionError(null);
  };

  const handleClearAttentionFilter = () => {
    setAttentionFilter("all");
    setSelectedMemberIds([]);
  };

  const toggleMemberSelection = (memberId: string) => {
    setSelectedMemberIds((current) =>
      current.includes(memberId)
        ? current.filter((id) => id !== memberId)
        : [...current, memberId]
    );
  };

  const toggleAllVisibleMembers = () => {
    const visibleIds = filteredMembers.map((member) => member.id);
    setSelectedMemberIds((current) => {
      const allVisibleSelected =
        visibleIds.length > 0 && visibleIds.every((memberId) => current.includes(memberId));
      if (allVisibleSelected) {
        return current.filter((memberId) => !visibleIds.includes(memberId));
      }
      const next = new Set(current);
      for (const memberId of visibleIds) {
        next.add(memberId);
      }
      return Array.from(next);
    });
  };

  const handleCreateMember = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setCreateError(null);
    setCreateHighlightedAgentFields([]);

    const payload: CreateMemberInput = {
      name: createForm.name.trim(),
      type: createForm.type,
      role: createForm.role.trim(),
      status: createForm.status,
      email: createForm.email.trim(),
      imPlatform: createForm.imPlatform.trim(),
      imUserId: createForm.imUserId.trim(),
      skills: parseCommaList(createForm.skillsInput),
    };

    if (createForm.type === "agent" && hasAgentProfileInput(createForm.agentProfile)) {
      payload.agentProfile = { ...createForm.agentProfile };
    }

    try {
      await onCreateMember(payload);
      setCreateForm(buildInitialCreateForm());
      setShowCreateForm(false);
      setCreateError(null);
      setCreateHighlightedAgentFields([]);
    } catch (error) {
      if (extractRoleFieldError(error) === "agentConfig.roleId") {
        setCreateHighlightedAgentFields(["roleId"]);
      }
      setCreateError(extractErrorMessage(error, t("management.failedToUpdate")));
    }
  };

  const handleEditMember = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!editingMemberId || !editForm) return;
    setEditError(null);
    setHighlightedAgentFields([]);

    const payload: UpdateMemberInput = {
      name: editForm.name.trim(),
      role: editForm.role.trim(),
      status: editForm.status,
      email: editForm.email.trim(),
      imPlatform: editForm.imPlatform.trim(),
      imUserId: editForm.imUserId.trim(),
      skills: parseCommaList(editForm.skillsInput),
    };

    if (editForm.type === "agent") {
      payload.agentProfile = { ...editForm.agentProfile };
    }

    try {
      await onUpdateMember(editingMemberId, payload);
      clearEditState();
    } catch (error) {
      if (extractRoleFieldError(error) === "agentConfig.roleId") {
        setHighlightedAgentFields(["roleId"]);
      }
      setEditError(extractErrorMessage(error, t("management.failedToUpdate")));
    }
  };

  const handleConfirmDelete = async () => {
    if (!deletingMemberId || !onDeleteMember) return;
    await onDeleteMember(deletingMemberId);
    setDeletingMemberId(null);
  };

  const handleBulkStatusUpdate = async (status: MemberStatus) => {
    if (!onBulkUpdateMembers || selectedMemberIds.length === 0) return;
    setQuickActionError(null);
    const result = await onBulkUpdateMembers(selectedMemberIds, status);
    setLocalBulkUpdateResult(result);
    if (result.results.some((item) => item.success)) {
      setSelectedMemberIds([]);
    }
  };

  const handleQuickLifecycleAction = async (member: TeamMember) => {
    const nextStatus = getQuickLifecycleTargetStatus(member);
    if (!nextStatus) return;
    if (pendingMemberIds.includes(member.id)) return;

    setPendingMemberIds((current) => [...current, member.id]);
    setQuickActionError(null);
    try {
      await onUpdateMember(member.id, { status: nextStatus });
    } catch (error) {
      setQuickActionError(
        error instanceof Error ? error.message : t("management.failedToUpdate")
      );
    } finally {
      setPendingMemberIds((current) =>
        current.filter((memberId) => memberId !== member.id)
      );
    }
  };

  if (loading) {
    return (
      <div className="flex flex-col gap-6">
        <div className="space-y-1">
          <span className="sr-only">{t("management.loadingRoster")}</span>
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-4 w-72" />
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-32" />
          </CardHeader>
          <CardContent className="space-y-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="flex items-center gap-4">
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-16" />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("management.rosterUnavailable")}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">{error}</p>
          <div>
            <Button type="button" onClick={onRetry}>
              {t("management.retryLoad")}
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <PageHeader
          title={t("management.title")}
          description={t("management.description", { projectName: selectedProjectName })}
        />

        <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
          <div className="flex flex-col gap-2">
            <Label htmlFor="team-project">{t("management.projectLabel")}</Label>
            <Select
              value={selectedProjectId ?? ""}
              onValueChange={onProjectChange}
            >
              <SelectTrigger id="team-project" className="w-[200px]">
                <SelectValue placeholder={t("management.selectProject")} />
              </SelectTrigger>
              <SelectContent>
                {projects.map((project) => (
                  <SelectItem key={project.id} value={project.id}>
                    {project.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button
            type="button"
            disabled={!selectedProjectId || !canCreateMember}
            onClick={() => setShowCreateForm((value) => !value)}
            title={!canCreateMember ? t("management.requiresAdmin") : undefined}
          >
            <Plus className="mr-1 size-4" />
            {t("management.addMember")}
          </Button>
        </div>
      </div>

      {showCreateForm ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("management.addTeamMember")}</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="grid gap-4 md:grid-cols-2" onSubmit={handleCreateMember}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-name">{t("management.memberName")}</Label>
                <Input
                  id="member-name"
                  value={createForm.name}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, name: event.target.value }))
                  }
                  required
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-type">{t("management.memberType")}</Label>
                <Select
                  value={createForm.type}
                  onValueChange={(value) =>
                    setCreateForm((state) => ({
                      ...state,
                      type: value as MemberFormState["type"],
                    }))
                  }
                >
                  <SelectTrigger id="member-type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="human">{t("management.human")}</SelectItem>
                    <SelectItem value="agent">{t("management.agent")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <StatusSelect
                id="member-status"
                label={t("management.status")}
                value={createForm.status}
                onChange={(status) =>
                  setCreateForm((state) => ({ ...state, status }))
                }
              />
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-role">{t("management.role")}</Label>
                <Input
                  id="member-role"
                  value={createForm.role}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, role: event.target.value }))
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-email">{t("management.email")}</Label>
                <Input
                  id="member-email"
                  type="email"
                  value={createForm.email}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, email: event.target.value }))
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-im-platform">{t("management.imPlatform")}</Label>
                <Input
                  id="member-im-platform"
                  value={createForm.imPlatform}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, imPlatform: event.target.value }))
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-im-user-id">{t("management.imUserId")}</Label>
                <Input
                  id="member-im-user-id"
                  value={createForm.imUserId}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, imUserId: event.target.value }))
                  }
                />
              </div>
              {createForm.type === "agent" ? (
                <>
                  <div className="flex flex-col gap-2 md:col-span-2">
                    <Label htmlFor="member-skills">{t("management.skills")}</Label>
                    <Input
                      id="member-skills"
                      value={createForm.skillsInput}
                      onChange={(event) =>
                        setCreateForm((state) => ({
                          ...state,
                          skillsInput: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="rounded-lg border p-4 md:col-span-2">
                    <h3 className="mb-4 text-sm font-semibold">{t("management.agentProfile")}</h3>
                    <AgentProfileFields
                      mode="create"
                      availableRoles={availableRoles}
                      value={createForm.agentProfile}
                      highlightedFields={createHighlightedAgentFields}
                      onChange={(agentProfile) =>
                        setCreateForm((state) => ({ ...state, agentProfile }))
                      }
                      t={t}
                    />
                    {createError ? (
                      <p className="mt-3 text-sm text-destructive">{createError}</p>
                    ) : null}
                  </div>
                </>
              ) : null}
              <div className="md:col-span-2">
                <Button type="submit" disabled={!createForm.name.trim()}>
                  {t("management.createMember")}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      ) : null}

      {attentionGroups.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("management.needsAttention")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-wrap items-center gap-2">
            {attentionGroups.map((group) => (
              <Button
                key={group.id}
                type="button"
                variant={attentionFilter === group.id ? "default" : "outline"}
                onClick={() => handleAttentionFilter(group.id)}
              >
                {group.label} ({group.count})
              </Button>
            ))}
            {attentionFilter !== "all" ? (
              <Button
                type="button"
                variant="ghost"
                onClick={handleClearAttentionFilter}
              >
                {t("management.clearAttentionFilter")}
              </Button>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {localBulkUpdateResult ? (
        <Card>
          <CardContent className="flex flex-col gap-2 py-4">
            <p className="text-sm font-medium">
              {t("management.bulkUpdateComplete", {
                updated: localBulkUpdateResult.results.filter((item) => item.success).length,
                failed: localBulkUpdateResult.results.filter((item) => !item.success).length,
              })}
            </p>
            {localBulkUpdateResult.results.some((item) => !item.success) ? (
              <ul className="space-y-1 text-sm text-muted-foreground">
                {localBulkUpdateResult.results
                  .filter((item) => !item.success)
                  .map((item) => {
                    const memberName =
                      members.find((member) => member.id === item.memberId)?.name ??
                      item.memberId;
                    return (
                      <li key={item.memberId}>
                        {memberName}: {item.error ?? t("management.failedToUpdate")}
                      </li>
                    );
                  })}
              </ul>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {quickActionError ? (
        <Card>
          <CardContent className="py-4 text-sm text-destructive">
            {quickActionError}
          </CardContent>
        </Card>
      ) : null}

      {members.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
            <Users className="size-10 text-muted-foreground" />
            {selectedProjectId ? (
              <>
                <div className="space-y-1">
                  <p className="font-medium">{t("management.noTeamMembers")}</p>
                  <p className="text-sm text-muted-foreground">
                    {t("management.addFirstCollaborator")}
                  </p>
                </div>
                <Button type="button" onClick={() => setShowCreateForm(true)}>
                  {t("management.addFirstMember")}
                </Button>
              </>
            ) : (
              <div className="space-y-1">
                <p className="font-medium">{t("management.noProjectSelected")}</p>
                <p className="text-sm text-muted-foreground">
                  {t("management.selectProjectToView")}
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex flex-col gap-2">
              <CardTitle>{t("management.unifiedRoster")}</CardTitle>
              {selectedMemberIds.length > 0 && onBulkUpdateMembers && canBulkUpdate ? (
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="outline">{t("management.selectedCount", { count: selectedMemberIds.length })}</Badge>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={bulkUpdatePending}
                    onClick={() => void handleBulkStatusUpdate("active")}
                  >
                    {t("management.markActive")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={bulkUpdatePending}
                    onClick={() => void handleBulkStatusUpdate("inactive")}
                  >
                    {t("management.markInactive")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={bulkUpdatePending}
                    onClick={() => void handleBulkStatusUpdate("suspended")}
                  >
                    {t("management.markSuspended")}
                  </Button>
                </div>
              ) : null}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative">
                <Search className="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
                <Input
                  placeholder={t("management.searchMembers")}
                  className="w-[180px] pl-8"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
              <Select value={typeFilter} onValueChange={setTypeFilter}>
                <SelectTrigger className="w-[120px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("management.allTypes")}</SelectItem>
                  <SelectItem value="human">{t("management.human")}</SelectItem>
                  <SelectItem value="agent">{t("management.agent")}</SelectItem>
                </SelectContent>
              </Select>
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="w-[120px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("management.allStatus")}</SelectItem>
                  <SelectItem value="active">{tc("status.active")}</SelectItem>
                  <SelectItem value="inactive">{tc("status.inactive")}</SelectItem>
                  <SelectItem value="suspended">{tc("status.suspended")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>
                    <input
                      type="checkbox"
                      aria-label={t("management.selectAllVisible")}
                      checked={
                        filteredMembers.length > 0 &&
                        filteredMembers.every((member) =>
                          selectedMemberIds.includes(member.id)
                        )
                      }
                      onChange={toggleAllVisibleMembers}
                    />
                  </TableHead>
                  <TableHead>{t("management.member")}</TableHead>
                  <TableHead>{t("management.type")}</TableHead>
                  <TableHead>{t("management.role")}</TableHead>
                  <TableHead>{t("management.agentConfig")}</TableHead>
                  <TableHead>{t("management.skills")}</TableHead>
                  <TableHead>{t("management.workload")}</TableHead>
                  <TableHead>{t("management.links")}</TableHead>
                  <TableHead className="text-right">{t("management.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredMembers.map((member) => (
                  <TableRow key={member.id}>
                    <TableCell>
                      <input
                        type="checkbox"
                        aria-label={t("management.selectMember", { name: member.name })}
                        checked={selectedMemberIds.includes(member.id)}
                        onChange={() => toggleMemberSelection(member.id)}
                      />
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <Link
                          href={`/project?id=${member.projectId}&member=${member.id}`}
                          className="font-medium hover:underline"
                        >
                          {member.name}
                        </Link>
                        <span className="text-xs text-muted-foreground">
                          {member.email || t("management.noDirectEmail")}
                        </span>
                        {member.imPlatform && member.imUserId ? (
                          <span className="text-xs text-muted-foreground">
                            {member.imPlatform} • {member.imUserId}
                          </span>
                        ) : null}
                        <span className="text-xs text-muted-foreground">
                          {member.lastActivityAt
                            ? t("management.lastActivity", {
                                time: member.lastActivityAt.slice(0, 16).replace("T", " "),
                              })
                            : t("management.noRecentActivity")}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <Badge variant="secondary" className="w-fit">
                          {member.typeLabel}
                        </Badge>
                        <Badge
                          variant={member.status === "active" ? "secondary" : "outline"}
                          className="w-fit"
                        >
                          {member.statusLabel || getMemberStatusLabel(member.status)}
                        </Badge>
                      </div>
                    </TableCell>
                    <TableCell>{member.role}</TableCell>
                    <TableCell>
                      {member.type === "agent" ? (
                        <div className="flex flex-col gap-1 text-xs">
                          {hasSetupRequired(member) || hasStaleRoleBinding(member) ? (
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              className="h-6 w-fit border-destructive/50 px-2 text-destructive"
                              onClick={() =>
                                openMemberEditor(member, {
                                  highlightedFields: hasStaleRoleBinding(member)
                                    ? ["roleId"]
                                    : setupRequiredFields(member),
                                })
                              }
                            >
                              {member.readinessLabel ?? t("management.needsAttention")}
                            </Button>
                          ) : (
                            <Badge
                              variant={
                                member.readinessState === "ready"
                                  ? "secondary"
                                  : "outline"
                              }
                              className="w-fit"
                            >
                              {member.readinessLabel ?? t("management.needsAttention")}
                            </Badge>
                          )}
                          <span className="text-muted-foreground">
                            {member.roleBindingLabel ?? t("management.unboundRole")}
                          </span>
                          {member.agentSummary?.length ? (
                            <span className="text-muted-foreground">
                              {member.agentSummary.join(" • ")}
                            </span>
                          ) : null}
                        </div>
                      ) : (
                        <span className="text-xs text-muted-foreground">
                          {t("management.humanManagedProfile")}
                        </span>
                      )}
                    </TableCell>
                    <TableCell>{member.skills.join(", ") || t("management.noSkillsTagged")}</TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1 text-xs text-muted-foreground">
                        <Link
                          href={`/project?id=${member.projectId}&member=${member.id}`}
                          aria-label={t("management.viewTasks", { name: member.name })}
                          className="hover:text-foreground hover:underline"
                        >
                          {t("management.assigned", { count: member.workload.assignedTasks })}
                        </Link>
                        <Link
                          href={`/project?id=${member.projectId}&member=${member.id}`}
                          aria-label={t("management.viewInProgress", { name: member.name })}
                          className="hover:text-foreground hover:underline"
                        >
                          {t("management.inProgress", { count: member.workload.inProgressTasks })}
                        </Link>
                        <Link
                          href={`/project?id=${member.projectId}&member=${member.id}`}
                          aria-label={t("management.viewInReview", { name: member.name })}
                          className="hover:text-foreground hover:underline"
                        >
                          {t("management.inReview", { count: member.workload.inReviewTasks })}
                        </Link>
                        <Link
                          href={`/agents?member=${member.id}`}
                          aria-label={t("management.viewAgentActivity", { name: member.name })}
                          className={cn(
                            "hover:text-foreground hover:underline",
                            member.type !== "agent" &&
                              "pointer-events-none text-muted-foreground/60 no-underline"
                          )}
                        >
                          {t("management.agentRuns", { count: member.workload.activeAgentRuns })}
                        </Link>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1 text-xs">
                        <Link
                          href={`/project?id=${member.projectId}&member=${member.id}`}
                          className="text-primary hover:underline"
                        >
                          {t("management.projectTasks")}
                        </Link>
                        <Link
                          href={`/agents?member=${member.id}`}
                          className="text-primary hover:underline"
                        >
                          {t("management.agentActivity")}
                        </Link>
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        {getQuickLifecycleTargetStatus(member) ? (
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            disabled={pendingMemberIds.includes(member.id)}
                            aria-label={
                              pendingMemberIds.includes(member.id)
                                ? t("management.updatingAria", { name: member.name })
                                : `${getQuickLifecycleLabel(member)} ${member.name}`
                            }
                            onClick={() => void handleQuickLifecycleAction(member)}
                          >
                            {pendingMemberIds.includes(member.id)
                              ? t("management.updating")
                              : getQuickLifecycleLabel(member)}
                          </Button>
                        ) : null}
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          aria-label={t("management.editAria", { name: member.name })}
                          disabled={!canUpdateMember}
                          title={!canUpdateMember ? t("management.requiresAdmin") : undefined}
                          onClick={() => {
                            openMemberEditor(member);
                          }}
                        >
                          {t("management.edit")}
                        </Button>
                        {onDeleteMember && canDeleteMember && (
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            className="text-muted-foreground hover:text-destructive"
                            onClick={() => setDeletingMemberId(member.id)}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
                {filteredMembers.length === 0 && members.length > 0 && (
                  <TableRow>
                    <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                      {t("management.noMembersMatchFilters")}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {editingMember && editForm ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("management.editMember")}</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="grid gap-4 md:grid-cols-2" onSubmit={handleEditMember}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-name">{t("management.editName")}</Label>
                <Input
                  id="edit-name"
                  value={editForm.name}
                  onChange={(event) =>
                    setEditForm((state) =>
                      state ? { ...state, name: event.target.value } : state,
                    )
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-role">{t("management.editRole")}</Label>
                <Input
                  id="edit-role"
                  value={editForm.role}
                  onChange={(event) =>
                    setEditForm((state) =>
                      state ? { ...state, role: event.target.value } : state,
                    )
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-email">{t("management.editEmail")}</Label>
                <Input
                  id="edit-email"
                  type="email"
                  value={editForm.email}
                  onChange={(event) =>
                    setEditForm((state) =>
                      state ? { ...state, email: event.target.value } : state,
                    )
                  }
                />
              </div>
              <StatusSelect
                id="edit-status"
                label={t("management.editStatus")}
                value={editForm.status}
                onChange={(status) =>
                  setEditForm((state) =>
                    state ? { ...state, status } : state,
                  )
                }
              />
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-im-platform">{t("management.editIMPlatform")}</Label>
                <Input
                  id="edit-im-platform"
                  value={editForm.imPlatform}
                  onChange={(event) =>
                    setEditForm((state) =>
                      state ? { ...state, imPlatform: event.target.value } : state,
                    )
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-im-user-id">{t("management.editIMUserId")}</Label>
                <Input
                  id="edit-im-user-id"
                  value={editForm.imUserId}
                  onChange={(event) =>
                    setEditForm((state) =>
                      state ? { ...state, imUserId: event.target.value } : state,
                    )
                  }
                />
              </div>
              {editForm.type === "agent" ? (
                <>
                  <div className="flex flex-col gap-2 md:col-span-2">
                    <Label htmlFor="edit-skills">{t("management.editSkills")}</Label>
                    <Input
                      id="edit-skills"
                      value={editForm.skillsInput}
                      onChange={(event) =>
                        setEditForm((state) =>
                          state
                            ? { ...state, skillsInput: event.target.value }
                            : state,
                        )
                      }
                    />
                  </div>
                  <div className="rounded-lg border p-4 md:col-span-2">
                    <h3 className="mb-4 text-sm font-semibold">
                      {t("management.agentProfileSettings")}
                    </h3>
                    <AgentProfileFields
                      mode="edit"
                      availableRoles={availableRoles}
                      value={editForm.agentProfile}
                      highlightedFields={highlightedAgentFields}
                      onChange={(agentProfile) =>
                        setEditForm((state) =>
                          state ? { ...state, agentProfile } : state,
                        )
                      }
                      t={t}
                    />
                    {editError ? (
                      <p className="mt-3 text-sm text-destructive">{editError}</p>
                    ) : null}
                  </div>
                </>
              ) : (
                <div className="flex flex-col gap-2 md:col-span-2">
                  <Label htmlFor="edit-skills">{t("management.editSkills")}</Label>
                  <Input
                    id="edit-skills"
                    value={editForm.skillsInput}
                    onChange={(event) =>
                      setEditForm((state) =>
                        state ? { ...state, skillsInput: event.target.value } : state,
                      )
                    }
                  />
                </div>
              )}
              <div className="flex gap-2 md:col-span-2">
                <Button type="submit">{t("management.saveMember")}</Button>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={clearEditState}
                >
                  {tc("action.cancel")}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      ) : null}

      <ConfirmDialog
        open={deletingMemberId !== null}
        title={t("management.deleteMemberTitle")}
        description={t("management.deleteMemberDescription", { name: deletingMember?.name ?? tc("action.delete") })}
        confirmLabel={tc("action.delete")}
        variant="destructive"
        onConfirm={() => void handleConfirmDelete()}
        onCancel={() => setDeletingMemberId(null)}
      />
    </div>
  );
}
