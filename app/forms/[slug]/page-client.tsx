"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
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
          err instanceof Error && err.message ? err.message : "Unable to load form."
        );
        setRequestState("error");
      });

    return () => {
      active = false;
    };
  }, [fetchFormBySlug, hasHydrated, router, slug, status]);

  if (
    !hasHydrated ||
    status === "checking" ||
    status === "idle" ||
    (requestState === "idle" && !form && !error)
  ) {
    return (
      <Card className="mx-auto max-w-2xl">
        <CardHeader>
          <CardTitle>Loading form</CardTitle>
          <CardDescription>Checking access and loading form details...</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  if (error === "form not found" || !form) {
    return (
      <Card className="mx-auto max-w-2xl">
        <CardHeader>
          <CardTitle>Form not found</CardTitle>
          <CardDescription>No form matched slug &quot;{slug}&quot;.</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className="mx-auto max-w-2xl">
        <CardHeader>
          <CardTitle>Form unavailable</CardTitle>
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
          Submit this form to create a task in this project.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <FormRenderer form={form} />
      </CardContent>
    </Card>
  );
}
