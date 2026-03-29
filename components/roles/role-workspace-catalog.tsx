"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { RoleCard } from "./role-card";
import type { RoleManifest, RoleSkillCatalogEntry } from "@/lib/stores/role-store";

interface RoleWorkspaceCatalogProps {
  roles: RoleManifest[];
  skillCatalog: RoleSkillCatalogEntry[];
  loading: boolean;
  error: string | null;
  onCreateNew: () => void;
  onEditRole: (role: RoleManifest) => void;
  onDeleteRole: (role: RoleManifest) => void;
}

export function RoleWorkspaceCatalog({
  roles,
  skillCatalog,
  loading,
  error,
  onCreateNew,
  onEditRole,
  onDeleteRole,
}: RoleWorkspaceCatalogProps) {
  const t = useTranslations("roles");
  return (
    <section className="flex flex-col gap-4">
      <Card>
        <CardHeader className="gap-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <CardTitle>{t("roleLibrary")}</CardTitle>
              <CardDescription>
                {t("roleLibraryDesc")}
              </CardDescription>
            </div>
            <Button onClick={onCreateNew}>{t("newRole")}</Button>
          </div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardHeader>
      </Card>

      {loading && roles.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("loadingRoles")}</p>
      ) : roles.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("roleLibrary")}</CardTitle>
            <CardDescription>
              {t("emptyLibrary")}
            </CardDescription>
          </CardHeader>
        </Card>
      ) : (
        <div className="grid gap-4">
          {roles.map((role) => (
            <RoleCard
              key={role.metadata.id}
              role={role}
              skillCatalog={skillCatalog}
              onEdit={() => onEditRole(role)}
              onDelete={() => onDeleteRole(role)}
            />
          ))}
        </div>
      )}
    </section>
  );
}
