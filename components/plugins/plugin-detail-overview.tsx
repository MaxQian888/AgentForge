"use client";

import { Badge } from "@/components/ui/badge";
import { PluginDetailSection } from "@/components/plugins/plugin-detail-sidebar";
import { PluginTrustBadge } from "@/components/plugins/plugin-trust-badge";
import type { PluginRecord } from "@/lib/stores/plugin-store";

interface PluginDetailOverviewProps {
  plugin: PluginRecord;
}

function renderPermissions(plugin: PluginRecord): string {
  const permissions: string[] = [];

  if (plugin.permissions.network?.required) {
    permissions.push(
      `Network${
        plugin.permissions.network.domains?.length
          ? ` (${plugin.permissions.network.domains.join(", ")})`
          : ""
      }`,
    );
  }

  if (plugin.permissions.filesystem?.required) {
    permissions.push(
      `Filesystem${
        plugin.permissions.filesystem.allowed_paths?.length
          ? ` (${plugin.permissions.filesystem.allowed_paths.join(", ")})`
          : ""
      }`,
    );
  }

  return permissions.length > 0 ? permissions.join(" · ") : "No declared permissions";
}

export function PluginDetailOverview({ plugin }: PluginDetailOverviewProps) {
  const roleConsumers = plugin.roleConsumers ?? [];
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-lg font-semibold">
            {plugin.metadata.name}
          </h3>
          <p className="text-sm text-muted-foreground">
            {plugin.metadata.description || "No plugin description provided."}
          </p>
        </div>
        <Badge variant="secondary">
          {plugin.lifecycle_state}
        </Badge>
      </div>

      <PluginTrustBadge source={plugin.source} />

      <div className="grid gap-3 text-sm">
        <PluginDetailSection title="Runtime host">
          {plugin.runtime_host ?? "Not executable"}
        </PluginDetailSection>

        <PluginDetailSection title="Runtime declaration">
          {plugin.spec.runtime}
        </PluginDetailSection>

        <PluginDetailSection title="Permissions">
          {renderPermissions(plugin)}
        </PluginDetailSection>

        <PluginDetailSection title="Resolved source path">
          {plugin.resolved_source_path ??
            plugin.source.path ??
            "No resolved source path"}
        </PluginDetailSection>

        {plugin.source.registry || plugin.source.entry || plugin.source.version ? (
          <PluginDetailSection title="Remote source provenance">
            <div className="grid gap-1">
              {plugin.source.registry ? (
                <p>Registry: {plugin.source.registry}</p>
              ) : null}
              {plugin.source.entry ? (
                <p>Entry: {plugin.source.entry}</p>
              ) : null}
              {plugin.source.version ? (
                <p>Requested version: {plugin.source.version}</p>
              ) : null}
            </div>
          </PluginDetailSection>
        ) : null}

        {plugin.source.type === "marketplace" || plugin.source.catalog || plugin.source.ref ? (
          <PluginDetailSection title="Marketplace provenance">
            <div className="grid gap-1">
              {plugin.source.catalog ? (
                <p>Marketplace item: {plugin.source.catalog}</p>
              ) : null}
              {plugin.source.ref ? (
                <p>Selected version: {plugin.source.ref}</p>
              ) : null}
              <p>Source type: {plugin.source.type}</p>
              {plugin.source.catalog ? (
                <p>
                  <a
                    href={`/marketplace?item=${encodeURIComponent(plugin.source.catalog)}`}
                    className="underline"
                  >
                    Open in marketplace
                  </a>
                </p>
              ) : null}
            </div>
          </PluginDetailSection>
        ) : null}

        <PluginDetailSection title="Runtime metadata">
          ABI {plugin.runtime_metadata?.abi_version ?? "n/a"} · Compatible{" "}
          {plugin.runtime_metadata?.compatible ? "yes" : "no"}
        </PluginDetailSection>

        <PluginDetailSection title="Restart count">
          {plugin.restart_count}
        </PluginDetailSection>

        <PluginDetailSection title="Last health">
          {plugin.last_health_at ?? "Not recorded yet"}
        </PluginDetailSection>

        <PluginDetailSection title="Last error">
          {plugin.last_error || "No recent runtime errors"}
        </PluginDetailSection>

        {plugin.builtIn ? (
          <PluginDetailSection title="Built-in readiness">
            <div className="grid gap-1">
              <p>{plugin.builtIn.readinessStatus ?? plugin.builtIn.availabilityStatus ?? "unknown"}</p>
              {plugin.builtIn.readinessMessage ?? plugin.builtIn.availabilityMessage ? (
                <p>{plugin.builtIn.readinessMessage ?? plugin.builtIn.availabilityMessage}</p>
              ) : null}
              {plugin.builtIn.nextStep ? (
                <p>Next step: {plugin.builtIn.nextStep}</p>
              ) : null}
              {plugin.builtIn.missingPrerequisites?.length ? (
                <p>
                  Missing prerequisites: {plugin.builtIn.missingPrerequisites.join(", ")}
                </p>
              ) : null}
              {plugin.builtIn.missingConfiguration?.length ? (
                <p>
                  Missing configuration: {plugin.builtIn.missingConfiguration.join(", ")}
                </p>
              ) : null}
              {plugin.builtIn.installable === false && plugin.builtIn.installBlockedReason ? (
                <p>Install blocked: {plugin.builtIn.installBlockedReason}</p>
              ) : null}
              {plugin.builtIn.docsRef ? <p>Docs: {plugin.builtIn.docsRef}</p> : null}
            </div>
          </PluginDetailSection>
        ) : null}

        {plugin.source.release ? (
          <PluginDetailSection title="Release info">
            <div className="grid gap-1">
              {plugin.source.release.version ? (
                <p>Version: {plugin.source.release.version}</p>
              ) : null}
              {plugin.source.release.channel ? (
                <p>Channel: {plugin.source.release.channel}</p>
              ) : null}
              {plugin.source.release.publishedAt ? (
                <p>Published: {plugin.source.release.publishedAt}</p>
              ) : null}
              {plugin.source.release.notesUrl ? (
                <p>
                  Notes:{" "}
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
                  Update available: v{plugin.source.release.availableVersion}
                </p>
              ) : null}
            </div>
          </PluginDetailSection>
        ) : null}

        {roleConsumers.length > 0 ? (
          <PluginDetailSection title="Role consumers">
            <div className="grid gap-1">
              {roleConsumers.map((consumer) => (
                <div key={`${consumer.roleId}:${consumer.referenceType}`} className="grid gap-0.5">
                  <p>{consumer.roleName ? `${consumer.roleName} (${consumer.roleId})` : consumer.roleId}</p>
                  <p>{consumer.referenceType} · {consumer.status}</p>
                </div>
              ))}
              <p>
                <a href="/roles" className="underline">
                  Open roles workspace
                </a>
              </p>
            </div>
          </PluginDetailSection>
        ) : null}
      </div>
    </div>
  );
}
