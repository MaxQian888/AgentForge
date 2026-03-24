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
import type { CreateMemberInput, UpdateMemberInput } from "@/lib/stores/member-store";

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
  onRetry: () => void;
  onProjectChange: (projectId: string) => void;
  onCreateMember: (input: CreateMemberInput) => Promise<void>;
  onUpdateMember: (memberId: string, input: UpdateMemberInput) => Promise<void>;
}

const initialCreateState: CreateMemberInput = {
  name: "",
  type: "human",
  role: "",
  email: "",
  skills: [],
};

export function TeamManagement({
  projects,
  selectedProjectId,
  members,
  loading,
  error,
  onRetry,
  onProjectChange,
  onCreateMember,
  onUpdateMember,
}: TeamManagementProps) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [createForm, setCreateForm] = useState<CreateMemberInput>(initialCreateState);
  const [editingMemberId, setEditingMemberId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<UpdateMemberInput & { name: string }>({
    name: "",
    role: "",
    email: "",
    isActive: true,
  });

  const selectedProjectName = useMemo(
    () => projects.find((project) => project.id === selectedProjectId)?.name ?? "Team",
    [projects, selectedProjectId]
  );

  const handleCreateMember = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await onCreateMember({
      name: createForm.name,
      type: createForm.type,
      role: createForm.role?.trim() ?? "",
      email: createForm.email?.trim() ?? "",
      skills: createForm.skills ?? [],
    });
    setCreateForm(initialCreateState);
    setShowCreateForm(false);
  };

  const handleEditMember = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!editingMemberId) return;
    await onUpdateMember(editingMemberId, {
      name: editForm.name,
      role: editForm.role,
      email: editForm.email,
      isActive: editForm.isActive,
    });
    setEditingMemberId(null);
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
                      type: event.target.value as CreateMemberInput["type"],
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
              <div className="md:col-span-2">
                <Button type="submit">Create Member</Button>
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
                  <TableHead>Status</TableHead>
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
                      <div className="flex flex-col">
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
                      <Badge variant={member.isActive ? "secondary" : "outline"}>
                        {member.status}
                      </Badge>
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
                          setEditForm({
                            name: member.name,
                            role: member.role,
                            email: member.email,
                            isActive: member.isActive,
                          });
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

      {editingMemberId ? (
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
                    setEditForm((state) => ({ ...state, name: event.target.value }))
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-role">Edit Role</Label>
                <Input
                  id="edit-role"
                  aria-label="Edit Role"
                  value={editForm.role ?? ""}
                  onChange={(event) =>
                    setEditForm((state) => ({ ...state, role: event.target.value }))
                  }
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-email">Edit Email</Label>
                <Input
                  id="edit-email"
                  aria-label="Edit Email"
                  type="email"
                  value={editForm.email ?? ""}
                  onChange={(event) =>
                    setEditForm((state) => ({ ...state, email: event.target.value }))
                  }
                />
              </div>
              <div className="flex items-center gap-2 pt-7">
                <input
                  id="edit-active"
                  aria-label="Edit Active"
                  type="checkbox"
                  checked={editForm.isActive ?? false}
                  onChange={(event) =>
                    setEditForm((state) => ({
                      ...state,
                      isActive: event.target.checked,
                    }))
                  }
                />
                <Label htmlFor="edit-active">Active member</Label>
              </div>
              <div className="md:col-span-2 flex gap-2">
                <Button type="submit">Save Member</Button>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setEditingMemberId(null)}
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
