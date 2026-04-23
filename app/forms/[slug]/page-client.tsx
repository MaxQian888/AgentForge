"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { FormRenderer } from "@/components/forms/form-renderer";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useAuthStore } from "@/lib/stores/auth-store";
import { useFormStore } from "@/lib/stores/form-store";

export function PublicFormPageClient({ slug }: { slug: string }) {
  const t = useTranslations("forms");
  const router = useRouter();
  const form = useFormStore((state) => (slug ? state.formsBySlug[slug] ?? null : null));
  const fetchFormBySlug = useFormStore((state) => state.fetchFormBySlug);
  const bootstrapSession = useAuthStore((state) => state.bootstrapSession);
  const hasHydrated = useAuthStore((state) => state.hasHydrated);
  const status = useAuthStore((state) => state.status);
  const bootstrapStartedRef = useRef(false);
  const [requestState, setRequestState] = useState<"idle" | "success" | "error">("idle");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!hasHydrated || status !== "idle" || bootstrapStartedRef.current) {
      return;
    }

    bootstrapStartedRef.current = true;
    void bootstrapSession();
  }, [bootstrapSession, hasHydrated, status]);

  useEffect(() => {
    if (!hasHydrated || !slug) {
      return;
    }
    if (status === "checking" || status === "idle") {
      return;
    }

    let active = true;

    void fetchFormBySlug(slug)
      .then(() => {
        if (active) {
          setError(null);
          setRequestState("success");
        }
      })
      .catch((err: unknown) => {
        if (!active) {
          return;
        }
        if ((err as { status?: number })?.status === 401) {
          router.replace("/login");
          return;
        }
        setError(
          err instanceof Error && err.message ? err.message : t("formUnavailable")
        );
        setRequestState("error");
      });

    return () => {
      active = false;
    };
  }, [fetchFormBySlug, hasHydrated, router, slug, status, t]);

  if (
    !hasHydrated ||
    status === "checking" ||
    status === "idle" ||
    (requestState === "idle" && !form && !error)
  ) {
    return (
      <Card className="mx-auto max-w-2xl">
        <CardHeader>
          <CardTitle>{t("loadingForm")}</CardTitle>
          <CardDescription>{t("checkingAccess")}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  if (error === "form not found" || !form) {
    return (
      <Card className="mx-auto max-w-2xl">
        <CardHeader>
          <CardTitle>{t("formNotFound")}</CardTitle>
          <CardDescription>{t("noFormMatched", { slug })}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className="mx-auto max-w-2xl">
        <CardHeader>
          <CardTitle>{t("formUnavailable")}</CardTitle>
          <CardDescription>{error}</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card className="mx-auto max-w-2xl">
      <CardHeader>
        <CardTitle>{form.name}</CardTitle>
        <CardDescription>
          {t("submitDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <FormRenderer form={form} />
      </CardContent>
    </Card>
  );
}
