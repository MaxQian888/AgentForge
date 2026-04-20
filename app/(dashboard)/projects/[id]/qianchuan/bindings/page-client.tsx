"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useParams, useSearchParams, useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useQianchuanBindingsStore,
  type QianchuanBindingStatus,
  type QianchuanBinding,
} from "@/lib/stores/qianchuan-bindings-store";
import { CreateBindingDialog } from "@/components/qianchuan/create-binding-dialog";
import { toast } from "sonner";
import { useAuthStore } from "@/lib/stores/auth-store";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

const STATUS_LABEL: Record<
  QianchuanBindingStatus,
  { label: string; variant: "default" | "secondary" | "destructive" }
> = {
  active: { label: "运行中", variant: "default" },
  auth_expired: { label: "授权过期", variant: "destructive" },
  paused: { label: "已暂停", variant: "secondary" },
};

// --- OAuth Bind Button ---

function BindOAuthButton({
  projectId,
}: {
  projectId: string;
}) {
  const [open, setOpen] = useState(false);
  const [displayName, setDisplayName] = useState("");
  const [loading, setLoading] = useState(false);

  const initiate = useCallback(async () => {
    setLoading(true);
    try {
      const token =
        (useAuthStore.getState() as { accessToken?: string | null })
          .accessToken ?? null;
      const res = await fetch(
        `${API_URL}/api/v1/projects/${projectId}/qianchuan/oauth/bind/initiate`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({
            display_name: displayName || undefined,
          }),
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        toast.error(`绑定发起失败: ${(err as { error?: string }).error ?? res.statusText}`);
        return;
      }
      const data = (await res.json()) as { authorize_url: string; state_token: string };
      window.location.assign(data.authorize_url);
    } catch (e) {
      toast.error(`绑定发起失败: ${(e as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, [projectId, displayName]);

  return (
    <>
      <Button onClick={() => setOpen(true)}>OAuth 绑定</Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>通过 OAuth 绑定千川账号</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div>
              <Label htmlFor="oauth-display-name">显示名称（可选）</Label>
              <Input
                id="oauth-display-name"
                placeholder="例：店铺A"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
              <p className="text-xs text-muted-foreground mt-1">
                OAuth 绑定仅支持单个广告主；如授权包含多个广告主，需逐一绑定。
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button onClick={initiate} disabled={loading}>
              {loading ? "跳转中..." : "前往授权"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

// --- Auth Expired Banner ---

function AuthExpiredBanner({
  binding,
  onRebind,
}: {
  binding: QianchuanBinding;
  onRebind: () => void;
}) {
  return (
    <div className="rounded-md border border-destructive bg-destructive/10 p-4 flex items-center justify-between">
      <div>
        <p className="font-medium text-destructive">
          绑定 &ldquo;{binding.displayName || binding.advertiserId}&rdquo;
          {" "}授权已过期
        </p>
        <p className="text-sm text-muted-foreground">
          请重新绑定以恢复策略循环。
        </p>
      </div>
      <Button variant="destructive" onClick={onRebind}>
        重新绑定
      </Button>
    </div>
  );
}

// --- Main Page ---

export default function QianchuanBindingsPage() {
  const params = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const router = useRouter();
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

  // Handle OAuth callback query params
  useEffect(() => {
    const bind = searchParams?.get("bind");
    const advertiser = searchParams?.get("advertiser");
    if (bind === "success") {
      toast.success(
        advertiser
          ? `千川账号 ${advertiser} 绑定成功`
          : "千川账号绑定成功",
      );
      router.replace(`/projects/${projectId}/qianchuan/bindings`);
    } else if (bind === "error") {
      const code = searchParams?.get("code") ?? "unknown";
      toast.error(`绑定失败: ${code}`);
      router.replace(`/projects/${projectId}/qianchuan/bindings`);
    }
  }, [searchParams, projectId, router]);

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

  const expiredBindings = useMemo(
    () => sorted.filter((b) => b.status === "auth_expired"),
    [sorted],
  );

  const handleRebind = useCallback(
    () => {
      toast.info("请点击 OAuth 绑定按钮重新授权");
    },
    [],
  );

  return (
    <div className="space-y-4 p-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">千川账号绑定</h1>
        <div className="flex gap-2">
          <BindOAuthButton projectId={projectId} />
          <Button variant="outline" onClick={() => setOpen(true)}>
            手动绑定
          </Button>
        </div>
      </div>

      {/* Auth expired banners */}
      {expiredBindings.map((b) => (
        <AuthExpiredBanner key={b.id} binding={b} onRebind={handleRebind} />
      ))}

      {loading && (
        <p className="text-sm text-muted-foreground">加载中...</p>
      )}
      {!loading && sorted.length === 0 && (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            尚未绑定任何千川账号。点击右上角&ldquo;OAuth 绑定&rdquo;开始。
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
                ) : b.status === "auth_expired" ? (
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => handleRebind()}
                  >
                    重新绑定
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
