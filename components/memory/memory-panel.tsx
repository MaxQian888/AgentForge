"use client";

import { useEffect, useState } from "react";
import { Search, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import {
  useMemoryStore,
  type AgentMemoryEntry,
} from "@/lib/stores/memory-store";

const scopeColors: Record<string, string> = {
  global: "bg-purple-500/15 text-purple-700 dark:text-purple-400",
  project: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  role: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
};

interface MemoryPanelProps {
  projectId: string;
}

export function MemoryPanel({ projectId }: MemoryPanelProps) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");

  const entries = useMemoryStore((s) => s.entries);
  const loading = useMemoryStore((s) => s.loading);
  const searchMemory = useMemoryStore((s) => s.searchMemory);
  const deleteMemory = useMemoryStore((s) => s.deleteMemory);

  useEffect(() => {
    const cat = category === "all" ? undefined : category;
    void searchMemory(projectId, query || undefined, undefined, cat);
  }, [projectId, query, category, searchMemory]);

  const filteredEntries = entries;

  return (
    <div className="flex flex-col gap-4">
      <div className="relative">
        <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search memory entries..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="pl-9"
        />
      </div>

      <Tabs value={category} onValueChange={setCategory}>
        <TabsList>
          <TabsTrigger value="all">All</TabsTrigger>
          <TabsTrigger value="episodic">Episodic</TabsTrigger>
          <TabsTrigger value="semantic">Semantic</TabsTrigger>
          <TabsTrigger value="procedural">Procedural</TabsTrigger>
        </TabsList>

        <TabsContent value={category} className="mt-4">
          {loading ? (
            <p className="text-muted-foreground">Loading memories...</p>
          ) : filteredEntries.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <p className="text-muted-foreground">
                  No memory entries found.
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="flex flex-col gap-3">
              {filteredEntries.map((entry) => (
                <MemoryEntryCard
                  key={entry.id}
                  entry={entry}
                  onDelete={() => deleteMemory(projectId, entry.id)}
                />
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}

function MemoryEntryCard({
  entry,
  onDelete,
}: {
  entry: AgentMemoryEntry;
  onDelete: () => void;
}) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-2 py-3">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-medium">{entry.key}</h4>
          <div className="flex items-center gap-2">
            <Badge
              variant="secondary"
              className={cn(scopeColors[entry.scope] ?? "")}
            >
              {entry.scope}
            </Badge>
            <Badge variant="outline" className="text-xs">
              {entry.category}
            </Badge>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={onDelete}
              className="text-muted-foreground hover:text-destructive"
            >
              <Trash2 className="size-3.5" />
            </Button>
          </div>
        </div>
        <p className="line-clamp-2 text-sm text-muted-foreground">
          {entry.content}
        </p>
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          <span>Accessed: {entry.accessCount}x</span>
          <span>{new Date(entry.createdAt).toLocaleString()}</span>
        </div>
      </CardContent>
    </Card>
  );
}
