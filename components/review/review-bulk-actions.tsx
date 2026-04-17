"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";

export type ReviewBulkAction = "approve" | "reject" | "block";

interface ReviewBulkActionsProps {
  selectedCount: number;
  eligibleCount: number;
  onBulkApprove: () => void | Promise<void>;
  onBulkReject: (reason: string) => void | Promise<void>;
  onBulkBlock: (reason: string) => void | Promise<void>;
  onClearSelection: () => void;
}

export function ReviewBulkActions({
  selectedCount,
  eligibleCount,
  onBulkApprove,
  onBulkReject,
  onBulkBlock,
  onClearSelection,
}: ReviewBulkActionsProps) {
  const t = useTranslations("reviews");
  const [pending, setPending] = useState<ReviewBulkAction | null>(null);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string | null>(null);

  if (selectedCount === 0) {
    return null;
  }

  const skipped = Math.max(0, selectedCount - eligibleCount);
  const hasEligible = eligibleCount > 0;

  const closeDialog = () => {
    setPending(null);
    setReason("");
    setReasonError(null);
  };

  const handleApproveConfirm = async () => {
    await onBulkApprove();
    closeDialog();
  };

  const handleReasonConfirm = async () => {
    const trimmed = reason.trim();
    if (!trimmed) {
      setReasonError(
        pending === "block"
          ? t("blockReasonRequired")
          : t("rejectReasonRequired"),
      );
      return;
    }

    if (pending === "reject") {
      await onBulkReject(trimmed);
    } else if (pending === "block") {
      await onBulkBlock(trimmed);
    }
    closeDialog();
  };

  const isReasonDialog = pending === "reject" || pending === "block";
  const reasonTitle =
    pending === "reject"
      ? t("bulkConfirmRejectTitle", { count: selectedCount })
      : pending === "block"
      ? t("bulkConfirmBlockTitle", { count: selectedCount })
      : "";
  const reasonDescription =
    pending === "reject"
      ? t("bulkConfirmRejectDescription")
      : pending === "block"
      ? t("bulkConfirmBlockDescription")
      : "";
  const reasonLabel =
    pending === "block" ? t("blockCommentLabel") : t("rejectCommentLabel");
  const reasonConfirmLabel =
    pending === "block" ? t("confirmBlock") : t("confirmReject");

  return (
    <div
      data-testid="review-bulk-actions"
      className="flex flex-wrap items-center gap-2 rounded-lg border bg-muted/50 px-4 py-2"
    >
      <span className="text-sm font-medium">
        {t("bulkSelected", { count: selectedCount })}
      </span>

      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() => setPending("approve")}
        disabled={!hasEligible}
      >
        {t("bulkApprove")}
      </Button>

      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() => setPending("reject")}
        disabled={!hasEligible}
      >
        {t("bulkReject")}
      </Button>

      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() => setPending("block")}
        disabled={!hasEligible}
      >
        {t("bulkBlock")}
      </Button>

      <Button
        type="button"
        size="sm"
        variant="ghost"
        onClick={onClearSelection}
      >
        {t("bulkClear")}
      </Button>

      {!hasEligible ? (
        <span className="text-xs text-muted-foreground">
          {t("bulkNoEligible")}
        </span>
      ) : null}

      <ConfirmDialog
        open={pending === "approve"}
        title={t("bulkConfirmApproveTitle", { count: selectedCount })}
        description={
          <div className="space-y-2">
            <p>{t("bulkConfirmApproveDescription")}</p>
            {skipped > 0 ? (
              <p className="text-xs text-amber-700 dark:text-amber-400">
                {t("bulkPartialSkipped", {
                  skipped,
                  count: selectedCount,
                })}
              </p>
            ) : null}
          </div>
        }
        confirmLabel={t("confirmApprove")}
        onConfirm={() => {
          void handleApproveConfirm();
        }}
        onCancel={closeDialog}
      />

      <Dialog
        open={isReasonDialog}
        onOpenChange={(open) => {
          if (!open) closeDialog();
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{reasonTitle}</DialogTitle>
            <DialogDescription>{reasonDescription}</DialogDescription>
          </DialogHeader>
          {skipped > 0 ? (
            <p className="text-xs text-amber-700 dark:text-amber-400">
              {t("bulkPartialSkipped", {
                skipped,
                count: selectedCount,
              })}
            </p>
          ) : null}
          <div className="space-y-1">
            <Label className="text-xs">{reasonLabel}</Label>
            <Input
              value={reason}
              onChange={(event) => {
                setReason(event.target.value);
                if (reasonError) {
                  setReasonError(null);
                }
              }}
              placeholder={t("bulkReasonPlaceholder")}
              className="h-8 text-sm"
            />
            {reasonError ? (
              <p
                data-testid="review-bulk-reason-error"
                className="text-xs text-red-600 dark:text-red-400"
              >
                {reasonError}
              </p>
            ) : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeDialog}>
              {t("cancelTrigger")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => {
                void handleReasonConfirm();
              }}
            >
              {reasonConfirmLabel}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
