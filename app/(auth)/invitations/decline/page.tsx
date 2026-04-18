"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { declineInvitation } from "@/lib/stores/invitation-store";

function DeclineInvitationInner() {
  const params = useSearchParams();
  const token = (params.get("token") ?? "").trim();
  const [state, setState] = useState<"idle" | "working" | "done" | "error">(
    () => (token ? "working" : "error"),
  );
  const [error, setError] = useState<string | null>(() =>
    token ? null : "Missing invitation token",
  );

  useEffect(() => {
    if (!token) {
      return;
    }
    let cancelled = false;
    declineInvitation(token)
      .then(() => {
        if (!cancelled) setState("done");
      })
      .catch((err) => {
        if (cancelled) return;
        setError(
          err instanceof Error ? err.message : "Failed to decline invitation",
        );
        setState("error");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <div className="flex min-h-screen items-center justify-center p-6">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <CardTitle>Decline Invitation</CardTitle>
          <CardDescription>
            {state === "done"
              ? "The invitation has been declined."
              : "Processing your decline request..."}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {state === "working" ? (
            <p className="text-sm text-muted-foreground">Working...</p>
          ) : null}
          {state === "error" ? (
            <p className="text-sm text-destructive">{error}</p>
          ) : null}
          {state === "done" ? (
            <Button asChild variant="outline">
              <Link href="/">Close</Link>
            </Button>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

export default function DeclineInvitationPage() {
  return (
    <Suspense fallback={null}>
      <DeclineInvitationInner />
    </Suspense>
  );
}
