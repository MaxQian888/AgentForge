"use client";

import { useEffect } from "react";
import { SkillsWorkspace } from "@/components/skills/skills-workspace";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { useSkillsStore } from "@/lib/stores/skills-store";

export default function SkillsPage() {
  useBreadcrumbs([
    { label: "Configuration", href: "/" },
    { label: "Skills" },
  ]);

  const {
    items,
    selectedSkill,
    loading,
    detailLoading,
    actionLoading,
    error,
    filters,
    fetchSkills,
    selectSkill,
    verifySkills,
    syncMirrors,
    setFilters,
  } = useSkillsStore();

  useEffect(() => {
    void fetchSkills();
  }, [fetchSkills]);

  useEffect(() => {
    if (!selectedSkill && items.length > 0) {
      void selectSkill(items[0].id);
    }
  }, [items, selectSkill, selectedSkill]);

  return (
    <SkillsWorkspace
      items={items}
      selectedSkill={selectedSkill}
      loading={loading}
      detailLoading={detailLoading}
      actionLoading={actionLoading}
      error={error}
      filters={filters}
      onSelectSkill={selectSkill}
      onVerifyInternal={async () => {
        await verifySkills();
      }}
      onVerifyBuiltIns={async () => {
        await verifySkills(["built-in-runtime"]);
      }}
      onSyncMirrors={async () => {
        await syncMirrors(selectedSkill ? [selectedSkill.id] : undefined);
      }}
      onSetFilters={setFilters}
    />
  );
}
