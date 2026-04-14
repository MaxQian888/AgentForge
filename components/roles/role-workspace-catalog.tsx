"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Store } from "lucide-react";
import { Button } from "@/components/ui/button";
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
    <div className="flex flex-col">
      <div className="sticky top-0 z-10 border-b bg-sidebar px-4 py-3">
        <div className="flex flex-col gap-2">
          <div>
            <p className="text-sm font-semibold">{t("roleLibrary")}</p>
            <p className="text-xs text-muted-foreground">{t("roleLibraryDesc")}</p>
          </div>
          <div className="flex items-center gap-2">
            <Button asChild size="sm" variant="outline">
              <Link href="/marketplace?type=role">
                <Store className="mr-1 size-3" />
                {t("browseMarketplace")}
              </Link>
            </Button>
            <Button size="sm" onClick={onCreateNew}>{t("newRole")}</Button>
          </div>
        </div>
        {error ? <p className="mt-1 text-xs text-destructive">{error}</p> : null}
      </div>

      <div className="flex flex-col gap-2 p-3">
        {loading && roles.length === 0 ? (
          <p className="py-6 text-center text-xs text-muted-foreground">{t("loadingRoles")}</p>
        ) : roles.length === 0 ? (
          <p className="py-6 text-center text-xs text-muted-foreground">{t("emptyLibrary")}</p>
        ) : (
          roles.map((role) => (
            <RoleCard
              key={role.metadata.id}
              role={role}
              skillCatalog={skillCatalog}
              onEdit={() => onEditRole(role)}
              onDelete={() => onDeleteRole(role)}
            />
          ))
        )}
      </div>
    </div>
  );
}
