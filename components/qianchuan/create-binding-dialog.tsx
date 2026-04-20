"use client";

import { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useQianchuanBindingsStore } from "@/lib/stores/qianchuan-bindings-store";
import { useSecretsStore } from "@/lib/stores/secrets-store";

interface Props {
  projectId: string;
  open: boolean;
  onOpenChange: (next: boolean) => void;
}

/**
 * Plan 3A only supports MANUAL token pasting via secret-ref selection.
 * The user is expected to have previously created two secrets (one for the
 * access token, one for the refresh token) on the project secrets page (1B).
 * Plan 3B replaces this dialog with an OAuth-driven flow.
 */
export function CreateBindingDialog({
  projectId,
  open,
  onOpenChange,
}: Props) {
  const create = useQianchuanBindingsStore((s) => s.createBinding);
  const fetchSecrets = useSecretsStore((s) => s.fetchSecrets);
  const secretNames = useSecretsStore(
    (s) => s.secretsByProject[projectId]?.map((x) => x.name) ?? [],
  );
  const [advertiserId, setAdvertiserId] = useState("");
  const [awemeId, setAwemeId] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [accessRef, setAccessRef] = useState("");
  const [refreshRef, setRefreshRef] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open && projectId) {
      void fetchSecrets(projectId);
    }
  }, [open, projectId, fetchSecrets]);

  const reset = () => {
    setAdvertiserId("");
    setAwemeId("");
    setDisplayName("");
    setAccessRef("");
    setRefreshRef("");
  };

  const onSubmit = async () => {
    if (!advertiserId || !accessRef || !refreshRef) return;
    setSubmitting(true);
    const out = await create(projectId, {
      advertiserId,
      awemeId: awemeId || undefined,
      displayName: displayName || undefined,
      accessTokenSecretRef: accessRef,
      refreshTokenSecretRef: refreshRef,
    });
    setSubmitting(false);
    if (out) {
      reset();
      onOpenChange(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>新增千川绑定（手动 token）</DialogTitle>
        </DialogHeader>
        <div className="grid gap-3 py-2">
          <div className="grid gap-1.5">
            <Label>advertiser_id *</Label>
            <Input
              value={advertiserId}
              onChange={(e) => setAdvertiserId(e.target.value)}
              placeholder="如 1234567890"
            />
          </div>
          <div className="grid gap-1.5">
            <Label>aweme_id（可选）</Label>
            <Input
              value={awemeId}
              onChange={(e) => setAwemeId(e.target.value)}
            />
          </div>
          <div className="grid gap-1.5">
            <Label>显示名</Label>
            <Input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="店铺A 主直播间"
            />
          </div>
          <div className="grid gap-1.5">
            <Label>access_token 密钥 *</Label>
            <SecretRefSelect
              value={accessRef}
              onChange={setAccessRef}
              options={secretNames}
            />
          </div>
          <div className="grid gap-1.5">
            <Label>refresh_token 密钥 *</Label>
            <SecretRefSelect
              value={refreshRef}
              onChange={setRefreshRef}
              options={secretNames}
            />
          </div>
          <p className="text-xs text-muted-foreground">
            提示：OAuth 一键绑定将在 Plan 3B 推出。当前需先在 项目设置 → 密钥管理 创建两个密钥。
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button onClick={onSubmit} disabled={submitting}>
            {submitting ? "创建中…" : "创建"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function SecretRefSelect({
  value,
  onChange,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  options: string[];
}) {
  if (!options || options.length === 0) {
    return (
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="未发现项目密钥；请先在密钥管理页创建"
      />
    );
  }
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger>
        <SelectValue placeholder="选择密钥…" />
      </SelectTrigger>
      <SelectContent>
        {options.map((name) => (
          <SelectItem key={name} value={name}>
            {name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
