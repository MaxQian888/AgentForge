"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Bot, ListTodo, DollarSign, ClipboardCheck } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useTaskStore } from "@/lib/stores/task-store";

const summaryCards = [
  { key: "agents", label: "Active Agents", icon: Bot, value: 0 },
  { key: "tasks", label: "Tasks In Progress", icon: ListTodo, value: 0 },
  { key: "cost", label: "Cost This Week", icon: DollarSign, value: "$0.00" },
  { key: "reviews", label: "Pending Reviews", icon: ClipboardCheck, value: 0 },
] as const;

export default function DashboardPage() {
  const agents = useAgentStore((s) => s.agents);
  const tasks = useTaskStore((s) => s.tasks);
  const fetchAgents = useAgentStore((s) => s.fetchAgents);

  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
    fetchAgents();
  }, [fetchAgents]);

  const activeAgents = agents.filter((a) => a.status === "running").length;
  const inProgress = tasks.filter((t) => t.status === "in_progress").length;
  const inReview = tasks.filter((t) => t.status === "in_review").length;
  const weekCost = agents.reduce((sum, a) => sum + a.cost, 0);

  const values = {
    agents: activeAgents,
    tasks: inProgress,
    cost: `$${weekCost.toFixed(2)}`,
    reviews: inReview,
  };

  if (!mounted) return null;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <div className="flex gap-2">
          <Button asChild size="sm">
            <Link href="/projects">View Projects</Link>
          </Button>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {summaryCards.map((card) => {
          const Icon = card.icon;
          return (
            <Card key={card.key}>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  {card.label}
                </CardTitle>
                <Icon className="size-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {values[card.key]}
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Recent Activity</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Activity feed will appear here once agents and tasks are active.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
