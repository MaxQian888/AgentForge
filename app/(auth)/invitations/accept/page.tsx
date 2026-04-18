"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useAuthStore } from "@/lib/stores/auth-store";
import {
  acceptInvitation,
  declineInvitation,
  fetchInvitationPreview,
  type InvitationPublicPreview,
} from "@/lib/stores/invitation-store";

function AcceptInvitationInner() {
  const router = useRouter();
  const params = useSearchParams();
  const token = (params.get("token") ?? "").trim();
  const authStatus = useAuthStore((s) => s.status);
  const [preview, setPreview] = useState<InvitationPublicPreview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionBusy, setActionBusy] = useState<
    "accept" | "decline" | null
  >(null);
  const [outcome, setOutcome] = useState<
    "accepted" | "declined" | null
  >(null);

  useEffect(() => {
    if (!token) {
      setError("Missing invitation token");
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchInvitationPreview(token)
      .then((data) => {
        if (!cancelled) setPreview(data);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : "Invitation not found",
          );
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  const handleAccept = async () => {
    if (!token) return;
    if (authStatus !== "authenticated") {
      router.push(
        `/login?redirect=${encodeURIComponent(
          `/invitations/accept?token=${token}`,
        )}`,
      );
      return;
    }
    setActionBusy("accept");
    setError(null);
    try {
      await acceptInvitation(token);
      setOutcome("accepted");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to accept invitation",
      );
    } finally {
      setActionBusy(null);
    }
  };

  const handleDecline = async () => {
    if (!token) return;
    setActionBusy("decline");
    setError(null);
    try {
      await declineInvitation(token);
      setOutcome("declined");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to decline invitation",
      );
    } finally {
      setActionBusy(null);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center p-6">
      <Card className="w-full max-w-xl">
        <CardHeader>
          <CardTitle>Project Invitation</CardTitle>
          <CardDescription>
            Review the invitation details below and choose whether to accept
            or decline.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {loading ? (
            <p className="text-sm text-muted-foreground">
              Loading invitation...
            </p>
          ) : null}

          {error ? <p className="text-sm text-destructive">{error}</p> : null}

          {preview && !outcome ? (
            <div className="flex flex-col gap-2 text-sm">
              <div>
                <span className="font-medium">Project:</span>{" "}
                {preview.projectName}
              </div>
              <div>
                <span className="font-medium">Role:</span>{" "}
                {preview.projectRole}
              </div>
              <div>
                <span className="font-medium">Invited by:</span>{" "}
                {preview.inviterName || preview.inviterEmail || "Admin"}
              </div>
              <div>
                <span className="font-medium">Expires:</span>{" "}
                {preview.expiresAt.slice(0, 16).replace("T", " ")} UTC
              </div>
              {preview.identityHint ? (
                <div className="text-muted-foreground">
                  For identity: {preview.identityHint}
                </div>
              ) : null}
              {preview.message ? (
                <div className="mt-2 rounded-md border bg-muted/30 p-3 text-xs">
                  {preview.message}
                </div>
              ) : null}
              {preview.status !== "pending" ? (
                <p className="text-sm text-destructive">
                  This invitation is {preview.status} and cannot be actioned.
                </p>
              ) : null}
            </div>
          ) : null}

          {outcome === "accepted" ? (
            <div className="flex flex-col gap-3">
              <p className="text-sm">
                Invitation accepted. You are now a member of the project.
              </p>
              <Button asChild>
                <Link href="/">Go to dashboard</Link>
              </Button>
            </div>
          ) : null}

          {outcome === "declined" ? (
            <div className="flex flex-col gap-3">
              <p className="text-sm">Invitation declined.</p>
              <Button asChild variant="outline">
                <Link href="/">Close</Link>
              </Button>
            </div>
          ) : null}

          {preview && !outcome && preview.status === "pending" ? (
            <div className="flex items-center gap-2">
              {authStatus !== "authenticated" ? (
                <Button
                  type="button"
                  onClick={() =>
                    router.push(
                      `/login?redirect=${encodeURIComponent(
                        `/invitations/accept?token=${token}`,
                      )}`,
                    )
                  }
                >
                  Sign in to accept
                </Button>
              ) : (
                <Button
                  type="button"
                  disabled={actionBusy !== null}
                  onClick={() => void handleAccept()}
                >
                  {actionBusy === "accept" ? "Accepting..." : "Accept"}
                </Button>
              )}
              <Button
                type="button"
                variant="outline"
                disabled={actionBusy !== null}
                onClick={() => void handleDecline()}
              >
                {actionBusy === "decline" ? "Declining..." : "Decline"}
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

export default function AcceptInvitationPage() {
  return (
    <Suspense fallback={null}>
      <AcceptInvitationInner />
    </Suspense>
  );
}
