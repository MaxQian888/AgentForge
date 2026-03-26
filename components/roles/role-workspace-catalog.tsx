"use client";

import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { RoleCard } from "./role-card";
import type { RoleManifest } from "@/lib/stores/role-store";

interface RoleWorkspaceCatalogProps {
  roles: RoleManifest[];
  loading: boolean;
  error: string | null;
  onCreateNew: () => void;
  onEditRole: (role: RoleManifest) => void;
  onDeleteRole: (role: RoleManifest) => void;
}

export function RoleWorkspaceCatalog({
  roles,
  loading,
  error,
  onCreateNew,
  onEditRole,
  onDeleteRole,
}: RoleWorkspaceCatalogProps) {
  return (
    <section className="flex flex-col gap-4">
      <Card>
        <CardHeader className="gap-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-1">
              <CardTitle>Role Library</CardTitle>
              <CardDescription>
                Compare reusable manifests before opening the authoring flow.
              </CardDescription>
            </div>
            <Button onClick={onCreateNew}>New Role</Button>
          </div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardHeader>
      </Card>

      {loading && roles.length === 0 ? (
        <p className="text-sm text-muted-foreground">Loading roles...</p>
      ) : roles.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Role Library</CardTitle>
            <CardDescription>
              Create the first role to start shaping your engineering roster.
            </CardDescription>
          </CardHeader>
        </Card>
      ) : (
        <div className="grid gap-4">
          {roles.map((role) => (
            <RoleCard
              key={role.metadata.id}
              role={role}
              onEdit={() => onEditRole(role)}
              onDelete={() => onDeleteRole(role)}
            />
          ))}
        </div>
      )}
    </section>
  );
}
