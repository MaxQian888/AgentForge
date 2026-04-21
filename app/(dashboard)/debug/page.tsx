"use client";

// Admin gate note: AuthUser in this project has no global `role` field
// (only id, email, name). There is no project-level RBAC applicable here
// either. The debug endpoints are JWT-gated server-side (403 for non-permitted
// users). The frontend page is therefore accessible to any authenticated user,
// and the backend 403 is the authoritative access control boundary.

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TimelineTab } from "./timeline";
import { LiveTailTab } from "./live-tail";

export default function DebugPage() {
  return (
    <div className="p-[var(--space-page-inline)] flex flex-col gap-[var(--space-section-gap)]">
      <div>
        <h1 className="text-2xl font-semibold">Debug</h1>
        <p className="text-sm text-muted-foreground">
          Cross-service timeline and live event stream.
        </p>
      </div>
      <Tabs defaultValue="timeline">
        <TabsList>
          <TabsTrigger value="timeline">Timeline</TabsTrigger>
          <TabsTrigger value="live">Live Tail</TabsTrigger>
        </TabsList>
        <TabsContent value="timeline">
          <TimelineTab />
        </TabsContent>
        <TabsContent value="live">
          <LiveTailTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
