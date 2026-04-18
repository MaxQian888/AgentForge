"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  useInvitationStore,
  type InvitationDTO,
  type InvitationIdentity,
} from "@/lib/stores/invitation-store";

export interface PendingInvitationsPanelProps {
  projectId: string;
  canManage: boolean;
}

function describeIdentity(identity: InvitationIdentity): string {
  if (identity.kind === "email") {
    return identity.value ?? "";
  }
  const tail = identity.displayName ? ` (${identity.displayName})` : "";
  return `${identity.platform ?? ""} • ${identity.userId ?? ""}${tail}`;
}

function describeStatus(invitation: InvitationDTO): string {
  return invitation.status;
}

export function PendingInvitationsPanel({
  projectId,
  canManage,
}: PendingInvitationsPanelProps) {
  const invitations = useInvitationStore(
    (s) => s.invitationsByProject[projectId] ?? [],
  );
  const loading = useInvitationStore(
    (s) => s.loadingByProject[projectId] ?? false,
  );
  const error = useInvitationStore(
    (s) => s.errorByProject[projectId] ?? null,
  );
  const fetchInvitations = useInvitationStore((s) => s.fetchInvitations);
  const revokeInvitation = useInvitationStore((s) => s.revokeInvitation);
  const resendInvitation = useInvitationStore((s) => s.resendInvitation);
  const [busyId, setBusyId] = useState<string | null>(null);

  useEffect(() => {
    if (!projectId) return;
    void fetchInvitations(projectId);
  }, [projectId, fetchInvitations]);

  const pending = invitations.filter(
    (invitation) => invitation.status === "pending",
  );

  if (!projectId) return null;
  if (!loading && pending.length === 0 && !error) return null;

  const handleRevoke = async (invitationId: string) => {
    if (!canManage) return;
    setBusyId(invitationId);
    try {
      await revokeInvitation(projectId, invitationId);
    } finally {
      setBusyId(null);
    }
  };
  const handleResend = async (invitationId: string) => {
    if (!canManage) return;
    setBusyId(invitationId);
    try {
      await resendInvitation(projectId, invitationId);
    } finally {
      setBusyId(null);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Pending Invitations</CardTitle>
      </CardHeader>
      <CardContent>
        {error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : null}
        {pending.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No pending invitations.
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Invited Identity</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead>Delivery</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pending.map((invitation) => (
                <TableRow key={invitation.id}>
                  <TableCell>
                    <div className="flex flex-col gap-1">
                      <span>{describeIdentity(invitation.invitedIdentity)}</span>
                      <Badge variant="outline" className="w-fit text-xs">
                        {invitation.invitedIdentity.kind}
                      </Badge>
                    </div>
                  </TableCell>
                  <TableCell>{invitation.projectRole}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {invitation.expiresAt.slice(0, 16).replace("T", " ")} UTC
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    <div className="flex flex-col gap-1">
                      <span>{describeStatus(invitation)}</span>
                      {invitation.lastDeliveryStatus ? (
                        <span>{invitation.lastDeliveryStatus}</span>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={!canManage || busyId === invitation.id}
                        onClick={() => void handleResend(invitation.id)}
                      >
                        Resend
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="text-destructive"
                        disabled={!canManage || busyId === invitation.id}
                        onClick={() => void handleRevoke(invitation.id)}
                      >
                        Revoke
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
