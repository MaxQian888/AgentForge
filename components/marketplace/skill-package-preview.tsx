"use client";

import { useMemo } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import YAML from "yaml";
import type { SkillPackagePreview } from "@/lib/stores/marketplace-store";

interface Props {
  preview: SkillPackagePreview;
}

function normalizeYaml(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }

  try {
    return YAML.parseDocument(trimmed).toString().trim();
  } catch {
    return trimmed;
  }
}

export function SkillPackagePreviewPane({ preview }: Props) {
  const frontmatterYaml = useMemo(
    () => normalizeYaml(preview.frontmatterYaml),
    [preview.frontmatterYaml],
  );

  return (
    <div className="space-y-4 rounded-lg border border-border/60 p-3">
      <div className="space-y-1">
        <p className="text-xs font-medium">Skill package</p>
        <p className="text-xs text-muted-foreground">
          {preview.canonicalPath}
        </p>
      </div>

      {preview.markdownBody ? (
        <div className="prose prose-sm max-w-none prose-headings:mb-2 prose-headings:mt-4 prose-p:my-2 prose-li:my-1">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>
            {preview.markdownBody}
          </ReactMarkdown>
        </div>
      ) : null}

      {frontmatterYaml ? (
        <div className="space-y-2">
          <p className="text-xs font-medium">Frontmatter YAML</p>
          <pre className="overflow-x-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
            {frontmatterYaml}
          </pre>
        </div>
      ) : null}

      {preview.agentConfigs.length > 0 ? (
        <div className="space-y-3">
          <p className="text-xs font-medium">Agent YAML</p>
          {preview.agentConfigs.map((config) => (
            <div key={config.path} className="space-y-2 rounded-md border border-border/60 p-3">
              <div className="space-y-1">
                <p className="text-xs font-medium">{config.path}</p>
                {config.displayName ? (
                  <p className="text-xs text-muted-foreground">{config.displayName}</p>
                ) : null}
              </div>
              <pre className="overflow-x-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
                {normalizeYaml(config.yaml)}
              </pre>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}
