"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { Plus, Users, Trash2, Search } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import type { TeamMember } from "@/lib/dashboard/summary";
import type {
  CreateMemberInput,
  UpdateMemberInput,
} from "@/lib/stores/member-store";
import type { RoleManifest } from "@/lib/stores/role-store";
import type { AgentProfileDraft } from "@/lib/team/agent-profile";
import { getMemberStatusLabel, type MemberStatus } from "@/lib/team/member-status";

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
  onRetry: () => void;
  onProjectChange: (projectId: string) => void;
  onCreateMember: (input: CreateMemberInput) => Promise<void>;
  onUpdateMember: (memberId: string, input: UpdateMemberInput) => Promise<void>;
  onDeleteMember?: (memberId: string) => Promise<void>;
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

function formatActivityTimestamp(value: string | null): string {
  if (!value) return "No recent activity";
  return `Last activity ${value.slice(0, 16).replace("T", " ")} UTC`;
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
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={(nextValue) => onChange(nextValue as MemberStatus)}>
        <SelectTrigger id={id}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="active">Active</SelectItem>
          <SelectItem value="inactive">Inactive</SelectItem>
          <SelectItem value="suspended">Suspended</SelectItem>
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
}: {
  id: string;
  label: string;
  value: string;
  availableRoles: RoleManifest[];
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id}>
          <SelectValue placeholder="Unbound role" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__unbound__">Unbound role</SelectItem>
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
}: {
  mode: "create" | "edit";
  availableRoles: RoleManifest[];
  value: AgentProfileDraft;
  onChange: (nextValue: AgentProfileDraft) => void;
}) {
  const prefix = mode === "edit" ? "Edit " : "";

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <RoleBindingSelect
        id={mode === "edit" ? "edit-bound-role" : "bound-role"}
        label={`${prefix}Bound Role`}
        value={value.roleId || "__unbound__"}
        availableRoles={availableRoles}
        onChange={(roleId) =>
          onChange({ ...value, roleId: roleId === "__unbound__" ? "" : roleId })
        }
      />
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-agent-budget" : "agent-budget"}>
          {prefix}Agent Budget USD
        </Label>
        <Input
          id={mode === "edit" ? "edit-agent-budget" : "agent-budget"}
          value={value.maxBudgetUsd}
          onChange={(event) =>
            onChange({ ...value, maxBudgetUsd: event.target.value })
          }
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-runtime" : "runtime"}>
          {prefix}Runtime
        </Label>
        <Input
          id={mode === "edit" ? "edit-runtime" : "runtime"}
          value={value.runtime}
          onChange={(event) => onChange({ ...value, runtime: event.target.value })}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-provider" : "provider"}>
          {prefix}Provider
        </Label>
        <Input
          id={mode === "edit" ? "edit-provider" : "provider"}
          value={value.provider}
          onChange={(event) => onChange({ ...value, provider: event.target.value })}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-model" : "model"}>
          {prefix}Model
        </Label>
        <Input
          id={mode === "edit" ? "edit-model" : "model"}
          value={value.model}
          onChange={(event) => onChange({ ...value, model: event.target.value })}
        />
      </div>
      <div className="flex flex-col gap-2 md:col-span-2">
        <Label htmlFor={mode === "edit" ? "edit-agent-notes" : "agent-notes"}>
          {prefix}Agent Notes
        </Label>
        <textarea
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
  onRetry,
  onProjectChange,
  onCreateMember,
  onUpdateMember,
  onDeleteMember,
}: TeamManagementProps) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [createForm, setCreateForm] = useState<MemberFormState>(() =>
    buildInitialCreateForm(),
  );
  const [editingMemberId, setEditingMemberId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<MemberFormState | null>(null);
  const [deletingMemberId, setDeletingMemberId] = useState<string | null>(null);

  // Filter state
  const [searchQuery, setSearchQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");

  const selectedProjectName = useMemo(
    () =>
      projects.find((project) => project.id === selectedProjectId)?.name ?? "Team",
    [projects, selectedProjectId],
  );

  const editingMember = useMemo(
    () => members.find((member) => member.id === editingMemberId) ?? null,
    [editingMemberId, members],
  );

  const deletingMember = useMemo(
    () => members.find((member) => member.id === deletingMemberId) ?? null,
    [deletingMemberId, members],
  );

  const filteredMembers = useMemo(() => {
    return members.filter((member) => {
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
  }, [members, searchQuery, typeFilter, statusFilter]);

  const handleCreateMember = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

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

    await onCreateMember(payload);
    setCreateForm(buildInitialCreateForm());
    setShowCreateForm(false);
  };

  const handleEditMember = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!editingMemberId || !editForm) return;

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

    await onUpdateMember(editingMemberId, payload);
    setEditingMemberId(null);
    setEditForm(null);
  };

  const handleConfirmDelete = async () => {
    if (!deletingMemberId || !onDeleteMember) return;
    await onDeleteMember(deletingMemberId);
    setDeletingMemberId(null);
  };

  if (loading) {
    return (
      <div className="flex flex-col gap-6">
        <div className="space-y-1">
          <span className="sr-only">Loading team roster...</span>
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
          <CardTitle>Team roster unavailable</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">{error}</p>
          <div>
            <Button type="button" onClick={onRetry}>
              Retry Team Load
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold">Team Management</h1>
          <p className="text-sm text-muted-foreground">
            Keep human and agent collaborators aligned for {selectedProjectName}.
          </p>
        </div>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
          <div className="flex flex-col gap-2">
            <Label htmlFor="team-project">Project</Label>
            <Select
              value={selectedProjectId ?? ""}
              onValueChange={onProjectChange}
            >
              <SelectTrigger id="team-project" className="w-[200px]">
                <SelectValue placeholder="Select project" />
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
          <Button type="button" onClick={() => setShowCreateForm((value) => !value)}>
            <Plus className="mr-1 size-4" />
            Add Member
          </Button>
        </div>
      </div>

      {showCreateForm ? (
        <Card>
          <CardHeader>
            <CardTitle>Add Team Member</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="grid gap-4 md:grid-cols-2" onSubmit={handleCreateMember}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-name">Member Name</Label>
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
                <Label htmlFor="member-type">Member Type</Label>
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
                    <SelectItem value="human">Human</SelectItem>
                    <SelectItem value="agent">Agent</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <StatusSelect
                id="member-status"
                label="Status"
                value={createForm.status}
                onChange={(status) =>
                  setCreateForm((state) => ({ ...state, status }))
                }
              />
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-role">Role</Label>
                <Input
                  id="member-role"
                  value={createForm.role}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, role: event.target.value }))
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-email">Email</Label>
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
                <Label htmlFor="member-im-platform">IM Platform</Label>
                <Input
                  id="member-im-platform"
                  value={createForm.imPlatform}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, imPlatform: event.target.value }))
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-im-user-id">IM User ID</Label>
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
                    <Label htmlFor="member-skills">Skills</Label>
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
                    <h3 className="mb-4 text-sm font-semibold">Agent Profile</h3>
                    <AgentProfileFields
                      mode="create"
                      availableRoles={availableRoles}
                      value={createForm.agentProfile}
                      onChange={(agentProfile) =>
                        setCreateForm((state) => ({ ...state, agentProfile }))
                      }
                    />
                  </div>
                </>
              ) : null}
              <div className="md:col-span-2">
                <Button type="submit" disabled={!createForm.name.trim()}>
                  Create Member
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      ) : null}

      {members.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
            <Users className="size-10 text-muted-foreground" />
            <div className="space-y-1">
              <p className="font-medium">No team members yet.</p>
              <p className="text-sm text-muted-foreground">
                Add the first human or agent collaborator for this project.
              </p>
            </div>
            <Button type="button" onClick={() => setShowCreateForm(true)}>
              Add the first member
            </Button>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle>Unified Roster</CardTitle>
            <div className="flex flex-wrap items-center gap-2">
              <div className="relative">
                <Search className="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
                <Input
                  placeholder="Search members..."
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
                  <SelectItem value="all">All Types</SelectItem>
                  <SelectItem value="human">Human</SelectItem>
                  <SelectItem value="agent">Agent</SelectItem>
                </SelectContent>
              </Select>
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="w-[120px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Status</SelectItem>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="inactive">Inactive</SelectItem>
                  <SelectItem value="suspended">Suspended</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Member</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Agent Configuration</TableHead>
                  <TableHead>Skills</TableHead>
                  <TableHead>Workload</TableHead>
                  <TableHead>Links</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredMembers.map((member) => (
                  <TableRow key={member.id}>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <span className="font-medium">{member.name}</span>
                        <span className="text-xs text-muted-foreground">
                          {member.email || "No direct email"}
                        </span>
                        {member.imPlatform && member.imUserId ? (
                          <span className="text-xs text-muted-foreground">
                            {member.imPlatform} • {member.imUserId}
                          </span>
                        ) : null}
                        <span className="text-xs text-muted-foreground">
                          {formatActivityTimestamp(member.lastActivityAt)}
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
                          <Badge
                            variant={
                              member.readinessState === "ready"
                                ? "secondary"
                                : "outline"
                            }
                            className="w-fit"
                          >
                            {member.readinessLabel ?? "Needs attention"}
                          </Badge>
                          <span className="text-muted-foreground">
                            Bound role: {member.roleBindingLabel ?? "Unbound role"}
                          </span>
                          {member.agentSummary?.length ? (
                            <span className="text-muted-foreground">
                              {member.agentSummary.join(" • ")}
                            </span>
                          ) : null}
                        </div>
                      ) : (
                        <span className="text-xs text-muted-foreground">
                          Human-managed profile
                        </span>
                      )}
                    </TableCell>
                    <TableCell>{member.skills.join(", ") || "No skills tagged"}</TableCell>
                    <TableCell>
                      <div className="text-xs text-muted-foreground">
                        <div>Assigned: {member.workload.assignedTasks}</div>
                        <div>In progress: {member.workload.inProgressTasks}</div>
                        <div>In review: {member.workload.inReviewTasks}</div>
                        <div>Agent runs: {member.workload.activeAgentRuns}</div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1 text-xs">
                        <Link
                          href={`/project?id=${member.projectId}&member=${member.id}`}
                          className="text-primary hover:underline"
                        >
                          Project tasks
                        </Link>
                        <Link
                          href={`/agents?member=${member.id}`}
                          className="text-primary hover:underline"
                        >
                          Agent activity
                        </Link>
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          aria-label={`Edit ${member.name}`}
                          onClick={() => {
                            setEditingMemberId(member.id);
                            setEditForm(buildEditForm(member));
                          }}
                        >
                          Edit
                        </Button>
                        {onDeleteMember && (
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
                    <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                      No members match the current filters.
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
            <CardTitle>Edit Member</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="grid gap-4 md:grid-cols-2" onSubmit={handleEditMember}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-name">Edit Name</Label>
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
                <Label htmlFor="edit-role">Edit Role</Label>
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
                <Label htmlFor="edit-email">Edit Email</Label>
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
                label="Edit Status"
                value={editForm.status}
                onChange={(status) =>
                  setEditForm((state) =>
                    state ? { ...state, status } : state,
                  )
                }
              />
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-im-platform">Edit IM Platform</Label>
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
                <Label htmlFor="edit-im-user-id">Edit IM User ID</Label>
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
                    <Label htmlFor="edit-skills">Edit Skills</Label>
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
                      Agent Profile Settings
                    </h3>
                    <AgentProfileFields
                      mode="edit"
                      availableRoles={availableRoles}
                      value={editForm.agentProfile}
                      onChange={(agentProfile) =>
                        setEditForm((state) =>
                          state ? { ...state, agentProfile } : state,
                        )
                      }
                    />
                  </div>
                </>
              ) : (
                <div className="flex flex-col gap-2 md:col-span-2">
                  <Label htmlFor="edit-skills">Edit Skills</Label>
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
                <Button type="submit">Save Member</Button>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => {
                    setEditingMemberId(null);
                    setEditForm(null);
                  }}
                >
                  Cancel
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      ) : null}

      <ConfirmDialog
        open={deletingMemberId !== null}
        title="Delete Member"
        description={`Permanently remove "${deletingMember?.name ?? "this member"}" from the project? This cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => void handleConfirmDelete()}
        onCancel={() => setDeletingMemberId(null)}
      />
    </div>
  );
}
