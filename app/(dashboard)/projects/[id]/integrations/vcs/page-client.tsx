// TODO(spec2-2A nav): once projects/[id]/layout.tsx ships, add an
// "Integrations" entry to the sidebar that links to ./integrations/vcs.
"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import {
  AlertTriangle,
  Code2 as Github,
  Plus,
  RefreshCw,
  Trash2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { useSecretsStore } from "@/lib/stores/secrets-store";
import {
  useVCSIntegrationsStore,
  type CreateIntegrationInput,
  type VCSIntegration,
  type VCSProvider,
} from "@/lib/stores/vcs-integrations-store";

const PROVIDERS: { value: VCSProvider; label: string; enabled: boolean }[] = [
  { value: "github", label: "GitHub", enabled: true },
  { value: "gitlab", label: "GitLab (coming soon)", enabled: false },
  { value: "gitea", label: "Gitea (coming soon)", enabled: false },
];

function StatusBadge({ status }: { status: VCSIntegration["status"] }) {
  switch (status) {
    case "active":
      return <Badge variant="default">active</Badge>;
    case "auth_expired":
      return <Badge variant="destructive">auth expired</Badge>;
    case "paused":
      return <Badge variant="secondary">paused</Badge>;
  }
}

export default function VCSIntegrationsPage() {
  const params = useParams<{ id: string }>();
  const projectId = params?.id ?? "";
  useBreadcrumbs([
    { label: "Projects", href: "/projects" },
    { label: projectId, href: `/projects/${projectId}` },
    { label: "Integrations · VCS" },
  ]);

  const {
    integrationsByProject,
    loadingByProject,
    fetchIntegrations,
    deleteIntegration,
    syncIntegration,
  } = useVCSIntegrationsStore();
  const { fetchSecrets } = useSecretsStore();

  const integrations = integrationsByProject[projectId] ?? [];
  const loading = loadingByProject[projectId] ?? false;

  useEffect(() => {
    if (projectId) {
      fetchIntegrations(projectId);
      fetchSecrets(projectId);
    }
  }, [projectId, fetchIntegrations, fetchSecrets]);

  const [createOpen, setCreateOpen] = useState(false);

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader
        title="VCS 集成"
        description="连接代码仓库以启用 PR 审查与自动修复。PAT 与 webhook secret 由 Secrets 子系统持有。"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="mr-1 size-4" />
            连接仓库
          </Button>
        }
      />

      <SectionCard
        title="已连接的仓库"
        description="每条记录绑定到一个项目；删除会同时撤销远端 webhook。"
      >
        {loading ? (
          <div className="flex flex-col gap-2 p-4">
            {Array.from({ length: 2 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        ) : integrations.length === 0 ? (
          <EmptyState icon={Github} title="尚未连接任何仓库" />
        ) : (
          <div className="flex flex-col gap-3 p-4">
            {integrations.map((it) => (
              <IntegrationRow
                key={it.id}
                integration={it}
                onSync={() => syncIntegration(it.id)}
                onDelete={() => {
                  if (confirm(`确认删除 ${it.owner}/${it.repo}？`)) {
                    deleteIntegration(projectId, it.id);
                  }
                }}
              />
            ))}
          </div>
        )}
      </SectionCard>

      <CreateIntegrationDialog
        projectId={projectId}
        open={createOpen}
        onClose={() => setCreateOpen(false)}
      />
    </div>
  );
}

function IntegrationRow({
  integration,
  onSync,
  onDelete,
}: {
  integration: VCSIntegration;
  onSync: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="flex flex-col gap-2 rounded-md border p-4">
      <div className="flex items-start justify-between">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2 font-medium">
            <Github className="size-4" />
            {integration.owner}/{integration.repo}
          </div>
          <p className="text-xs text-muted-foreground">
            {integration.host} · 默认分支 {integration.defaultBranch}
          </p>
        </div>
        <StatusBadge status={integration.status} />
      </div>
      <div className="grid grid-cols-1 gap-1 text-xs sm:grid-cols-2">
        <div>
          <span className="text-muted-foreground">Webhook ID: </span>
          <code>{integration.webhookId ?? "—"}</code>
        </div>
        <div>
          <span className="text-muted-foreground">PAT secret: </span>
          <code>{integration.tokenSecretRef}</code>
        </div>
        <div>
          <span className="text-muted-foreground">Webhook secret: </span>
          <code>{integration.webhookSecretRef}</code>
        </div>
        <div>
          <span className="text-muted-foreground">最近同步: </span>
          {integration.lastSyncedAt
            ? new Date(integration.lastSyncedAt).toLocaleString()
            : "—"}
        </div>
      </div>
      <div className="flex justify-end gap-2 pt-2">
        <Button size="sm" variant="outline" onClick={onSync}>
          <RefreshCw className="mr-1 size-3" />
          重新同步
        </Button>
        <Button size="sm" variant="destructive" onClick={onDelete}>
          <Trash2 className="mr-1 size-3" />
          删除
        </Button>
      </div>
    </div>
  );
}

function CreateIntegrationDialog({
  projectId,
  open,
  onClose,
}: {
  projectId: string;
  open: boolean;
  onClose: () => void;
}) {
  const { createIntegration } = useVCSIntegrationsStore();
  const { secretsByProject } = useSecretsStore();
  const secrets = secretsByProject[projectId] ?? [];

  const [provider, setProvider] = useState<VCSProvider>("github");
  const [host, setHost] = useState("github.com");
  const [owner, setOwner] = useState("");
  const [repo, setRepo] = useState("");
  const [branch, setBranch] = useState("main");
  const [tokenSecretRef, setTokenSecretRef] = useState("");
  const [webhookSecretRef, setWebhookSecretRef] = useState("");
  const [createdWebhookID, setCreatedWebhookID] = useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!tokenSecretRef || !webhookSecretRef) return;
    const input: CreateIntegrationInput = {
      provider,
      host,
      owner,
      repo,
      defaultBranch: branch,
      tokenSecretRef,
      webhookSecretRef,
    };
    const result = await createIntegration(projectId, input);
    if (result?.webhookId) {
      setCreatedWebhookID(result.webhookId);
    }
  };

  const close = () => {
    onClose();
    setCreatedWebhookID(null);
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && close()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>连接仓库</DialogTitle>
          <DialogDescription>
            PAT / webhook secret 必须先在 Secrets 页面创建。GitLab 与 Gitea 暂未实现。
          </DialogDescription>
        </DialogHeader>

        {createdWebhookID ? (
          <div className="flex flex-col gap-2 text-sm">
            <p>
              已创建 webhook <code>{createdWebhookID}</code>。
            </p>
            <p className="text-xs text-muted-foreground">
              回调 URL 已自动注册到仓库；如需查看可在 GitHub Settings → Webhooks 验证。
            </p>
            <DialogFooter>
              <Button onClick={close}>完成</Button>
            </DialogFooter>
          </div>
        ) : (
          <form onSubmit={onSubmit} className="flex flex-col gap-3">
            <div className="flex flex-col gap-1">
              <Label>Provider</Label>
              <Select
                value={provider}
                onValueChange={(v) => setProvider(v as VCSProvider)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PROVIDERS.map((p) => (
                    <SelectItem
                      key={p.value}
                      value={p.value}
                      disabled={!p.enabled}
                    >
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="vcs-host">Host</Label>
              <Input
                id="vcs-host"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                required
              />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div className="flex flex-col gap-1">
                <Label htmlFor="vcs-owner">Owner</Label>
                <Input
                  id="vcs-owner"
                  value={owner}
                  onChange={(e) => setOwner(e.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor="vcs-repo">Repo</Label>
                <Input
                  id="vcs-repo"
                  value={repo}
                  onChange={(e) => setRepo(e.target.value)}
                  required
                />
              </div>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="vcs-branch">默认分支</Label>
              <Input
                id="vcs-branch"
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1">
              <Label>PAT secret</Label>
              <Select value={tokenSecretRef} onValueChange={setTokenSecretRef}>
                <SelectTrigger>
                  <SelectValue placeholder="选择密钥" />
                </SelectTrigger>
                <SelectContent>
                  {secrets.map((s) => (
                    <SelectItem key={s.name} value={s.name}>
                      {s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1">
              <Label>Webhook secret</Label>
              <Select
                value={webhookSecretRef}
                onValueChange={setWebhookSecretRef}
              >
                <SelectTrigger>
                  <SelectValue placeholder="选择密钥" />
                </SelectTrigger>
                <SelectContent>
                  {secrets.map((s) => (
                    <SelectItem key={s.name} value={s.name}>
                      {s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {(!tokenSecretRef || !webhookSecretRef) && (
              <p className="flex items-center gap-1 text-xs text-amber-600">
                <AlertTriangle className="size-3" />
                需要先在 Secrets 页面创建 PAT 与 webhook secret。
              </p>
            )}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={close}>
                取消
              </Button>
              <Button
                type="submit"
                disabled={!tokenSecretRef || !webhookSecretRef}
              >
                创建
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
