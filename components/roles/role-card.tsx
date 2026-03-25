"use client";

import { Shield, Pencil, Trash2 } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { RoleManifest } from "@/lib/stores/role-store";

interface RoleCardProps {
  role: RoleManifest;
  onEdit: () => void;
  onDelete: () => void;
}

export function RoleCard({ role, onEdit, onDelete }: RoleCardProps) {
  const allowedTools = role.capabilities.allowedTools ?? role.capabilities.tools ?? [];
  const skills = role.capabilities.skills ?? [];
  const autoLoadCount = skills.filter((skill) => skill.autoLoad).length;
  const onDemandCount = skills.length - autoLoadCount;
  const hasPathRestrictions =
    role.security.allowedPaths.length > 0 || role.security.deniedPaths.length > 0;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center gap-3 pb-2">
        <Shield className="size-5 text-primary" />
        <div className="space-y-1">
          <CardTitle className="text-base">{role.metadata.name}</CardTitle>
          <p className="text-xs text-muted-foreground">
            {role.identity.role || role.metadata.id}
          </p>
        </div>
      </CardHeader>
      <CardContent>
        <p className="mb-3 text-sm text-muted-foreground">
          {role.metadata.description}
        </p>
        <div className="mb-2 flex flex-wrap gap-1.5">
          <Badge variant="outline" className="text-xs">
            v{role.metadata.version || "1.0.0"}
          </Badge>
          {(role.metadata.tags ?? []).map((tag) => (
            <Badge key={tag} variant="secondary" className="text-xs">
              {tag}
            </Badge>
          ))}
          {role.extends ? (
            <Badge variant="outline" className="text-xs">
              Extends {role.extends}
            </Badge>
          ) : null}
        </div>
        <div className="mb-4 grid gap-2 text-xs text-muted-foreground">
          {allowedTools.length > 0 ? (
            <p>Tools: {allowedTools.join(", ")}</p>
          ) : (
            <p>Tools: inherits default platform capabilities</p>
          )}
          {role.capabilities.maxBudgetUsd != null ? (
            <p>Max budget: ${role.capabilities.maxBudgetUsd.toFixed(2)}</p>
          ) : null}
          {role.capabilities.maxTurns != null ? (
            <p>Max turns: {role.capabilities.maxTurns}</p>
          ) : null}
          {skills.length > 0 ? (
            <>
              <p>Skills: {autoLoadCount} auto / {onDemandCount} on-demand</p>
              <p>Key skills: {skills.slice(0, 3).map((skill) => skill.path).join(", ")}</p>
            </>
          ) : null}
          {role.security.requireReview ? (
            <p>Review gate: required before execution</p>
          ) : null}
          {hasPathRestrictions ? (
            <p>
              Path policy: {role.security.allowedPaths.length} allow /
              {" "}
              {role.security.deniedPaths.length} deny
            </p>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={onEdit}
            aria-label={`Edit ${role.metadata.name}`}
          >
            <Pencil className="mr-1 size-3.5" />
            Edit
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={onDelete}
            aria-label={`Delete ${role.metadata.name}`}
          >
            <Trash2 className="mr-1 size-3.5" />
            Delete
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
