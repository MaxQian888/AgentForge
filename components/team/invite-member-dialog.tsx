"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
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
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {
  CreateInvitationInput,
  InvitationCreateResponse,
  InvitationIdentity,
  InvitationIdentityKind,
} from "@/lib/stores/invitation-store";

export interface InviteMemberDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onInvite: (input: CreateInvitationInput) => Promise<InvitationCreateResponse>;
}

export function InviteMemberDialog({
  open,
  onOpenChange,
  onInvite,
}: InviteMemberDialogProps) {
  const t = useTranslations("invitations");
  const tc = useTranslations("common");
  const [kind, setKind] = useState<InvitationIdentityKind>("email");
  const [email, setEmail] = useState("");
  const [imPlatform, setImPlatform] = useState("");
  const [imUserId, setImUserId] = useState("");
  const [imDisplayName, setImDisplayName] = useState("");
  const [projectRole, setProjectRole] = useState("editor");
  const [message, setMessage] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastCreated, setLastCreated] = useState<InvitationCreateResponse | null>(
    null,
  );

  const reset = () => {
    setKind("email");
    setEmail("");
    setImPlatform("");
    setImUserId("");
    setImDisplayName("");
    setProjectRole("editor");
    setMessage("");
    setError(null);
    setLastCreated(null);
  };

  const handleSubmit = async () => {
    setError(null);
    let identity: InvitationIdentity;
    if (kind === "email") {
      if (!email.trim()) {
        setError(t("dialog.errorEmailRequired"));
        return;
      }
      identity = { kind: "email", value: email.trim() };
    } else {
      if (!imPlatform.trim() || !imUserId.trim()) {
        setError(t("dialog.errorIMRequired"));
        return;
      }
      identity = {
        kind: "im",
        platform: imPlatform.trim(),
        userId: imUserId.trim(),
        displayName: imDisplayName.trim() || undefined,
      };
    }
    setSubmitting(true);
    try {
      const result = await onInvite({
        invitedIdentity: identity,
        projectRole,
        message: message.trim() || undefined,
      });
      setLastCreated(result);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t("dialog.errorSendFailed"),
      );
    } finally {
      setSubmitting(false);
    }
  };

  const handleClose = () => {
    reset();
    onOpenChange(false);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("dialog.title")}</DialogTitle>
          <DialogDescription>{t("dialog.description")}</DialogDescription>
        </DialogHeader>

        {lastCreated ? (
          <div className="flex flex-col gap-3">
            <p className="text-sm">{t("dialog.createdHeading")}</p>
            <Input
              readOnly
              value={lastCreated.acceptUrl}
              onFocus={(e) => e.currentTarget.select()}
              aria-label={t("dialog.acceptUrlAriaLabel")}
            />
            <DialogFooter>
              <Button type="button" variant="outline" onClick={handleClose}>
                {t("dialog.close")}
              </Button>
              <Button
                type="button"
                onClick={() => {
                  setLastCreated(null);
                  reset();
                }}
              >
                {t("dialog.sendAnother")}
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label>{t("dialog.identityType")}</Label>
              <Select
                value={kind}
                onValueChange={(next) =>
                  setKind(next as InvitationIdentityKind)
                }
              >
                <SelectTrigger aria-label={t("dialog.identityType")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="email">{t("dialog.identityEmail")}</SelectItem>
                  <SelectItem value="im">{t("dialog.identityIM")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {kind === "email" ? (
              <div className="flex flex-col gap-2">
                <Label htmlFor="invite-email">{t("dialog.emailLabel")}</Label>
                <Input
                  id="invite-email"
                  type="email"
                  placeholder={t("dialog.emailPlaceholder")}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>
            ) : (
              <>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="invite-im-platform">{t("dialog.imPlatformLabel")}</Label>
                  <Input
                    id="invite-im-platform"
                    placeholder={t("dialog.imPlatformPlaceholder")}
                    value={imPlatform}
                    onChange={(e) => setImPlatform(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="invite-im-user-id">{t("dialog.imUserIdLabel")}</Label>
                  <Input
                    id="invite-im-user-id"
                    value={imUserId}
                    onChange={(e) => setImUserId(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="invite-im-display-name">{t("dialog.imDisplayNameLabel")}</Label>
                  <Input
                    id="invite-im-display-name"
                    value={imDisplayName}
                    onChange={(e) => setImDisplayName(e.target.value)}
                  />
                </div>
              </>
            )}

            <div className="flex flex-col gap-2">
              <Label htmlFor="invite-role">{t("dialog.roleLabel")}</Label>
              <Select value={projectRole} onValueChange={setProjectRole}>
                <SelectTrigger id="invite-role">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="owner">Owner</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                  <SelectItem value="editor">Editor</SelectItem>
                  <SelectItem value="viewer">Viewer</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="invite-message">{t("dialog.messageLabel")}</Label>
              <Textarea
                id="invite-message"
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                maxLength={1000}
              />
            </div>

            {error ? (
              <p className="text-sm text-destructive">{error}</p>
            ) : null}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={handleClose}
                disabled={submitting}
              >
                {tc("action.cancel")}
              </Button>
              <Button
                type="button"
                disabled={submitting}
                onClick={() => void handleSubmit()}
              >
                {submitting ? t("dialog.sending") : t("dialog.send")}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
