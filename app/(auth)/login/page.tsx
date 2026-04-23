"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useAuthStore } from "@/lib/stores/auth-store";

function normalizeEmail(value: string): string {
  return value.trim().toLowerCase();
}

export default function LoginPage() {
  const router = useRouter();
  const t = useTranslations("auth");
  const login = useAuthStore((s) => s.login);
  const bootstrapSession = useAuthStore((s) => s.bootstrapSession);
  const status = useAuthStore((s) => s.status);
  const hasHydrated = useAuthStore((s) => s.hasHydrated);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (status !== "authenticated") {
      return;
    }

    router.replace("/");
  }, [router, status]);

  useEffect(() => {
    if (!hasHydrated || status !== "idle") {
      return;
    }

    void bootstrapSession();
  }, [bootstrapSession, hasHydrated, status]);

  const sessionPending =
    !hasHydrated ||
    status === "idle" ||
    status === "checking" ||
    status === "authenticated";

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(normalizeEmail(email), password);
      router.push("/");
    } catch (err) {
      setError(
        err instanceof Error && err.message
          ? err.message
          : t("login.defaultError")
      );
    } finally {
      setLoading(false);
    }
  };

  if (sessionPending) {
    return (
      <div className="flex h-full min-h-full items-center justify-center bg-background p-4">
        <Card className="w-full max-w-sm">
          <CardHeader>
            <CardTitle className="text-2xl">{t("login.title")}</CardTitle>
            <CardDescription>{t("login.checkingSession")}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-full items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-2xl">{t("login.title")}</CardTitle>
          <CardDescription>{t("login.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}
            <div className="flex flex-col gap-2">
              <Label htmlFor="email">{t("login.email")}</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete="email"
                disabled={loading}
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password">{t("login.password")}</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                disabled={loading}
                required
              />
            </div>
            <Button type="submit" disabled={loading} className="w-full">
              {loading ? t("login.submitting") : t("login.submit")}
            </Button>
            <p className="text-center text-sm text-muted-foreground">
              {t("login.noAccount")}{" "}
              <Link href="/register" className="text-primary underline">
                {t("login.register")}
              </Link>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
