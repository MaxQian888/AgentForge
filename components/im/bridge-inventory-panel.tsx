import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { IMBridgeInstance } from "@/lib/stores/im-store";

export interface BridgeInventoryPanelProps {
  bridges: IMBridgeInstance[];
}

export function BridgeInventoryPanel({ bridges }: BridgeInventoryPanelProps) {
  if (bridges.length === 0) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-sm text-muted-foreground">
          No IM bridges online. Start <code>src-im-bridge</code> to populate this panel.
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      {bridges.map((bridge) => (
        <Card
          key={bridge.bridgeId}
          data-testid={`bridge-card-${bridge.bridgeId}`}
          className={cn(bridge.status !== "online" && "opacity-60")}
        >
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-base">
              {bridge.bridgeId}
              <span className="ml-2 text-xs font-normal text-muted-foreground">
                ({bridge.status})
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <section>
              <h3 className="mb-2 text-sm font-semibold">Providers</h3>
              <ul className="space-y-2">
                {(bridge.providers ?? []).map((p) => (
                  <li key={p.id} className="rounded-md border p-3 text-sm">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{p.id}</span>
                      <Badge variant="outline">{p.transport}</Badge>
                      {p.readinessTier && <Badge variant="secondary">{p.readinessTier}</Badge>}
                    </div>
                    {(p.tenants?.length ?? 0) > 0 && (
                      <div className="mt-2 flex flex-wrap gap-1">
                        {p.tenants!.map((t) => (
                          <Badge key={t} variant="outline">{t}</Badge>
                        ))}
                      </div>
                    )}
                    {p.capabilityMatrix && (
                      <details className="mt-2 text-xs text-muted-foreground">
                        <summary className="cursor-pointer">Capability matrix</summary>
                        <pre className="mt-1 overflow-auto">{JSON.stringify(p.capabilityMatrix, null, 2)}</pre>
                      </details>
                    )}
                  </li>
                ))}
              </ul>
            </section>
            <section>
              <h3 className="mb-2 text-sm font-semibold">Command Plugins</h3>
              {(bridge.commandPlugins?.length ?? 0) === 0 ? (
                <p className="text-xs text-muted-foreground">None loaded.</p>
              ) : (
                <ul className="space-y-2">
                  {bridge.commandPlugins!.map((cp) => (
                    <li key={cp.id} className="rounded-md border p-3 text-sm">
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{cp.id}</span>
                        <Badge variant="outline">v{cp.version}</Badge>
                      </div>
                      <div className="mt-2 flex flex-wrap gap-1">
                        {cp.commands.map((cmd) => (
                          <Badge key={cmd} variant="secondary">{cmd}</Badge>
                        ))}
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
