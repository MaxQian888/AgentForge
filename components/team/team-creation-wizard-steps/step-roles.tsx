"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { Check } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useRoleStore } from "@/lib/stores/role-store";
import { cn } from "@/lib/utils";

interface StepRolesProps {
  selectedRoleIds: string[];
  onChange: (roleIds: string[]) => void;
}

export function StepRoles({ selectedRoleIds, onChange }: StepRolesProps) {
  const t = useTranslations("teams");
  const { roles, loading, fetchRoles } = useRoleStore();

  useEffect(() => {
    if (roles.length === 0) {
      void fetchRoles();
    }
  }, [roles.length, fetchRoles]);

  const toggleRole = (roleId: string) => {
    if (selectedRoleIds.includes(roleId)) {
      onChange(selectedRoleIds.filter((id) => id !== roleId));
    } else {
      onChange([...selectedRoleIds, roleId]);
    }
  };

  if (loading) {
    return (
      <p className="text-sm text-muted-foreground">{t("wizard.loadingRoles")}</p>
    );
  }

  if (roles.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">{t("wizard.noRoles")}</p>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">{t("wizard.rolesHint")}</p>
      <div className="grid gap-3 sm:grid-cols-2">
        {roles.map((role) => {
          const id = role.metadata.id;
          const selected = selectedRoleIds.includes(id);
          const skillCount =
            (role.capabilities.skills?.length ?? 0) +
            (role.capabilities.tools?.length ?? 0);

          return (
            <Card
              key={id}
              className={cn(
                "cursor-pointer transition-colors",
                selected && "border-primary bg-primary/5"
              )}
              onClick={() => toggleRole(id)}
            >
              <CardHeader className="flex flex-row items-center gap-2 p-3">
                <div
                  className={cn(
                    "flex size-5 shrink-0 items-center justify-center rounded border",
                    selected
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-muted-foreground/30"
                  )}
                >
                  {selected && <Check className="size-3" />}
                </div>
                <CardTitle className="text-sm font-medium">
                  {role.metadata.name}
                </CardTitle>
              </CardHeader>
              <CardContent className="px-3 pb-3 pt-0">
                <p className="text-xs text-muted-foreground line-clamp-2">
                  {role.metadata.description || t("wizard.noDescription")}
                </p>
                {skillCount > 0 && (
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t("wizard.skillCount", { count: skillCount })}
                  </p>
                )}
              </CardContent>
            </Card>
          );
        })}
      </div>
    </div>
  );
}
