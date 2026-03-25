"use client";

import { Suspense } from "react";
import { useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { Pause, Play, Skull } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { OutputStream } from "@/components/agent/output-stream";
import { useAgentStore } from "@/lib/stores/agent-store";
import { cn } from "@/lib/utils";

function AgentView() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const agentId = searchParams.get("id");
  const agent = useAgentStore((s) =>
    agentId ? s.agents.find((a) => a.id === agentId) : undefined
  );
  const outputs = useAgentStore((s) =>
    agentId ? s.agentOutputs.get(agentId) ?? [] : []
  );
  const fetchAgent = useAgentStore((s) => s.fetchAgent);
  const pauseAgent = useAgentStore((s) => s.pauseAgent);
  const resumeAgent = useAgentStore((s) => s.resumeAgent);
  const killAgent = useAgentStore((s) => s.killAgent);

  useEffect(() => {
    if (agentId) {
      void fetchAgent(agentId);
    }
  }, [agentId, fetchAgent]);

  if (!agentId) {
    router.replace("/agents");
    return null;
  }

  if (!agent) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="text-muted-foreground">Agent not found</p>
      </div>
    );
  }

  const costPct = agent.budget > 0 ? (agent.cost / agent.budget) * 100 : 0;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{agent.roleName}</h1>
          <p className="text-muted-foreground">{agent.taskTitle}</p>
        </div>
        <div className="flex gap-2">
          {agent.status === "running" && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => pauseAgent(agent.id)}
            >
              <Pause className="mr-1 size-4" />
              Pause
            </Button>
          )}
          {agent.status === "paused" && (
            <Button variant="outline" size="sm" onClick={() => resumeAgent(agent.id)}>
              <Play className="mr-1 size-4" />
              Resume
            </Button>
          )}
          {(agent.status === "running" || agent.status === "paused") && (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => killAgent(agent.id)}
            >
              <Skull className="mr-1 size-4" />
              Kill
            </Button>
          )}
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Status
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="secondary">{agent.status}</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Runtime
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <p className="font-medium">{agent.runtime || "-"}</p>
            <p className="text-muted-foreground">
              {agent.provider || "-"} / {agent.model || "-"}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Turns
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">{agent.turns}</span>
          </CardContent>
        </Card>
        <Card className="sm:col-span-3">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Cost
            </CardTitle>
          </CardHeader>
          <CardContent>
            <span className="text-2xl font-bold">
              ${agent.cost.toFixed(2)}
            </span>
            <span className="text-sm text-muted-foreground">
              {" "}
              / ${agent.budget.toFixed(2)}
            </span>
            <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                className={cn(
                  "h-full rounded-full",
                  costPct > 80 ? "bg-destructive" : "bg-primary"
                )}
                style={{ width: `${Math.min(costPct, 100)}%` }}
              />
            </div>
          </CardContent>
        </Card>
      </div>

      <Separator />

      <div>
        <h2 className="mb-3 text-lg font-semibold">Output Stream</h2>
        <OutputStream lines={outputs} />
      </div>
    </div>
  );
}

export default function AgentPage() {
  return (
    <Suspense>
      <AgentView />
    </Suspense>
  );
}
