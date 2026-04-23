"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Plus, Archive, Pause, Play, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { useTranslations } from "next-intl";
import {
  useEmployeeStore,
  type CreateEmployeeInput,
  type Employee,
  type EmployeeState,
} from "@/lib/stores/employee-store";

const stateColor: Record<EmployeeState, "default" | "secondary" | "outline"> = {
  active: "default",
  paused: "secondary",
  archived: "outline",
};

interface EmployeesSectionProps {
  projectId: string | null;
}

export function EmployeesSection({ projectId }: EmployeesSectionProps) {
  const t = useTranslations("employees");
  const employeesByProject = useEmployeeStore((s) => s.employeesByProject);
  const loadingByProject = useEmployeeStore((s) => s.loadingByProject);
  const fetchEmployees = useEmployeeStore((s) => s.fetchEmployees);
  const createEmployee = useEmployeeStore((s) => s.createEmployee);
  const updateEmployee = useEmployeeStore((s) => s.updateEmployee);
  const setState = useEmployeeStore((s) => s.setState);
  const deleteEmployee = useEmployeeStore((s) => s.deleteEmployee);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<Employee | null>(null);

  useEffect(() => {
    if (projectId) void fetchEmployees(projectId);
  }, [projectId, fetchEmployees]);

  if (!projectId) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{t("selectProject")}</p>
        </CardContent>
      </Card>
    );
  }

  const employees = employeesByProject[projectId] ?? [];
  const loading = loadingByProject[projectId] ?? false;

  const handleOpenCreate = () => {
    setEditing(null);
    setDrawerOpen(true);
  };

  const handleOpenEdit = (emp: Employee) => {
    setEditing(emp);
    setDrawerOpen(true);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle>{t("title")}</CardTitle>
          <p className="text-sm text-muted-foreground mt-1">
            {t("description")}
          </p>
        </div>
        <Button onClick={handleOpenCreate} size="sm">
          <Plus className="h-4 w-4 mr-1" />
          {t("create")}
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground">{t("loading")}</p>
        ) : employees.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-sm text-muted-foreground">
              {t("empty")}
            </p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("table.name")}</TableHead>
                <TableHead>{t("table.role")}</TableHead>
                <TableHead>{t("table.status")}</TableHead>
                <TableHead>{t("table.createdAt")}</TableHead>
                <TableHead className="text-right">{t("table.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {employees.map((emp) => (
                <TableRow key={emp.id}>
                  <TableCell>
                    <div className="font-medium">{emp.displayName || emp.name}</div>
                    <div className="text-xs text-muted-foreground">{emp.name}</div>
                  </TableCell>
                  <TableCell>
                    <code className="text-xs bg-muted px-1 py-0.5 rounded">
                      {emp.roleId}
                    </code>
                  </TableCell>
                  <TableCell>
                    <Badge variant={stateColor[emp.state]}>{t(`status.${emp.state}`)}</Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(emp.createdAt).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button asChild variant="ghost" size="sm" className="mr-1">
                      <Link href={`/employees/${emp.id}/runs`}>{t("runsLink")}</Link>
                    </Button>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="sm">
                          ...
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => handleOpenEdit(emp)}>
                          {t("action.edit")}
                        </DropdownMenuItem>
                        {emp.state !== "active" && (
                          <DropdownMenuItem
                            onClick={() => setState(projectId, emp.id, "active")}
                          >
                            <Play className="h-3.5 w-3.5 mr-2" />
                            {t("action.activate")}
                          </DropdownMenuItem>
                        )}
                        {emp.state === "active" && (
                          <DropdownMenuItem
                            onClick={() => setState(projectId, emp.id, "paused")}
                          >
                            <Pause className="h-3.5 w-3.5 mr-2" />
                            {t("action.pause")}
                          </DropdownMenuItem>
                        )}
                        {emp.state !== "archived" && (
                          <DropdownMenuItem
                            onClick={() => setState(projectId, emp.id, "archived")}
                          >
                            <Archive className="h-3.5 w-3.5 mr-2" />
                            {t("action.archive")}
                          </DropdownMenuItem>
                        )}
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <DropdownMenuItem
                              onSelect={(e) => e.preventDefault()}
                              className="text-destructive"
                            >
                              <Trash2 className="h-3.5 w-3.5 mr-2" />
                              {t("action.delete")}
                            </DropdownMenuItem>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>
                                {t("delete.confirmTitle", { name: emp.name })}
                              </AlertDialogTitle>
                              <AlertDialogDescription>
                                {t("delete.confirmDescription")}
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>{t("delete.cancel")}</AlertDialogCancel>
                              <AlertDialogAction
                                onClick={() => deleteEmployee(projectId, emp.id)}
                              >
                                {t("delete.confirm")}
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>

      <EmployeeDrawer
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        projectId={projectId}
        editing={editing}
        onSave={async (input) => {
          if (editing) {
            const ok = await updateEmployee(projectId, editing.id, {
              displayName: input.displayName,
              roleId: input.roleId,
              runtimePrefs: input.runtimePrefs,
              config: input.config,
            });
            if (ok) setDrawerOpen(false);
          } else {
            const created = await createEmployee(projectId, input);
            if (created) setDrawerOpen(false);
          }
        }}
      />
    </Card>
  );
}

interface EmployeeDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
  editing: Employee | null;
  onSave: (input: CreateEmployeeInput) => Promise<void>;
}

function EmployeeDrawer({ open, onOpenChange, editing, onSave }: EmployeeDrawerProps) {
  const t = useTranslations("employees");
  const [name, setName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [roleId, setRoleId] = useState("code-reviewer");
  const [runtimePrefs, setRuntimePrefs] = useState<string>("{}");
  const [config, setConfig] = useState<string>("{}");
  const [skillsText, setSkillsText] = useState<string>("");

  useEffect(() => {
    // Reset form when the drawer opens or the edited record changes. React
    // batches the updates below into a single render; the cascading-render
    // lint warning is a false positive in this setState-reset pattern.
    if (!open) return;
    if (editing) {
      setName(editing.name);
      setDisplayName(editing.displayName ?? "");
      setRoleId(editing.roleId);
      setRuntimePrefs(JSON.stringify(editing.runtimePrefs ?? {}, null, 2));
      setConfig(JSON.stringify(editing.config ?? {}, null, 2));
      setSkillsText((editing.skills ?? []).map((s) => s.skillPath).join("\n"));
      return;
    }
    setName("");
    setDisplayName("");
    setRoleId("code-reviewer");
    setRuntimePrefs(
      JSON.stringify(
        { runtime: "claude_code", provider: "anthropic", model: "claude-opus-4-7", budgetUsd: 5 },
        null,
        2,
      ),
    );
    setConfig("{}");
    setSkillsText("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editing?.id, open]);

  const handleSave = async () => {
    let parsedPrefs: Record<string, unknown> = {};
    let parsedConfig: Record<string, unknown> = {};
    try {
      parsedPrefs = runtimePrefs.trim() ? JSON.parse(runtimePrefs) : {};
    } catch {
      alert(t("error.invalidJson.runtimePrefs"));
      return;
    }
    try {
      parsedConfig = config.trim() ? JSON.parse(config) : {};
    } catch {
      alert(t("error.invalidJson.config"));
      return;
    }

    const skills = skillsText
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((skillPath) => ({ skillPath, autoLoad: true }));

    await onSave({
      name: name.trim(),
      displayName: displayName.trim() || undefined,
      roleId: roleId.trim(),
      runtimePrefs: parsedPrefs,
      config: parsedConfig,
      skills: skills.length > 0 ? skills : undefined,
    });
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>
            {editing ? t("drawer.editTitle", { name: editing.name }) : t("drawer.createTitle")}
          </SheetTitle>
          <SheetDescription>
            {editing ? t("drawer.editDescription") : t("drawer.createDescription")}
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="emp-name">{t("form.name.label")}</Label>
            <Input
              id="emp-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!!editing}
              placeholder={t("form.name.placeholder")}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-display">{t("form.displayName.label")}</Label>
            <Input
              id="emp-display"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder={t("form.displayName.placeholder")}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-role">{t("form.roleId.label")}</Label>
            <Input
              id="emp-role"
              value={roleId}
              onChange={(e) => setRoleId(e.target.value)}
              placeholder={t("form.roleId.placeholder")}
            />
            <p className="text-xs text-muted-foreground">
              {t("form.roleId.hint")}
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-prefs">{t("form.runtimePrefs.label")}</Label>
            <Textarea
              id="emp-prefs"
              value={runtimePrefs}
              onChange={(e) => setRuntimePrefs(e.target.value)}
              rows={6}
              className="font-mono text-xs"
            />
            <p className="text-xs text-muted-foreground">
              {t("form.runtimePrefs.hint")}
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-config">{t("form.config.label")}</Label>
            <Textarea
              id="emp-config"
              value={config}
              onChange={(e) => setConfig(e.target.value)}
              rows={4}
              className="font-mono text-xs"
            />
            <p className="text-xs text-muted-foreground">
              {t("form.config.hint")}
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-skills">{t("form.skills.label")}</Label>
            <Textarea
              id="emp-skills"
              value={skillsText}
              onChange={(e) => setSkillsText(e.target.value)}
              rows={3}
              placeholder={t("form.skills.placeholder")}
            />
            <p className="text-xs text-muted-foreground">
              {t("form.skills.hint")}
            </p>
          </div>
        </div>

        <SheetFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("form.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={!name.trim() || !roleId.trim()}>
            {editing ? t("form.save") : t("form.create")}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
