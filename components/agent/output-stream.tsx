"use client";

import { useEffect, useRef } from "react";
import { ScrollArea } from "@/components/ui/scroll-area";

interface OutputStreamProps {
  lines: string[];
}

export function OutputStream({ lines }: OutputStreamProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [lines.length]);

  return (
    <ScrollArea className="h-96 rounded-md border bg-zinc-950 p-4">
      <pre className="font-mono text-xs leading-5 text-green-400">
        {lines.length === 0
          ? "Waiting for output..."
          : lines.map((line, i) => (
              <div key={i}>{line}</div>
            ))}
      </pre>
      <div ref={bottomRef} />
    </ScrollArea>
  );
}
