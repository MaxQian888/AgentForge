"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface ReviewDecisionActionsProps {
  reviewId: string;
  onApprove?: (id: string, comment?: string) => void | Promise<void>;
  onRequestChanges?: (id: string, comment?: string) => void | Promise<void>;
  compact?: boolean;
}

type DecisionMode = "approve" | "request_changes" | null;

export function ReviewDecisionActions({
  reviewId,
  onApprove,
  onRequestChanges,
  compact = false,
}: ReviewDecisionActionsProps) {
  const t = useTranslations("reviews");
  const [mode, setMode] = useState<DecisionMode>(null);
  const [comment, setComment] = useState("");

  if (!onApprove && !onRequestChanges) {
    return null;
  }

  const buttonClassName = compact ? "h-6 text-xs" : undefined;
  const confirmButtonClassName = compact ? "h-7 text-xs" : undefined;

  const reset = () => {
    setMode(null);
    setComment("");
  };

  const handleConfirm = async () => {
    const normalizedComment = comment.trim() || undefined;

    if (mode === "approve") {
      await onApprove?.(reviewId, normalizedComment);
      reset();
      return;
    }

    if (mode === "request_changes") {
      await onRequestChanges?.(reviewId, normalizedComment);
      reset();
    }
  };

  const label =
    mode === "approve"
      ? t("approveCommentLabel")
      : t("requestChangesCommentLabel");
  const placeholder =
    mode === "approve"
      ? t("approveCommentPlaceholder")
      : t("requestChangesCommentPlaceholder");
  const confirmLabel =
    mode === "approve"
      ? t("confirmApprove")
      : t("confirmRequestChanges");

  return (
    <div className="flex flex-col gap-2">
      {!mode ? (
        <div className="flex items-center gap-1">
          {onApprove ? (
            <Button
              size="sm"
              variant="outline"
              className={buttonClassName}
              onClick={() => setMode("approve")}
            >
              {t("recommendationApprove")}
            </Button>
          ) : null}
          {onRequestChanges ? (
            <Button
              size="sm"
              variant="outline"
              className={buttonClassName}
              onClick={() => setMode("request_changes")}
            >
              {t("recommendationRequestChanges")}
            </Button>
          ) : null}
        </div>
      ) : (
        <div className="flex flex-col gap-2 rounded-md border p-3">
          <Label className="text-xs">{label}</Label>
          <Input
            value={comment}
            onChange={(event) => setComment(event.target.value)}
            placeholder={placeholder}
            className="h-8 text-sm"
          />
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              className={confirmButtonClassName}
              onClick={() => {
                void handleConfirm();
              }}
            >
              {confirmLabel}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className={confirmButtonClassName}
              onClick={reset}
            >
              {t("cancelTrigger")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
