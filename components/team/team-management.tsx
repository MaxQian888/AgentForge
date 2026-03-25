"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { Plus, Users } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { TeamMember } from "@/lib/dashboard/summary";
import type {
  CreateMemberInput,
  UpdateMemberInput,
} from "@/lib/stores/member-store";
import type { RoleManifest } from "@/lib/stores/role-store";
import type { AgentProfileDraft } from "@/lib/team/agent-profile";

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
}

interface MemberFormState {
  name: string;
  type: "human" | "agent";
  role: string;
  email: string;
  skillsInput: string;
  isActive: boolean;
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
    email: "",
    skillsInput: "",
    isActive: true,
    agentProfile: EMPTY_AGENT_PROFILE,
  };
}

function buildEditForm(member: TeamMember): MemberFormState {
  return {
    name: member.name,
    type: member.type,
    role: member.role,
    email: member.email,
    skillsInput: member.skills.join(", "),
    isActive: member.isActive,
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
      <select
        id={id}
        aria-label={label}
        className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">Unbound role</option>
        {availableRoles.map((role) => (
          <option key={role.metadata.id} value={role.metadata.id}>
            {role.metadata.name}
          </option>
        ))}
      </select>
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
        value={value.roleId}
        availableRoles={availableRoles}
        onChange={(roleId) => onChange({ ...value, roleId })}
      />
      <div className="flex flex-col gap-2">
        <Label htmlFor={mode === "edit" ? "edit-agent-budget" : "agent-budget"}>
          {prefix}Agent Budget USD
        </Label>
        <Input
          id={mode === "edit" ? "edit-agent-budget" : "agent-budget"}
          aria-label={`${prefix}Agent Budget USD`}
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
          aria-label={`${prefix}Runtime`}
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
          aria-label={`${prefix}Provider`}
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
          aria-label={`${prefix}Model`}
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
          aria-label={`${prefix}Agent Notes`}
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
}: TeamManagementProps) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [createForm, setCreateForm] = useState<MemberFormState>(() =>
    buildInitialCreateForm(),
  );
  const [editingMemberId, setEditingMemberId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<MemberFormState | null>(null);

  const selectedProjectName = useMemo(
    () =>
      projects.find((project) => project.id === selectedProjectId)?.name ?? "Team",
    [projects, selectedProjectId],
  );

  const editingMember = useMemo(
    () => members.find((member) => member.id === editingMemberId) ?? null,
    [editingMemberId, members],
  );

  const handleCreateMember = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const payload: CreateMemberInput = {
      name: createForm.name.trim(),
      type: createForm.type,
      role: createForm.role.trim(),
      email: createForm.email.trim(),
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
      email: editForm.email.trim(),
      skills: parseCommaList(editForm.skillsInput),
      isActive: editForm.isActive,
    };

    if (editForm.type === "agent") {
      payload.agentProfile = { ...editForm.agentProfile };
    }

    await onUpdateMember(editingMemberId, payload);
    setEditingMemberId(null);
    setEditForm(null);
  };

  if (loading) {
    return <p className="text-sm text-muted-foreground">Loading team roster...</p>;
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
            <Label htmlFor="project-selector">Project</Label>
            <select
              id="project-selector"
              aria-label="Project"
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={selectedProjectId ?? ""}
              onChange={(event) => onProjectChange(event.target.value)}
            >
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
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
                  aria-label="Member Name"
                  value={createForm.name}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, name: event.target.value }))
                  }
                  required
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-type">Member Type</Label>
                <select
                  id="member-type"
                  aria-label="Member Type"
                  className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                  value={createForm.type}
                  onChange={(event) =>
                    setCreateForm((state) => ({
                      ...state,
                      type: event.target.value as MemberFormState["type"],
                    }))
                  }
                >
                  <option value="human">Human</option>
                  <option value="agent">Agent</option>
                </select>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-role">Role</Label>
                <Input
                  id="member-role"
                  aria-label="Role"
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
                  aria-label="Email"
                  type="email"
                  value={createForm.email}
                  onChange={(event) =>
                    setCreateForm((state) => ({ ...state, email: event.target.value }))
                  }
                />
              </div>
              {createForm.type === "agent" ? (
                <>
                  <div className="flex flex-col gap-2 md:col-span-2">
                    <Label htmlFor="member-skills">Skills</Label>
                    <Input
                      id="member-skills"
                      aria-label="Skills"
                      value={createForm.skillsInput}
                      onChange={(event) =>
                        setCreateForm((state) => ({
                          ...state,
                          skillsInput: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className="md:col-span-2 rounded-lg border p-4">
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
          <CardHeader>
            <CardTitle>Unified Roster</CardTitle>
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
                {members.map((member) => (
                  <TableRow key={member.id}>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <span className="font-medium">{member.name}</span>
                        <span className="text-xs text-muted-foreground">
                          {member.email || "No direct email"}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary">{member.typeLabel}</Badge>
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
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setEditingMemberId(member.id);
                          setEditForm(buildEditForm(member));
                        }}
                      >
                        Edit {member.name}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
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
                  aria-label="Edit Name"
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
                  aria-label="Edit Role"
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
                  aria-label="Edit Email"
                  type="email"
                  value={editForm.email}
                  onChange={(event) =>
                    setEditForm((state) =>
                      state ? { ...state, email: event.target.value } : state,
                    )
                  }
                />
              </div>
              <div className="flex items-center gap-2 pt-7">
                <input
                  id="edit-active"
                  aria-label="Edit Active"
                  type="checkbox"
                  checked={editForm.isActive}
                  onChange={(event) =>
                    setEditForm((state) =>
                      state ? { ...state, isActive: event.target.checked } : state,
                    )
                  }
                />
                <Label htmlFor="edit-active">Active member</Label>
              </div>
              {editForm.type === "agent" ? (
                <>
                  <div className="flex flex-col gap-2 md:col-span-2">
                    <Label htmlFor="edit-skills">Edit Skills</Label>
                    <Input
                      id="edit-skills"
                      aria-label="Edit Skills"
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
                  <div className="md:col-span-2 rounded-lg border p-4">
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
                    aria-label="Edit Skills"
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
    </div>
  );
}
