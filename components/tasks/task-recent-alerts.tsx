import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Notification } from "@/lib/stores/notification-store";

export interface TaskRecentAlertsProps {
  alerts: Notification[];
}

export function TaskRecentAlerts({ alerts }: TaskRecentAlertsProps) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">Recent alerts</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3 text-sm">
        {alerts.length === 0 ? (
          <div className="text-muted-foreground">No recent task alerts.</div>
        ) : (
          alerts.map((alert) => (
            <div key={alert.id} className="rounded-md border border-border/60 p-3">
              {alert.href ? (
                <a className="font-medium hover:underline" href={alert.href}>
                  {alert.title}
                </a>
              ) : (
                <div className="font-medium">{alert.title}</div>
              )}
              <div className="mt-1 text-muted-foreground">{alert.message}</div>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  );
}
