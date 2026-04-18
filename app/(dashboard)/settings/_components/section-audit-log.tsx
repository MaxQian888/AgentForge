"use client";

import { useTranslations } from "next-intl";
import { AuditLogPanel } from "@/components/project/audit-log-panel";
import { useProjectRole } from "@/hooks/use-project-role";

interface SectionAuditLogProps {
  projectId: string;
}

// SectionAuditLog gates the panel on the server-issued `audit.read` action
// from the per-project permissions endpoint. Editors and viewers see the
// forbidden message even if they navigate directly via URL — the backend
// would 403 anyway, this just avoids a confusing empty state.
export function SectionAuditLog({ projectId }: SectionAuditLogProps) {
  const t = useTranslations("audit");
  const { can, loading } = useProjectRole(projectId);

  if (loading) {
    return <div className="text-sm text-muted-foreground">…</div>;
  }
  if (!can("audit.read")) {
    return (
      <div className="rounded-lg border border-muted-foreground/20 bg-muted/40 p-6 text-sm text-muted-foreground">
        {t("errors.forbidden")}
      </div>
    );
  }
  return <AuditLogPanel projectId={projectId} />;
}
