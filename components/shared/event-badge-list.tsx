import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type EventBadgeListProps = {
  events: string[];
  className?: string;
  emptyLabel?: string;
};

export function EventBadgeList({
  events,
  className,
  emptyLabel,
}: EventBadgeListProps) {
  if (events.length === 0 && emptyLabel) {
    return <span className="text-xs text-muted-foreground">{emptyLabel}</span>;
  }

  return (
    <div className={cn("flex flex-wrap gap-1", className)}>
      {events.map((event) => (
        <Badge key={event} variant="secondary" className="text-xs">
          {event}
        </Badge>
      ))}
    </div>
  );
}
