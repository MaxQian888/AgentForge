"use client";

import Link from "next/link";
import { AlertTriangle, CheckCircle2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { DashboardBootstrapSummary } from "@/lib/dashboard/summary";

interface ProjectBootstrapPanelProps {
  bootstrap?: DashboardBootstrapSummary;
}

export function ProjectBootstrapPanel({
  bootstrap,
}: ProjectBootstrapPanelProps) {
  if (!bootstrap || bootstrap.unresolvedCount === 0) {
    return null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Project bootstrap</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {bootstrap.phases.map((phase) => (
          <div
            key={phase.id}
            className="flex flex-col gap-2 rounded-lg border border-border/60 p-3 md:flex-row md:items-start md:justify-between"
          >
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                {phase.state === "ready" ? (
                  <CheckCircle2 className="size-4 text-emerald-600" />
                ) : (
                  <AlertTriangle className="size-4 text-amber-600" />
                )}
                <span className="font-medium">{phase.title}</span>
                <Badge
                  variant={phase.state === "ready" ? "secondary" : "outline"}
                  className="text-[11px]"
                >
                  {phase.state}
                </Badge>
              </div>
              <p className="text-sm text-muted-foreground">{phase.reason}</p>
            </div>
            {phase.state !== "ready" ? (
              <Button asChild size="sm" variant="outline">
                <Link href={phase.href}>{phase.actionLabel}</Link>
              </Button>
            ) : null}
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
