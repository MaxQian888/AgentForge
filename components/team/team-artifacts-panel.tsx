"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { FileText, Code, CheckCircle, AlertCircle } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "@/lib/stores/auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

interface TeamArtifact {
  id: string;
  teamId: string;
  runId: string;
  role: string;
  key: string;
  value: unknown;
  createdAt: string;
}

interface TeamArtifactsPanelProps {
  teamId: string;
}

const roleIcons: Record<string, React.ElementType> = {
  planner: FileText,
  coder: Code,
  reviewer: CheckCircle,
};

const roleColors: Record<string, string> = {
  planner: "text-blue-600 dark:text-blue-400",
  coder: "text-emerald-600 dark:text-emerald-400",
  reviewer: "text-amber-600 dark:text-amber-400",
};

function formatArtifactValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === null || value === undefined) return "-";
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function ArtifactCard({ artifact }: { artifact: TeamArtifact }) {
  const Icon = roleIcons[artifact.role] ?? AlertCircle;
  const value = formatArtifactValue(artifact.value);
  const isJson = typeof artifact.value === "object" && artifact.value !== null;

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Icon
              className={cn(
                "size-4",
                roleColors[artifact.role] ?? "text-muted-foreground"
              )}
            />
            <span className="capitalize">{artifact.role}</span>
            <Badge variant="outline" className="text-[10px]">
              {artifact.key}
            </Badge>
          </CardTitle>
          <span className="text-xs text-muted-foreground">
            {new Date(artifact.createdAt).toLocaleTimeString()}
          </span>
        </div>
      </CardHeader>
      <CardContent>
        <ScrollArea className="max-h-64">
          {isJson ? (
            <pre className="whitespace-pre-wrap text-xs text-muted-foreground font-mono bg-muted/50 rounded-md p-3">
              {value}
            </pre>
          ) : (
            <p className="whitespace-pre-wrap text-sm text-muted-foreground">
              {value}
            </p>
          )}
        </ScrollArea>
      </CardContent>
    </Card>
  );
}

export function TeamArtifactsPanel({ teamId }: TeamArtifactsPanelProps) {
  const t = useTranslations("teams");
  const [artifacts, setArtifacts] = useState<TeamArtifact[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchArtifacts = useCallback(async () => {
    const token = useAuthStore.getState().accessToken;
    if (!token) return;

    setLoading(true);
    setError(null);
    try {
      const api = createApiClient(API_URL);
      const { data } = await api.get<TeamArtifact[]>(
        `/api/v1/teams/${teamId}/artifacts`,
        { token }
      );
      setArtifacts(data ?? []);
    } catch {
      setError(t("artifacts.loadError"));
    } finally {
      setLoading(false);
    }
  }, [teamId, t]);

  useEffect(() => {
    void fetchArtifacts();
  }, [fetchArtifacts]);

  const plannerArtifacts = artifacts.filter((a) => a.role === "planner");
  const coderArtifacts = artifacts.filter((a) => a.role === "coder");
  const reviewerArtifacts = artifacts.filter((a) => a.role === "reviewer");

  if (loading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-32 w-full rounded-lg" />
        <Skeleton className="h-32 w-full rounded-lg" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        {error}
      </div>
    );
  }

  if (artifacts.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
        {t("artifacts.empty")}
      </div>
    );
  }

  return (
    <Tabs defaultValue="all">
      <TabsList>
        <TabsTrigger value="all">{t("artifacts.all", { count: artifacts.length })}</TabsTrigger>
        {plannerArtifacts.length > 0 && (
          <TabsTrigger value="plan">
            {t("artifacts.plan", { count: plannerArtifacts.length })}
          </TabsTrigger>
        )}
        {coderArtifacts.length > 0 && (
          <TabsTrigger value="code">
            {t("artifacts.code", { count: coderArtifacts.length })}
          </TabsTrigger>
        )}
        {reviewerArtifacts.length > 0 && (
          <TabsTrigger value="review">
            {t("artifacts.review", { count: reviewerArtifacts.length })}
          </TabsTrigger>
        )}
      </TabsList>

      <TabsContent value="all" className="mt-4">
        <div className="flex flex-col gap-3">
          {artifacts.map((artifact) => (
            <ArtifactCard key={artifact.id} artifact={artifact} />
          ))}
        </div>
      </TabsContent>

      <TabsContent value="plan" className="mt-4">
        <div className="flex flex-col gap-3">
          {plannerArtifacts.map((artifact) => (
            <ArtifactCard key={artifact.id} artifact={artifact} />
          ))}
        </div>
      </TabsContent>

      <TabsContent value="code" className="mt-4">
        <div className="flex flex-col gap-3">
          {coderArtifacts.map((artifact) => (
            <ArtifactCard key={artifact.id} artifact={artifact} />
          ))}
        </div>
      </TabsContent>

      <TabsContent value="review" className="mt-4">
        <div className="flex flex-col gap-3">
          {reviewerArtifacts.map((artifact) => (
            <ArtifactCard key={artifact.id} artifact={artifact} />
          ))}
        </div>
      </TabsContent>
    </Tabs>
  );
}
