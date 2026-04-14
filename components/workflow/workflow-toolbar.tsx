"use client";

import { useState } from "react";
import {
  Play,
  GitBranch,
  Bot,
  Bell,
  ArrowRightLeft,
  Lock,
  Split,
  Merge,
  Save,
  PlayCircle,
  GripVertical,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

export interface NodeTypeItem {
  type: string;
  label: string;
  icon: React.ElementType;
  color: string;
}

export const NODE_TYPE_PALETTE: NodeTypeItem[] = [
  { type: "trigger", label: "Trigger", icon: Play, color: "text-green-600" },
  {
    type: "condition",
    label: "Condition",
    icon: GitBranch,
    color: "text-amber-600",
  },
  {
    type: "agent_dispatch",
    label: "Agent Dispatch",
    icon: Bot,
    color: "text-blue-600",
  },
  {
    type: "notification",
    label: "Notification",
    icon: Bell,
    color: "text-yellow-600",
  },
  {
    type: "status_transition",
    label: "Status Transition",
    icon: ArrowRightLeft,
    color: "text-purple-600",
  },
  { type: "gate", label: "Gate", icon: Lock, color: "text-red-600" },
  {
    type: "parallel_split",
    label: "Parallel Split",
    icon: Split,
    color: "text-orange-600",
  },
  {
    type: "parallel_join",
    label: "Parallel Join",
    icon: Merge,
    color: "text-orange-600",
  },
];

interface WorkflowToolbarProps {
  name: string;
  description: string;
  status: string;
  saving: boolean;
  onNameChange: (name: string) => void;
  onDescriptionChange: (description: string) => void;
  onSave: () => void;
  onExecute: () => void;
  onAddNode: (type: string) => void;
}

export function WorkflowToolbar({
  name,
  description,
  status,
  saving,
  onNameChange,
  onDescriptionChange,
  onSave,
  onExecute,
  onAddNode,
}: WorkflowToolbarProps) {
  const [showPalette, setShowPalette] = useState(true);

  return (
    <div className="flex flex-col gap-3 p-3 border-b bg-background">
      <div className="flex items-center gap-3">
        <Input
          value={name}
          onChange={(e) => onNameChange(e.target.value)}
          placeholder="Workflow name"
          className="max-w-xs font-medium"
        />
        <Input
          value={description}
          onChange={(e) => onDescriptionChange(e.target.value)}
          placeholder="Description"
          className="max-w-sm text-sm"
        />
        <Badge variant={status === "active" ? "default" : "secondary"}>
          {status}
        </Badge>
        <div className="ml-auto flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={onSave}
            disabled={saving}
          >
            <Save className="h-4 w-4 mr-1" />
            {saving ? "Saving..." : "Save"}
          </Button>
          <Button
            size="sm"
            onClick={onExecute}
            disabled={status !== "active"}
          >
            <PlayCircle className="h-4 w-4 mr-1" />
            Execute
          </Button>
        </div>
      </div>

      <div className="flex items-center gap-1">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setShowPalette(!showPalette)}
          className="text-xs text-muted-foreground"
        >
          <GripVertical className="h-3 w-3 mr-1" />
          {showPalette ? "Hide palette" : "Show palette"}
        </Button>
        {showPalette && (
          <div className="flex items-center gap-1 flex-wrap">
            {NODE_TYPE_PALETTE.map((item) => {
              const Icon = item.icon;
              return (
                <Tooltip key={item.type}>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-8 gap-1.5 text-xs"
                      onClick={() => onAddNode(item.type)}
                      draggable
                      onDragStart={(e) => {
                        e.dataTransfer.setData(
                          "application/workflow-node-type",
                          item.type
                        );
                        e.dataTransfer.effectAllowed = "move";
                      }}
                    >
                      <Icon className={cn("h-3.5 w-3.5", item.color)} />
                      {item.label}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    Drag or click to add a {item.label} node
                  </TooltipContent>
                </Tooltip>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
