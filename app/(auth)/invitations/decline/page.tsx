"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("invitations");
  const token = (params.get("token") ?? "").trim();
  const [state, setState] = useState<"idle" | "working" | "done" | "error">(
    () => (token ? "working" : "error"),
  );
  const [error, setError] = useState<string | null>(() =>
    token ? null : t("decline.missingToken"),
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
          err instanceof Error ? err.message : t("dialog.errorSendFailed"),
        );
        setState("error");
      });
    return () => {
      cancelled = true;
    };
  }, [token, t]);

  return (
    <div className="flex min-h-screen items-center justify-center p-6">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <CardTitle>{t("decline.title")}</CardTitle>
          <CardDescription>
            {state === "done"
              ? t("decline.done")
              : t("accept.loading")}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {state === "working" ? (
            <p className="text-sm text-muted-foreground">{t("decline.working")}</p>
          ) : null}
          {state === "error" ? (
            <p className="text-sm text-destructive">{error}</p>
          ) : null}
          {state === "done" ? (
            <Button asChild variant="outline">
              <Link href="/">{t("accept.close")}</Link>
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
