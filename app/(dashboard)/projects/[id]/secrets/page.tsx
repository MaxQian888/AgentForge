// TODO(spec1-1A): once projects/[id]/layout.tsx ships, add a "Secrets"
// entry to the sidebar that links here.
"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Copy, KeyRound, Plus, RotateCw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { EmptyState } from "@/components/shared/empty-state";
import {
  useSecretsStore,
  type SecretMetadata,
} from "@/lib/stores/secrets-store";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { toast } from "sonner";

export default function ProjectSecretsPage() {
  const params = useParams<{ id: string }>();
  const projectId = params?.id ?? "";
  useBreadcrumbs([
    { label: "Projects", href: "/projects" },
    { label: projectId, href: `/projects/${projectId}` },
    { label: "Secrets" },
  ]);

  const {
    secretsByProject,
    loadingByProject,
    lastRevealedValue,
    fetchSecrets,
    createSecret,
    rotateSecret,
    deleteSecret,
    consumeRevealedValue,
  } = useSecretsStore();

  const [createOpen, setCreateOpen] = useState(false);
  const [rotateTarget, setRotateTarget] = useState<SecretMetadata | null>(null);

  useEffect(() => {
    if (projectId) fetchSecrets(projectId);
  }, [projectId, fetchSecrets]);

  const rows = secretsByProject[projectId] ?? [];
  const loading = loadingByProject[projectId] ?? false;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="项目密钥"
        description="管理项目级敏感凭证。值仅在创建/轮换时一次性返回。"
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="mr-1 size-4" />
            新建密钥
          </Button>
        }
      />

      <SectionCard
        title="密钥列表"
        description="本表只显示元数据。明文值不会被存储或再次返回。"
      >
        {loading ? (
          <div className="flex flex-col gap-2 p-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </div>
        ) : rows.length === 0 ? (
          <EmptyState icon={KeyRound} title="尚未创建任何密钥" />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>名称</TableHead>
                <TableHead>描述</TableHead>
                <TableHead>最近使用</TableHead>
                <TableHead>创建时间</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={r.name}>
                  <TableCell className="font-mono">{r.name}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {r.description ?? "—"}
                  </TableCell>
                  <TableCell>
                    {r.lastUsedAt
                      ? new Date(r.lastUsedAt).toLocaleString()
                      : "—"}
                  </TableCell>
                  <TableCell>
                    {new Date(r.createdAt).toLocaleString()}
                  </TableCell>
                  <TableCell className="flex justify-end gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => setRotateTarget(r)}
                    >
                      <RotateCw className="mr-1 size-3" />
                      轮换
                    </Button>
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={async () => {
                        if (confirm(`确认删除 ${r.name}？`)) {
                          await deleteSecret(projectId, r.name);
                        }
                      }}
                    >
                      <Trash2 className="mr-1 size-3" />
                      删除
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </SectionCard>

      <CreateSecretDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onSubmit={async (name, value, description) => {
          await createSecret(projectId, name, value, description);
          setCreateOpen(false);
        }}
      />

      {rotateTarget && (
        <RotateSecretDialog
          target={rotateTarget}
          onClose={() => setRotateTarget(null)}
          onSubmit={async (value) => {
            await rotateSecret(projectId, rotateTarget.name, value);
            setRotateTarget(null);
          }}
        />
      )}

      {lastRevealedValue && lastRevealedValue.projectId === projectId && (
        <RevealedValueDialog
          name={lastRevealedValue.name}
          value={lastRevealedValue.value}
          onClose={consumeRevealedValue}
        />
      )}
    </div>
  );
}

function CreateSecretDialog({
  open,
  onClose,
  onSubmit,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (name: string, value: string, description: string) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [value, setValue] = useState("");
  const [description, setDescription] = useState("");
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>新建密钥</DialogTitle>
          <DialogDescription>
            名称在项目内必须唯一；值仅在本次响应中显示一次。
          </DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-3"
          onSubmit={async (e) => {
            e.preventDefault();
            if (!name || !value) return;
            await onSubmit(name, value, description);
            setName("");
            setValue("");
            setDescription("");
          }}
        >
          <div className="flex flex-col gap-1">
            <Label htmlFor="secret-name">名称</Label>
            <Input
              id="secret-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="secret-value">值</Label>
            <Input
              id="secret-value"
              type="password"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              required
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="secret-desc">描述（可选）</Label>
            <Input
              id="secret-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit">创建</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function RotateSecretDialog({
  target,
  onClose,
  onSubmit,
}: {
  target: SecretMetadata;
  onClose: () => void;
  onSubmit: (value: string) => Promise<void>;
}) {
  const [value, setValue] = useState("");
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>轮换 {target.name}</DialogTitle>
          <DialogDescription>新值仅会显示一次；旧值立即失效。</DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-3"
          onSubmit={async (e) => {
            e.preventDefault();
            if (!value) return;
            await onSubmit(value);
            setValue("");
          }}
        >
          <Label htmlFor="rotate-value">新值</Label>
          <Input
            id="rotate-value"
            type="password"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            required
          />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit">轮换</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function RevealedValueDialog({
  name,
  value,
  onClose,
}: {
  name: string;
  value: string;
  onClose: () => void;
}) {
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>请保存 {name} 的值</DialogTitle>
          <DialogDescription className="text-destructive">
            关闭此对话框后将无法再次查看；请立即复制并妥善保管。
          </DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2">
          <Input readOnly value={value} className="font-mono" />
          <Button
            size="sm"
            variant="outline"
            onClick={async () => {
              await navigator.clipboard.writeText(value);
              toast.success("已复制到剪贴板");
            }}
          >
            <Copy className="mr-1 size-3" />
            复制
          </Button>
        </div>
        <DialogFooter>
          <Button onClick={onClose}>我已保存</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
