"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("invitations");
  const token = (params.get("token") ?? "").trim();
  const authStatus = useAuthStore((s) => s.status);
  const [preview, setPreview] = useState<InvitationPublicPreview | null>(null);
  const [loading, setLoading] = useState(() => !!token);
  const [error, setError] = useState<string | null>(() =>
    token ? null : t("decline.missingToken"),
  );
  const [actionBusy, setActionBusy] = useState<
    "accept" | "decline" | null
  >(null);
  const [outcome, setOutcome] = useState<
    "accepted" | "declined" | null
  >(null);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    fetchInvitationPreview(token)
      .then((data) => {
        if (!cancelled) setPreview(data);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : t("accept.loading"),
          );
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [token, t]);

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
        err instanceof Error ? err.message : t("dialog.errorSendFailed"),
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
        err instanceof Error ? err.message : t("dialog.errorSendFailed"),
      );
    } finally {
      setActionBusy(null);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center p-6">
      <Card className="w-full max-w-xl">
        <CardHeader>
          <CardTitle>{t("accept.title")}</CardTitle>
          <CardDescription>{t("accept.description")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {loading ? (
            <p className="text-sm text-muted-foreground">
              {t("accept.loading")}
            </p>
          ) : null}

          {error ? <p className="text-sm text-destructive">{error}</p> : null}

          {preview && !outcome ? (
            <div className="flex flex-col gap-2 text-sm">
              <div>
                <span className="font-medium">{t("accept.project")}:</span>{" "}
                {preview.projectName}
              </div>
              <div>
                <span className="font-medium">{t("accept.role")}:</span>{" "}
                {preview.projectRole}
              </div>
              <div>
                <span className="font-medium">{t("accept.invitedBy")}:</span>{" "}
                {preview.inviterName || preview.inviterEmail || "Admin"}
              </div>
              <div>
                <span className="font-medium">{t("accept.expires")}:</span>{" "}
                {preview.expiresAt.slice(0, 16).replace("T", " ")} UTC
              </div>
              {preview.identityHint ? (
                <div className="text-muted-foreground">
                  {t("accept.forIdentity")}: {preview.identityHint}
                </div>
              ) : null}
              {preview.message ? (
                <div className="mt-2 rounded-md border bg-muted/30 p-3 text-xs">
                  {preview.message}
                </div>
              ) : null}
              {preview.status !== "pending" ? (
                <p className="text-sm text-destructive">
                  {t("accept.notPending", { status: preview.status })}
                </p>
              ) : null}
            </div>
          ) : null}

          {outcome === "accepted" ? (
            <div className="flex flex-col gap-3">
              <p className="text-sm">
                {t("accept.accepted")}
              </p>
              <Button asChild>
                <Link href="/">{t("accept.goToDashboard")}</Link>
              </Button>
            </div>
          ) : null}

          {outcome === "declined" ? (
            <div className="flex flex-col gap-3">
              <p className="text-sm">{t("accept.declined")}</p>
              <Button asChild variant="outline">
                <Link href="/">{t("accept.close")}</Link>
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
                  {t("accept.signInToAccept")}
                </Button>
              ) : (
                <Button
                  type="button"
                  disabled={actionBusy !== null}
                  onClick={() => void handleAccept()}
                >
                  {actionBusy === "accept" ? t("accept.accepting") : t("accept.accept")}
                </Button>
              )}
              <Button
                type="button"
                variant="outline"
                disabled={actionBusy !== null}
                onClick={() => void handleDecline()}
              >
                {actionBusy === "decline" ? t("accept.declining") : t("accept.decline")}
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
