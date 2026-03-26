"use client";

import Link from "next/link";
import { Bot, ClipboardCheck, ListTodo } from "lucide-react";
import { defaultProps } from "@blocknote/core";
import { BlockContentWrapper, createReactBlockSpec } from "@blocknote/react";

const entityCardConfig = {
  type: "entityCard",
  propSchema: {
    textAlignment: defaultProps.textAlignment,
    entityType: { default: "task" },
    entityId: { default: "" },
    label: { default: "Linked entity" },
  },
  content: "none" as const,
};

function resolveEntityIcon(entityType: string) {
  switch (entityType) {
    case "agent":
      return Bot;
    case "review":
      return ClipboardCheck;
    default:
      return ListTodo;
  }
}

export const createEntityCardBlock = createReactBlockSpec(
  entityCardConfig as never,
  {
    render: ({ block }) => {
      const props = (block as {
        props?: { entityType?: string; entityId?: string; label?: string };
      }).props;
      const entityType = String(props?.entityType ?? "task");
      const entityId = String(props?.entityId ?? "");
      const label = String(props?.label ?? "Linked entity");
      const Icon = resolveEntityIcon(entityType);
      const href =
        entityType === "review"
          ? `/reviews?id=${entityId}`
          : entityType === "agent"
            ? `/agents?id=${entityId}`
            : `/project?id=${entityId}`;

      return (
        <BlockContentWrapper
          blockType="entityCard"
          blockProps={(block as { props: Record<string, unknown> }).props as never}
          propSchema={entityCardConfig.propSchema as never}
        >
          <Link
            href={href}
            className="flex items-center gap-3 rounded-lg border border-amber-500/20 bg-amber-500/5 p-4 transition hover:border-amber-500/40 hover:bg-amber-500/10"
          >
            <Icon className="size-5 text-amber-700" />
            <div className="flex flex-col">
              <span className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
                {entityType}
              </span>
              <span className="font-medium">{label}</span>
            </div>
          </Link>
        </BlockContentWrapper>
      );
    },
  }
);
