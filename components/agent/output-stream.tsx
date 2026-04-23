"use client";

import { useEffect, useRef } from "react";
import { useTranslations } from "next-intl";
import { TerminalSquareIcon } from "lucide-react";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { ScrollArea } from "@/components/ui/scroll-area";

interface OutputStreamProps {
  lines: string[];
}

export function OutputStream({ lines }: OutputStreamProps) {
  const t = useTranslations("agents");
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [lines.length]);

  if (lines.length === 0) {
    return (
      <div className="flex h-96 items-center justify-center rounded-md border border-dashed bg-muted/20 p-4">
        <Empty className="border-0 p-0">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <TerminalSquareIcon className="size-5" />
            </EmptyMedia>
            <EmptyTitle>{t("outputStream.waitingTitle")}</EmptyTitle>
            <EmptyDescription>
              {t("outputStream.waitingDescription")}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
        <div ref={bottomRef} />
      </div>
    );
  }

  return (
    <ScrollArea className="h-96 rounded-md border bg-zinc-950 p-4">
      <pre className="font-mono text-xs leading-5 text-green-400">
        {lines.map((line, i) => (
          <div key={i}>{line}</div>
        ))}
      </pre>
      <div ref={bottomRef} />
    </ScrollArea>
  );
}
