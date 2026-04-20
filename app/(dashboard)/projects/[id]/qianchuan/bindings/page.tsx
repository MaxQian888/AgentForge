"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  useQianchuanBindingsStore,
  type QianchuanBindingStatus,
} from "@/lib/stores/qianchuan-bindings-store";
import { CreateBindingDialog } from "@/components/qianchuan/create-binding-dialog";

const STATUS_LABEL: Record<
  QianchuanBindingStatus,
  { label: string; variant: "default" | "secondary" | "destructive" }
> = {
  active: { label: "运行中", variant: "default" },
  auth_expired: { label: "授权过期", variant: "destructive" },
  paused: { label: "已暂停", variant: "secondary" },
};

export default function QianchuanBindingsPage() {
  const params = useParams<{ id: string }>();
  const projectId = params?.id ?? "";

  const rows = useQianchuanBindingsStore(
    (s) => s.byProject[projectId] ?? [],
  );
  const loading = useQianchuanBindingsStore(
    (s) => s.loading[projectId] ?? false,
  );
  const fetchBindings = useQianchuanBindingsStore((s) => s.fetchBindings);
  const updateBinding = useQianchuanBindingsStore((s) => s.updateBinding);
  const syncBinding = useQianchuanBindingsStore((s) => s.syncBinding);
  const testBinding = useQianchuanBindingsStore((s) => s.testBinding);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (projectId) {
      void fetchBindings(projectId);
    }
  }, [projectId, fetchBindings]);

  const sorted = useMemo(
    () =>
      [...rows].sort((a, b) =>
        (a.displayName ?? "").localeCompare(b.displayName ?? ""),
      ),
    [rows],
  );

  return (
    <div className="space-y-4 p-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">千川账号绑定</h1>
        <Button onClick={() => setOpen(true)}>新增绑定</Button>
      </div>
      {loading && (
        <p className="text-sm text-muted-foreground">加载中…</p>
      )}
      {!loading && sorted.length === 0 && (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            尚未绑定任何千川账号。点击右上角“新增绑定”开始。
          </CardContent>
        </Card>
      )}
      <div className="grid gap-3">
        {sorted.map((b) => (
          <Card key={b.id}>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-base">
                {b.displayName || b.advertiserId}
                <span className="ml-2 text-xs text-muted-foreground">
                  advertiser_id={b.advertiserId}
                  {b.awemeId ? ` · aweme_id=${b.awemeId}` : ""}
                </span>
              </CardTitle>
              <Badge variant={STATUS_LABEL[b.status].variant}>
                {STATUS_LABEL[b.status].label}
              </Badge>
            </CardHeader>
            <CardContent className="flex items-center gap-2 text-sm">
              <span className="text-muted-foreground">
                最近同步：{b.lastSyncedAt ?? "—"}
              </span>
              <span className="ml-auto flex gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={async () => {
                    const r = await testBinding(b.id);
                    if (r.ok) {
                      await syncBinding(b.id);
                    }
                  }}
                >
                  测试
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => syncBinding(b.id)}
                >
                  同步
                </Button>
                {b.status === "paused" ? (
                  <Button
                    size="sm"
                    onClick={() =>
                      updateBinding(b.id, { status: "active" })
                    }
                  >
                    恢复
                  </Button>
                ) : (
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() =>
                      updateBinding(b.id, { status: "paused" })
                    }
                  >
                    暂停
                  </Button>
                )}
              </span>
            </CardContent>
          </Card>
        ))}
      </div>
      <CreateBindingDialog
        projectId={projectId}
        open={open}
        onOpenChange={setOpen}
      />
    </div>
  );
}
