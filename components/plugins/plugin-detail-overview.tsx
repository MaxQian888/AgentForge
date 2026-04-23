"use client";

import { Badge } from "@/components/ui/badge";
import { useTranslations } from "next-intl";
import { PluginDetailSection } from "@/components/plugins/plugin-detail-section";
import { PluginTrustBadge } from "@/components/plugins/plugin-trust-badge";
import type { PluginRecord } from "@/lib/stores/plugin-store";

interface PluginDetailOverviewProps {
  plugin: PluginRecord;
}

function renderPermissions(plugin: PluginRecord, t: ReturnType<typeof useTranslations>): string {
  const permissions: string[] = [];

  if (plugin.permissions.network?.required) {
    permissions.push(
      `${t("detailOverview.network")}${
        plugin.permissions.network.domains?.length
          ? ` (${plugin.permissions.network.domains.join(", ")})`
          : ""
      }`,
    );
  }

  if (plugin.permissions.filesystem?.required) {
    permissions.push(
      `${t("detailOverview.filesystem")}${
        plugin.permissions.filesystem.allowed_paths?.length
          ? ` (${plugin.permissions.filesystem.allowed_paths.join(", ")})`
          : ""
      }`,
    );
  }

  return permissions.length > 0 ? permissions.join(" · ") : t("detailOverview.noPermissions");
}

export function PluginDetailOverview({ plugin }: PluginDetailOverviewProps) {
  const t = useTranslations("plugins");
  const roleConsumers = plugin.roleConsumers ?? [];
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-lg font-semibold">
            {plugin.metadata.name}
          </h3>
          <p className="text-sm text-muted-foreground">
            {plugin.metadata.description || t("detailOverview.noDescription")}
          </p>
        </div>
        <Badge variant="secondary">
          {plugin.lifecycle_state}
        </Badge>
      </div>

      <PluginTrustBadge source={plugin.source} />

      <div className="grid gap-3 text-sm">
        <PluginDetailSection title={t("detailOverview.runtimeHost")}>
          {plugin.runtime_host ?? t("detailOverview.notExecutable")}
        </PluginDetailSection>

        <PluginDetailSection title={t("detailOverview.runtimeDeclaration")}>
          {plugin.spec.runtime}
        </PluginDetailSection>

        <PluginDetailSection title={t("detailOverview.permissions")}>
          {renderPermissions(plugin, t)}
        </PluginDetailSection>

        <PluginDetailSection title={t("detailOverview.resolvedSourcePath")}>
          {plugin.resolved_source_path ??
            plugin.source.path ??
            t("detailOverview.noResolvedPath")}
        </PluginDetailSection>

        {plugin.source.registry || plugin.source.entry || plugin.source.version ? (
          <PluginDetailSection title={t("detailOverview.remoteProvenance")}>
            <div className="grid gap-1">
              {plugin.source.registry ? (
                <p>{t("detailOverview.registry", { registry: plugin.source.registry })}</p>
              ) : null}
              {plugin.source.entry ? (
                <p>{t("detailOverview.entry", { entry: plugin.source.entry })}</p>
              ) : null}
              {plugin.source.version ? (
                <p>{t("detailOverview.requestedVersion", { version: plugin.source.version })}</p>
              ) : null}
            </div>
          </PluginDetailSection>
        ) : null}

        {plugin.source.type === "marketplace" || plugin.source.catalog || plugin.source.ref ? (
          <PluginDetailSection title={t("detailOverview.marketplaceProvenance")}>
            <div className="grid gap-1">
              {plugin.source.catalog ? (
                <p>{t("detailOverview.marketplaceItem", { item: plugin.source.catalog })}</p>
              ) : null}
              {plugin.source.ref ? (
                <p>{t("detailOverview.selectedVersion", { version: plugin.source.ref })}</p>
              ) : null}
              <p>{t("detailOverview.sourceType", { type: plugin.source.type })}</p>
              {plugin.source.catalog ? (
                <p>
                  <a
                    href={`/marketplace?item=${encodeURIComponent(plugin.source.catalog)}`}
                    className="underline"
                  >
                    {t("detailOverview.openInMarketplace")}
                  </a>
                </p>
              ) : null}
            </div>
          </PluginDetailSection>
        ) : null}

        <PluginDetailSection title={t("detailOverview.runtimeMetadata")}>
          {t("detailOverview.abi", {
            version: plugin.runtime_metadata?.abi_version ?? "n/a",
            compatible: plugin.runtime_metadata?.compatible
              ? t("detailOverview.yes")
              : t("detailOverview.no"),
          })}
        </PluginDetailSection>

        <PluginDetailSection title={t("detailOverview.restartCount")}>
          {plugin.restart_count}
        </PluginDetailSection>

        <PluginDetailSection title={t("detailOverview.lastHealth")}>
          {plugin.last_health_at ?? t("detailOverview.notRecorded")}
        </PluginDetailSection>

        <PluginDetailSection title={t("detailOverview.lastError")}>
          {plugin.last_error || t("detailOverview.noRecentErrors")}
        </PluginDetailSection>

        {plugin.builtIn ? (
          <PluginDetailSection title={t("detailOverview.builtInReadiness")}>
            <div className="grid gap-1">
              <p>{plugin.builtIn.readinessStatus ?? plugin.builtIn.availabilityStatus ?? t("detailOverview.unknown")}</p>
              {plugin.builtIn.readinessMessage ?? plugin.builtIn.availabilityMessage ? (
                <p>{plugin.builtIn.readinessMessage ?? plugin.builtIn.availabilityMessage}</p>
              ) : null}
              {plugin.builtIn.nextStep ? (
                <p>{t("detailOverview.nextStep", { step: plugin.builtIn.nextStep })}</p>
              ) : null}
              {plugin.builtIn.starterFamily ? (
                <p>{t("detailOverview.starterFamily", { family: plugin.builtIn.starterFamily })}</p>
              ) : null}
              {plugin.builtIn.coreFlows?.length ? (
                <p>{t("detailOverview.coreFlows", { flows: plugin.builtIn.coreFlows.join(", ") })}</p>
              ) : null}
              {plugin.builtIn.dependencyRefs?.length ? (
                <p>{t("detailOverview.dependencies", { deps: plugin.builtIn.dependencyRefs.join(", ") })}</p>
              ) : null}
              {plugin.builtIn.workspaceRefs?.length ? (
                <p>{t("detailOverview.workspaces", { spaces: plugin.builtIn.workspaceRefs.join(", ") })}</p>
              ) : null}
              {plugin.builtIn.missingPrerequisites?.length ? (
                <p>
                  {t("detailOverview.missingPrerequisites", { items: plugin.builtIn.missingPrerequisites.join(", ") })}
                </p>
              ) : null}
              {plugin.builtIn.missingConfiguration?.length ? (
                <p>
                  {t("detailOverview.missingConfiguration", { items: plugin.builtIn.missingConfiguration.join(", ") })}
                </p>
              ) : null}
              {plugin.builtIn.installable === false && plugin.builtIn.installBlockedReason ? (
                <p>{t("detailOverview.installBlocked", { reason: plugin.builtIn.installBlockedReason })}</p>
              ) : null}
              {plugin.builtIn.docsRef ? <p>{t("detailOverview.docs", { ref: plugin.builtIn.docsRef })}</p> : null}
            </div>
          </PluginDetailSection>
        ) : null}

        {plugin.source.release ? (
          <PluginDetailSection title={t("detailOverview.releaseInfo")}>
            <div className="grid gap-1">
              {plugin.source.release.version ? (
                <p>{t("detailOverview.version", { version: plugin.source.release.version })}</p>
              ) : null}
              {plugin.source.release.channel ? (
                <p>{t("detailOverview.channel", { channel: plugin.source.release.channel })}</p>
              ) : null}
              {plugin.source.release.publishedAt ? (
                <p>{t("detailOverview.published", { date: plugin.source.release.publishedAt })}</p>
              ) : null}
              {plugin.source.release.notesUrl ? (
                <p>
                  {t("detailOverview.notes")}{" "}
                  <a
                    href={plugin.source.release.notesUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline"
                  >
                    {plugin.source.release.notesUrl}
                  </a>
                </p>
              ) : null}
              {plugin.source.release.availableVersion ? (
                <p className="font-medium text-foreground">
                  {t("detailOverview.updateAvailable", {
                    version: plugin.source.release.availableVersion,
                  })}
                </p>
              ) : null}
            </div>
          </PluginDetailSection>
        ) : null}

        {roleConsumers.length > 0 ? (
          <PluginDetailSection title={t("detailOverview.roleConsumers")}>
            <div className="grid gap-1">
              {roleConsumers.map((consumer) => (
                <div key={`${consumer.roleId}:${consumer.referenceType}`} className="grid gap-0.5">
                  <p>{consumer.roleName ? `${consumer.roleName} (${consumer.roleId})` : consumer.roleId}</p>
                  <p>{consumer.referenceType} · {consumer.status}</p>
                </div>
              ))}
              <p>
                <a href="/roles" className="underline">
                  {t("detailOverview.openRolesWorkspace")}
                </a>
              </p>
            </div>
          </PluginDetailSection>
        ) : null}
      </div>
    </div>
  );
}
