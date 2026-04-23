"use client";

import { useTranslations } from "next-intl";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { TaskDetailContent } from "@/components/tasks/task-detail-content";
import { useAgentStore } from "@/lib/stores/agent-store";
import { useMemberStore } from "@/lib/stores/member-store";
import {
  useTaskStore,
  type Task,
} from "@/lib/stores/task-store";

interface TaskDetailPanelProps {
  task: Task | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function TaskDetailPanel({
  task,
  open,
  onOpenChange,
}: TaskDetailPanelProps) {
  const t = useTranslations("tasks");
  const tasks = useTaskStore((state) => state.tasks);
  const updateTask = useTaskStore((state) => state.updateTask);
  const assignTask = useTaskStore((state) => state.assignTask);
  const transitionTask = useTaskStore((state) => state.transitionTask);
  const decomposeTask = useTaskStore((state) => state.decomposeTask);
  const spawnAgent = useAgentStore((state) => state.spawnAgent);
  const members = useMemberStore(
    (state) => state.membersByProject[task?.projectId ?? ""] ?? []
  );
  const agents = useAgentStore((state) =>
    state.agents.filter((agent) => tasks.some((item) => item.projectId === task?.projectId && item.id === agent.taskId))
  );
  const handleTaskAssign = async (
    taskId: string,
    assigneeId: string,
    assigneeType: "human" | "agent"
  ) => {
    const member = members.find((item) => item.id === assigneeId);
    await assignTask(taskId, assigneeId, assigneeType, member?.name);
  };

  if (!task) return null;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{t("detail.title")}</SheetTitle>
          <SheetDescription>
            {t("detail.panelDescription")}
          </SheetDescription>
        </SheetHeader>
        <TaskDetailContent
          key={task.id}
          task={task}
          tasks={tasks.filter((item) => item.projectId === task.projectId)}
          members={members}
          agents={agents}
          onTaskSave={async (taskId, data) => {
            await updateTask(taskId, data);
            onOpenChange(false);
          }}
          onTaskAssign={handleTaskAssign}
          onTaskStatusChange={transitionTask}
          onTaskDecompose={decomposeTask}
          onSpawnAgent={spawnAgent}
        />
      </SheetContent>
    </Sheet>
  );
}
