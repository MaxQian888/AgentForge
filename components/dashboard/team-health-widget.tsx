"use client";

import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface TeamMemberHealth {
  id: string;
  name: string;
  role: string;
  workloadPercent: number;
  status: string;
}

interface TeamHealthWidgetProps {
  members: TeamMemberHealth[];
}

function workloadColor(percent: number): string {
  if (percent >= 90) return "bg-red-500";
  if (percent >= 70) return "bg-amber-500";
  return "bg-emerald-500";
}

export function TeamHealthWidget({ members }: TeamHealthWidgetProps) {
  const t = useTranslations("dashboard");
  const commonT = useTranslations("common");

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">
          {t("teamHealth.title")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {members.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("teamHealth.empty")}
          </p>
        ) : (
          <div className="space-y-3">
            {members.map((member) => (
              <div key={member.id} className="space-y-1">
                <div className="flex items-center justify-between text-sm">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{member.name}</span>
                    <span className="text-xs text-muted-foreground">
                      {member.role}
                    </span>
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {commonT(`status.${member.status}`)}
                  </span>
                </div>
                <div className="h-1.5 w-full rounded-full bg-muted">
                  <div
                    className={cn(
                      "h-1.5 rounded-full transition-all",
                      workloadColor(member.workloadPercent)
                    )}
                    style={{ width: `${Math.min(member.workloadPercent, 100)}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
