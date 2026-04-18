"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  useAuditStore,
  type AuditEvent,
  type AuditQueryFilters,
  type AuditResourceType,
} from "@/lib/stores/audit-store";

const RESOURCE_TYPES: AuditResourceType[] = [
  "project", "member", "task", "team_run", "workflow",
  "wiki", "settings", "automation", "dashboard", "auth",
];

interface AuditLogPanelProps {
  projectId: string;
}

// AuditLogPanel renders the project-scoped audit log with filterable list +
// infinite cursor pagination + detail drawer. Caller is responsible for
// gating mounting (i.e. only render when use-project-role.can('audit.read')).
export function AuditLogPanel({ projectId }: AuditLogPanelProps) {
  const t = useTranslations("audit");
  const fetchEvents = useAuditStore((s) => s.fetchEvents);
  const page = useAuditStore((s) => s.byProject[projectId]);
  const detailById = useAuditStore((s) => s.detailById);
  const fetchEventDetail = useAuditStore((s) => s.fetchEventDetail);

  const [filters, setFilters] = useState<AuditQueryFilters>({});
  const [openDetailId, setOpenDetailId] = useState<string | null>(null);

  useEffect(() => {
    void fetchEvents(projectId, filters);
  }, [projectId, fetchEvents]);

  const events = page?.events ?? [];
  const loading = page?.loading ?? false;
  const loadingMore = page?.loadingMore ?? false;
  const error = page?.error ?? null;
  const nextCursor = page?.nextCursor;

  const detailEvent = useMemo(
    () => (openDetailId ? detailById[openDetailId] : undefined),
    [openDetailId, detailById],
  );

  const handleApply = () => {
    void fetchEvents(projectId, filters);
  };
  const handleReset = () => {
    const cleared: AuditQueryFilters = {};
    setFilters(cleared);
    void fetchEvents(projectId, cleared);
  };
  const handleLoadMore = () => {
    if (!nextCursor) return;
    void fetchEvents(projectId, filters, { append: true, cursor: nextCursor });
  };
  const handleOpenDetail = (event: AuditEvent) => {
    setOpenDetailId(event.id);
    if (!detailById[event.id]) {
      void fetchEventDetail(projectId, event.id);
    }
  };

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <h2 className="text-xl font-semibold">{t("title")}</h2>
        <p className="text-sm text-muted-foreground">{t("description")}</p>
      </div>

      <div className="grid gap-3 rounded-lg border p-4 sm:grid-cols-2 lg:grid-cols-4">
        <div>
          <Label htmlFor="audit-action">{t("filters.actionId")}</Label>
          <Input
            id="audit-action"
            value={filters.actionId ?? ""}
            placeholder={t("filters.anyAction")}
            onChange={(e) =>
              setFilters((prev) => ({ ...prev, actionId: e.target.value || undefined }))
            }
          />
        </div>
        <div>
          <Label htmlFor="audit-actor">{t("filters.actorUserId")}</Label>
          <Input
            id="audit-actor"
            value={filters.actorUserId ?? ""}
            placeholder={t("filters.anyActor")}
            onChange={(e) =>
              setFilters((prev) => ({ ...prev, actorUserId: e.target.value || undefined }))
            }
          />
        </div>
        <div>
          <Label htmlFor="audit-resource-type">{t("filters.resourceType")}</Label>
          <Select
            value={filters.resourceType ?? ""}
            onValueChange={(value) =>
              setFilters((prev) => ({
                ...prev,
                resourceType: (value || undefined) as AuditQueryFilters["resourceType"],
              }))
            }
          >
            <SelectTrigger id="audit-resource-type">
              <SelectValue placeholder={t("filters.anyResource")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("filters.anyResource")}</SelectItem>
              {RESOURCE_TYPES.map((rt) => (
                <SelectItem key={rt} value={rt}>
                  {rt}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <Label htmlFor="audit-resource-id">{t("filters.resourceId")}</Label>
          <Input
            id="audit-resource-id"
            value={filters.resourceId ?? ""}
            onChange={(e) =>
              setFilters((prev) => ({ ...prev, resourceId: e.target.value || undefined }))
            }
          />
        </div>
        <div>
          <Label htmlFor="audit-from">{t("filters.from")}</Label>
          <Input
            id="audit-from"
            type="datetime-local"
            value={filters.from ?? ""}
            onChange={(e) =>
              setFilters((prev) => ({ ...prev, from: e.target.value || undefined }))
            }
          />
        </div>
        <div>
          <Label htmlFor="audit-to">{t("filters.to")}</Label>
          <Input
            id="audit-to"
            type="datetime-local"
            value={filters.to ?? ""}
            onChange={(e) =>
              setFilters((prev) => ({ ...prev, to: e.target.value || undefined }))
            }
          />
        </div>
        <div className="flex items-end gap-2 sm:col-span-2 lg:col-span-4">
          <Button onClick={handleApply} disabled={loading}>
            {t("filters.apply")}
          </Button>
          <Button variant="outline" onClick={handleReset} disabled={loading}>
            {t("filters.reset")}
          </Button>
        </div>
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
          {t("errors.loadFailed")}: {error}
        </div>
      ) : null}

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("table.occurredAt")}</TableHead>
              <TableHead>{t("table.actor")}</TableHead>
              <TableHead>{t("table.role")}</TableHead>
              <TableHead>{t("table.actionId")}</TableHead>
              <TableHead>{t("table.resourceType")}</TableHead>
              <TableHead>{t("table.resourceId")}</TableHead>
              <TableHead>{t("table.outcome")}</TableHead>
              <TableHead>{t("table.system")}</TableHead>
              <TableHead className="w-[1%]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.map((event) => (
              <TableRow key={event.id}>
                <TableCell className="whitespace-nowrap text-xs">
                  {event.occurredAt}
                </TableCell>
                <TableCell className="text-xs">{event.actorUserId ?? "—"}</TableCell>
                <TableCell className="text-xs">
                  {event.actorProjectRoleAtTime ? (
                    <Badge variant="outline">{event.actorProjectRoleAtTime}</Badge>
                  ) : (
                    "—"
                  )}
                </TableCell>
                <TableCell className="text-xs font-mono">{event.actionId}</TableCell>
                <TableCell className="text-xs">{event.resourceType}</TableCell>
                <TableCell className="text-xs">{event.resourceId ?? "—"}</TableCell>
                <TableCell>
                  <Badge
                    variant={event.resourceType === "auth" ? "destructive" : "secondary"}
                  >
                    {event.resourceType === "auth"
                      ? t("outcome.denied")
                      : t("outcome.allowed")}
                  </Badge>
                </TableCell>
                <TableCell className="text-xs">
                  {event.systemInitiated ? "✓" : "—"}
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleOpenDetail(event)}
                  >
                    {t("table.viewDetail")}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {events.length === 0 && !loading ? (
              <TableRow>
                <TableCell colSpan={9} className="text-center text-sm text-muted-foreground py-8">
                  {t("table.empty")}
                </TableCell>
              </TableRow>
            ) : null}
          </TableBody>
        </Table>
      </div>

      <div className="flex justify-center">
        {nextCursor ? (
          <Button onClick={handleLoadMore} disabled={loadingMore} variant="outline">
            {t("table.loadMore")}
          </Button>
        ) : null}
      </div>

      <Sheet
        open={Boolean(openDetailId)}
        onOpenChange={(open) => !open && setOpenDetailId(null)}
      >
        <SheetContent className="w-full sm:max-w-xl overflow-y-auto">
          <SheetHeader>
            <SheetTitle>{t("detail.title")}</SheetTitle>
            <SheetDescription>{detailEvent?.id ?? openDetailId}</SheetDescription>
          </SheetHeader>
          {detailEvent ? (
            <dl className="mt-6 grid grid-cols-3 gap-2 text-sm">
              <dt className="font-medium text-muted-foreground">{t("detail.occurredAt")}</dt>
              <dd className="col-span-2 break-all">{detailEvent.occurredAt}</dd>

              <dt className="font-medium text-muted-foreground">{t("detail.actor")}</dt>
              <dd className="col-span-2 break-all">{detailEvent.actorUserId ?? "—"}</dd>

              <dt className="font-medium text-muted-foreground">{t("detail.actorRole")}</dt>
              <dd className="col-span-2 break-all">
                {detailEvent.actorProjectRoleAtTime ?? "—"}
              </dd>

              <dt className="font-medium text-muted-foreground">{t("detail.actionId")}</dt>
              <dd className="col-span-2 break-all font-mono text-xs">
                {detailEvent.actionId}
              </dd>

              <dt className="font-medium text-muted-foreground">{t("detail.resourceType")}</dt>
              <dd className="col-span-2 break-all">{detailEvent.resourceType}</dd>

              <dt className="font-medium text-muted-foreground">{t("detail.resourceId")}</dt>
              <dd className="col-span-2 break-all">{detailEvent.resourceId ?? "—"}</dd>

              <dt className="font-medium text-muted-foreground">
                {t("detail.systemInitiated")}
              </dt>
              <dd className="col-span-2">{detailEvent.systemInitiated ? "✓" : "—"}</dd>

              <dt className="font-medium text-muted-foreground">{t("detail.configuredBy")}</dt>
              <dd className="col-span-2 break-all">
                {detailEvent.configuredByUserId ?? "—"}
              </dd>

              <dt className="font-medium text-muted-foreground">{t("detail.requestId")}</dt>
              <dd className="col-span-2 break-all">{detailEvent.requestId ?? "—"}</dd>

              <dt className="font-medium text-muted-foreground">{t("detail.ip")}</dt>
              <dd className="col-span-2 break-all">{detailEvent.ip ?? "—"}</dd>

              <dt className="font-medium text-muted-foreground">{t("detail.userAgent")}</dt>
              <dd className="col-span-2 break-all text-xs">{detailEvent.userAgent ?? "—"}</dd>

              <dt className="col-span-3 font-medium text-muted-foreground mt-4">
                {t("detail.payload")}
              </dt>
              <dd className="col-span-3">
                <pre className="rounded-lg border bg-muted/40 p-3 text-xs overflow-auto max-h-96">
                  {formatJSON(detailEvent.payloadSnapshotJson)}
                </pre>
              </dd>
            </dl>
          ) : (
            <p className="mt-6 text-sm text-muted-foreground">…</p>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}

function formatJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}
