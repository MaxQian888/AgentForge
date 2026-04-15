"use client";

import { useTranslations } from "next-intl";
import { Shield, Pencil, Trash2, Wrench, BookOpen, AlertCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  buildRoleCapabilitySourceFromManifest,
  resolveRoleSkillReferences,
} from "@/lib/roles/role-management";
import type { RoleManifest, RoleSkillCatalogEntry } from "@/lib/stores/role-store";

interface RoleCardProps {
  role: RoleManifest;
  skillCatalog?: RoleSkillCatalogEntry[];
  onEdit: () => void;
  onDelete: () => void;
}

export function RoleCard({ role, skillCatalog = [], onEdit, onDelete }: RoleCardProps) {
  const t = useTranslations("roles");
  const allowedTools = role.capabilities.allowedTools ?? role.capabilities.tools ?? [];
  const skills = role.capabilities.skills ?? [];
  const autoLoadCount = skills.filter((skill) => skill.autoLoad).length;
  const onDemandCount = skills.length - autoLoadCount;
  const skillResolution = resolveRoleSkillReferences({
    skills,
    catalog: skillCatalog,
    roleCapabilities: buildRoleCapabilitySourceFromManifest(role),
  });
  const resolvedCount = skillResolution.filter((skill) => skill.status === "resolved").length;
  const unresolvedCount = skillResolution.length - resolvedCount;
  const blockingCount = skillResolution.filter(
    (skill) => skill.compatibilityStatus === "blocking",
  ).length;
  const warningCount = skillResolution.filter(
    (skill) => skill.compatibilityStatus === "warning",
  ).length;
  const pluginConsumerCount = role.pluginConsumers?.length ?? 0;

  return (
    <div className="relative overflow-hidden rounded-lg border bg-card text-card-foreground shadow-sm transition-colors hover:bg-accent/30">
      {/* Left accent stripe */}
      <div className="absolute inset-y-0 left-0 w-0.5 bg-primary/40" />

      {/* Header */}
      <div className="flex items-start gap-2.5 pl-5 pr-3 pt-3 pb-2">
        <Shield className="mt-0.5 size-3.5 shrink-0 text-primary/70" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-xs font-semibold leading-tight">{role.metadata.name}</p>
          <p className="truncate text-xs text-muted-foreground">
            {role.identity.role || role.metadata.id}
          </p>
        </div>
        <Badge variant="outline" className="shrink-0 text-xs">
          v{role.metadata.version || "1.0.0"}
        </Badge>
      </div>

      {/* Description */}
      {role.metadata.description ? (
        <p className="line-clamp-2 pl-5 pr-3 pb-1.5 text-xs text-muted-foreground">
          {role.metadata.description}
        </p>
      ) : null}

      {/* Tags */}
      {((role.metadata.tags ?? []).length > 0 || role.extends) ? (
        <div className="flex flex-wrap gap-1 pl-5 pr-3 pb-1.5">
          {(role.metadata.tags ?? []).map((tag) => (
            <Badge key={tag} variant="secondary" className="text-xs">{tag}</Badge>
          ))}
          {role.extends ? (
            <Badge variant="outline" className="text-xs">
              {t("card.extends", { name: role.extends })}
            </Badge>
          ) : null}
        </div>
      ) : null}

      {/* Icon-group metadata */}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 pl-5 pr-3 pb-2 text-xs text-muted-foreground">
        {allowedTools.length > 0 ? (
          <span className="flex items-center gap-1">
            <Wrench className="size-3" />
            {allowedTools.length}
          </span>
        ) : null}
        {skills.length > 0 ? (
          <span className="flex items-center gap-1">
            <BookOpen className="size-3" />
            {t("card.skillsAutoOnDemand", { auto: autoLoadCount, onDemand: onDemandCount })}
          </span>
        ) : null}
        {role.security.requireReview ? (
          <span className="flex items-center gap-1 text-amber-600 dark:text-amber-400">
            <AlertCircle className="size-3" />
            {t("card.reviewGate")}
          </span>
        ) : null}
        {unresolvedCount > 0 ? (
          <span className="flex items-center gap-1 text-destructive">
            <AlertCircle className="size-3" />
            {unresolvedCount} unresolved
          </span>
        ) : null}
        {blockingCount > 0 ? (
          <span className="flex items-center gap-1 text-destructive">
            <AlertCircle className="size-3" />
            {blockingCount} blocking
          </span>
        ) : null}
        {warningCount > 0 ? (
          <span className="flex items-center gap-1 text-amber-600 dark:text-amber-400">
            <AlertCircle className="size-3" />
            {warningCount} warning
          </span>
        ) : null}
        {pluginConsumerCount > 0 ? (
          <span className="flex items-center gap-1 text-amber-600 dark:text-amber-400">
            <AlertCircle className="size-3" />
            {pluginConsumerCount} plugin consumer
          </span>
        ) : null}
      </div>

      {/* Actions */}
      <div className="flex items-center gap-1 border-t pl-4 py-1.5 pr-2">
        <Button
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-xs"
          onClick={onEdit}
          aria-label={`Edit ${role.metadata.name}`}
        >
          <Pencil className="mr-1 size-3" />
          {t("card.edit")}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-xs text-muted-foreground"
          onClick={onDelete}
          aria-label={`Delete ${role.metadata.name}`}
        >
          <Trash2 className="mr-1 size-3" />
          {t("card.delete")}
        </Button>
      </div>
    </div>
  );
}
