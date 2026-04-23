"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  useSprintStore,
  type Sprint,
  type SprintStatus,
} from "@/lib/stores/sprint-store";

interface SprintManagementProps {
  projectId: string;
  sprints: Sprint[];
}

function statusBadgeVariant(status: SprintStatus): "default" | "secondary" | "outline" {
  switch (status) {
    case "active":
      return "default";
    case "planning":
      return "secondary";
    case "closed":
      return "outline";
    default:
      return "outline";
  }
}

function formatDate(iso: string): string {
  return iso.slice(0, 10);
}

export function SprintManagement({ projectId, sprints }: SprintManagementProps) {
  const t = useTranslations("sprints");
  const createSprint = useSprintStore((s) => s.createSprint);
  const updateSprint = useSprintStore((s) => s.updateSprint);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Form state
  const [name, setName] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [budget, setBudget] = useState("");

  const resetForm = () => {
    setName("");
    setStartDate("");
    setEndDate("");
    setBudget("");
    setError(null);
  };

  const handleCreate = async () => {
    if (!name.trim() || !startDate || !endDate) {
      setError(t("dialog.error.requiredFields"));
      return;
    }

    setCreating(true);
    setError(null);

    try {
      await createSprint(projectId, {
        name: name.trim(),
        startDate: new Date(startDate).toISOString(),
        endDate: new Date(endDate).toISOString(),
        totalBudgetUsd: parseFloat(budget) || 0,
      });
      resetForm();
      setDialogOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("dialog.error.createFailed"));
    } finally {
      setCreating(false);
    }
  };

  const handleStatusChange = async (sprint: Sprint, nextStatus: SprintStatus) => {
    try {
      await updateSprint(projectId, sprint.id, { status: nextStatus });
    } catch {
      // Silently handled -- the store retains the old state on failure
    }
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle>{t("card.title")}</CardTitle>
          <CardDescription>
            {t("card.description")}
          </CardDescription>
        </div>
        <Dialog open={dialogOpen} onOpenChange={(open) => { setDialogOpen(open); if (!open) resetForm(); }}>
          <DialogTrigger asChild>
            <Button size="sm">{t("dialog.createTitle")}</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("dialog.createTitle")}</DialogTitle>
              <DialogDescription>
                {t("dialog.createDescription")}
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="sprint-name">{t("dialog.name")}</Label>
                <Input
                  id="sprint-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={t("dialog.namePlaceholder")}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="sprint-start">{t("dialog.startDate")}</Label>
                  <Input
                    id="sprint-start"
                    type="date"
                    value={startDate}
                    onChange={(e) => setStartDate(e.target.value)}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="sprint-end">{t("dialog.endDate")}</Label>
                  <Input
                    id="sprint-end"
                    type="date"
                    value={endDate}
                    onChange={(e) => setEndDate(e.target.value)}
                  />
                </div>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="sprint-budget">{t("dialog.budgetUsd")}</Label>
                <Input
                  id="sprint-budget"
                  type="number"
                  min="0"
                  step="0.01"
                  value={budget}
                  onChange={(e) => setBudget(e.target.value)}
                  placeholder="0.00"
                />
              </div>
              {error && (
                <div className="text-sm text-destructive">{error}</div>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => { setDialogOpen(false); resetForm(); }}>
                {t("dialog.cancel")}
              </Button>
              <Button onClick={handleCreate} disabled={creating}>
                {creating ? t("dialog.saving") : t("dialog.create")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent>
        {sprints.length === 0 ? (
          <div className="text-sm text-muted-foreground">
            {t("empty.createFirst")}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("table.name")}</TableHead>
                <TableHead>{t("table.status")}</TableHead>
                <TableHead>{t("table.dateRange")}</TableHead>
                <TableHead>{t("table.budget")}</TableHead>
                <TableHead>{t("table.spent")}</TableHead>
                <TableHead className="text-right">{t("table.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sprints.map((sprint) => (
                <TableRow key={sprint.id}>
                  <TableCell className="font-medium">{sprint.name}</TableCell>
                  <TableCell>
                    <Badge variant={statusBadgeVariant(sprint.status)}>
                      {t(`status.${sprint.status}`)}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {formatDate(sprint.startDate)} - {formatDate(sprint.endDate)}
                  </TableCell>
                  <TableCell>${sprint.totalBudgetUsd.toFixed(2)}</TableCell>
                  <TableCell>${sprint.spentUsd.toFixed(2)}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      {sprint.status === "planning" && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleStatusChange(sprint, "active")}
                        >
                          {t("actions.activate")}
                        </Button>
                      )}
                      {sprint.status === "active" && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleStatusChange(sprint, "closed")}
                        >
                          {t("actions.close")}
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
