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
          <CardTitle>员工 (Employees)</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">请选择项目以查看员工列表。</p>
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
          <CardTitle>员工 (Employees)</CardTitle>
          <p className="text-sm text-muted-foreground mt-1">
            员工是持久化的能力载体。每个员工绑定一个 role、可扩展技能、拥有独立记忆命名空间，在 workflow 的 llm_agent 节点中被复用。
          </p>
        </div>
        <Button onClick={handleOpenCreate} size="sm">
          <Plus className="h-4 w-4 mr-1" />
          新建员工
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground">加载中...</p>
        ) : employees.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-sm text-muted-foreground">
              当前项目还没有员工。点击&ldquo;新建员工&rdquo;创建第一个。
            </p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>名称</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>创建时间</TableHead>
                <TableHead className="text-right">操作</TableHead>
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
                    <Badge variant={stateColor[emp.state]}>{emp.state}</Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(emp.createdAt).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button asChild variant="ghost" size="sm" className="mr-1">
                      <Link href={`/employees/${emp.id}/runs`}>Runs</Link>
                    </Button>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="sm">
                          ...
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => handleOpenEdit(emp)}>
                          编辑
                        </DropdownMenuItem>
                        {emp.state !== "active" && (
                          <DropdownMenuItem
                            onClick={() => setState(projectId, emp.id, "active")}
                          >
                            <Play className="h-3.5 w-3.5 mr-2" />
                            启用
                          </DropdownMenuItem>
                        )}
                        {emp.state === "active" && (
                          <DropdownMenuItem
                            onClick={() => setState(projectId, emp.id, "paused")}
                          >
                            <Pause className="h-3.5 w-3.5 mr-2" />
                            暂停
                          </DropdownMenuItem>
                        )}
                        {emp.state !== "archived" && (
                          <DropdownMenuItem
                            onClick={() => setState(projectId, emp.id, "archived")}
                          >
                            <Archive className="h-3.5 w-3.5 mr-2" />
                            归档
                          </DropdownMenuItem>
                        )}
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <DropdownMenuItem
                              onSelect={(e) => e.preventDefault()}
                              className="text-destructive"
                            >
                              <Trash2 className="h-3.5 w-3.5 mr-2" />
                              删除
                            </DropdownMenuItem>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>
                                确认删除员工 {emp.name}?
                              </AlertDialogTitle>
                              <AlertDialogDescription>
                                此操作将永久删除该员工及其所有技能绑定、执行历史。
                                记忆（agent_memory 表中 scope=employee 的行）会随之清除。
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>取消</AlertDialogCancel>
                              <AlertDialogAction
                                onClick={() => deleteEmployee(projectId, emp.id)}
                              >
                                确认删除
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
      alert("Runtime prefs 必须是合法 JSON");
      return;
    }
    try {
      parsedConfig = config.trim() ? JSON.parse(config) : {};
    } catch {
      alert("Config 必须是合法 JSON");
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
          <SheetTitle>{editing ? `编辑员工: ${editing.name}` : "新建员工"}</SheetTitle>
          <SheetDescription>
            {editing
              ? "修改员工配置。Name 一经创建不可更改。"
              : "员工一经创建后，Name 字段不可再修改（用作唯一标识）。"}
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="emp-name">Name (唯一标识)</Label>
            <Input
              id="emp-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={!!editing}
              placeholder="如: product-selector"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-display">Display Name (可选)</Label>
            <Input
              id="emp-display"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="如: 选品员工"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-role">Role ID</Label>
            <Input
              id="emp-role"
              value={roleId}
              onChange={(e) => setRoleId(e.target.value)}
              placeholder="如: code-reviewer, coder, planner"
            />
            <p className="text-xs text-muted-foreground">
              必须对应 roles/&lt;id&gt;/role.yaml 中注册的 role。
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-prefs">Runtime Prefs (JSON)</Label>
            <Textarea
              id="emp-prefs"
              value={runtimePrefs}
              onChange={(e) => setRuntimePrefs(e.target.value)}
              rows={6}
              className="font-mono text-xs"
            />
            <p className="text-xs text-muted-foreground">
              示例: {'{"runtime":"claude_code","provider":"anthropic","model":"claude-opus-4-7","budgetUsd":5}'}
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-config">Config (JSON)</Label>
            <Textarea
              id="emp-config"
              value={config}
              onChange={(e) => setConfig(e.target.value)}
              rows={4}
              className="font-mono text-xs"
            />
            <p className="text-xs text-muted-foreground">
              可用字段 system_prompt_override 覆盖 role 默认系统提示。
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="emp-skills">额外技能 (每行一个路径)</Label>
            <Textarea
              id="emp-skills"
              value={skillsText}
              onChange={(e) => setSkillsText(e.target.value)}
              rows={3}
              placeholder="skills/typescript&#10;skills/go"
            />
            <p className="text-xs text-muted-foreground">
              在 role 基础技能之外追加；路径格式与 role manifest 中 skills[].path 相同。
            </p>
          </div>
        </div>

        <SheetFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button onClick={handleSave} disabled={!name.trim() || !roleId.trim()}>
            {editing ? "保存修改" : "创建员工"}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
