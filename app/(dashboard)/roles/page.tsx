"use client";

import { useEffect } from "react";
import { RoleWorkspace } from "@/components/roles/role-workspace";
import { useRoleStore, type RoleManifest } from "@/lib/stores/role-store";
import { usePluginStore } from "@/lib/stores/plugin-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function RolesPage() {
  useBreadcrumbs([{ label: "Configuration", href: "/" }, { label: "Roles" }]);
  const {
    roles,
    skillCatalog,
    loading,
    skillCatalogLoading,
    error,
    fetchRoles,
    fetchSkillCatalog,
    fetchRoleReferences,
    createRole,
    updateRole,
    deleteRole,
    previewRole,
    sandboxRole,
  } =
    useRoleStore();
  const plugins = usePluginStore((state) => state.plugins);
  const fetchPlugins = usePluginStore((state) => state.fetchPlugins);

  useEffect(() => {
    fetchRoles();
    fetchSkillCatalog();
    fetchPlugins();
  }, [fetchRoles, fetchSkillCatalog, fetchPlugins]);

  async function handleSubmit(data: Partial<RoleManifest>) {
    await createRole(data);
  }

  return (
    <RoleWorkspace
      roles={roles}
      skillCatalog={skillCatalog}
      availablePlugins={plugins.filter((plugin) => plugin.kind === "ToolPlugin")}
      skillCatalogLoading={skillCatalogLoading}
      loading={loading}
      error={error}
      onLoadRoleReferences={fetchRoleReferences}
      onCreateRole={handleSubmit}
      onUpdateRole={updateRole}
      onDeleteRole={async (role) => {
        try {
          await deleteRole(role.metadata.id);
        } catch {
          // error is set in the store
        }
      }}
      onPreviewRole={previewRole}
      onSandboxRole={sandboxRole}
    />
  );
}
