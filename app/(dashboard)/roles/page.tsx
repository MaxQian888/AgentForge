"use client";

import { useEffect } from "react";
import { RoleWorkspace } from "@/components/roles/role-workspace";
import { useRoleStore, type RoleManifest } from "@/lib/stores/role-store";

export default function RolesPage() {
  const { roles, loading, error, fetchRoles, createRole, updateRole, deleteRole } =
    useRoleStore();

  useEffect(() => {
    fetchRoles();
  }, [fetchRoles]);

  async function handleSubmit(data: Partial<RoleManifest>) {
    await createRole(data);
  }

  return (
    <RoleWorkspace
      roles={roles}
      loading={loading}
      error={error}
      onCreateRole={handleSubmit}
      onUpdateRole={updateRole}
      onDeleteRole={async (role) => {
        try {
          await deleteRole(role.metadata.id);
        } catch {
          // error is set in the store
        }
      }}
    />
  );
}
