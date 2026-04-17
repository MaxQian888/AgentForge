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
  onReject?: (id: string, reason: string, comment?: string) => void | Promise<void>;
  onBlock?: (id: string, reason: string, comment?: string) => void | Promise<void>;
  compact?: boolean;
}

type DecisionMode = "approve" | "request_changes" | "reject" | "block" | null;

export function ReviewDecisionActions({
  reviewId,
  onApprove,
  onRequestChanges,
  onReject,
  onBlock,
  compact = false,
}: ReviewDecisionActionsProps) {
  const t = useTranslations("reviews");
  const [mode, setMode] = useState<DecisionMode>(null);
  const [comment, setComment] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);

  if (!onApprove && !onRequestChanges && !onReject && !onBlock) {
    return null;
  }

  const buttonClassName = compact ? "h-6 text-xs" : undefined;
  const confirmButtonClassName = compact ? "h-7 text-xs" : undefined;

  const reset = () => {
    setMode(null);
    setComment("");
    setValidationError(null);
  };

  const handleConfirm = async () => {
    const trimmed = comment.trim();
    const normalizedComment = trimmed || undefined;

    if (mode === "approve") {
      await onApprove?.(reviewId, normalizedComment);
      reset();
      return;
    }

    if (mode === "request_changes") {
      await onRequestChanges?.(reviewId, normalizedComment);
      reset();
      return;
    }

    if (mode === "reject") {
      if (!trimmed) {
        setValidationError(t("rejectReasonRequired"));
        return;
      }
      await onReject?.(reviewId, trimmed, normalizedComment);
      reset();
      return;
    }

    if (mode === "block") {
      if (!trimmed) {
        setValidationError(t("blockReasonRequired"));
        return;
      }
      await onBlock?.(reviewId, trimmed, normalizedComment);
      reset();
    }
  };

  const label =
    mode === "approve"
      ? t("approveCommentLabel")
      : mode === "request_changes"
      ? t("requestChangesCommentLabel")
      : mode === "reject"
      ? t("rejectCommentLabel")
      : t("blockCommentLabel");
  const placeholder =
    mode === "approve"
      ? t("approveCommentPlaceholder")
      : mode === "request_changes"
      ? t("requestChangesCommentPlaceholder")
      : mode === "reject"
      ? t("rejectCommentPlaceholder")
      : t("blockCommentPlaceholder");
  const confirmLabel =
    mode === "approve"
      ? t("confirmApprove")
      : mode === "request_changes"
      ? t("confirmRequestChanges")
      : mode === "reject"
      ? t("confirmReject")
      : t("confirmBlock");

  return (
    <div className="flex flex-col gap-2">
      {!mode ? (
        <div className="flex flex-wrap items-center gap-1">
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
          {onReject ? (
            <Button
              size="sm"
              variant="outline"
              className={buttonClassName}
              onClick={() => setMode("reject")}
            >
              {t("rejectReview")}
            </Button>
          ) : null}
          {onBlock ? (
            <Button
              size="sm"
              variant="outline"
              className={buttonClassName}
              onClick={() => setMode("block")}
            >
              {t("blockReview")}
            </Button>
          ) : null}
        </div>
      ) : (
        <div className="flex flex-col gap-2 rounded-md border p-3">
          <Label className="text-xs">{label}</Label>
          <Input
            value={comment}
            onChange={(event) => {
              setComment(event.target.value);
              if (validationError) {
                setValidationError(null);
              }
            }}
            placeholder={placeholder}
            className="h-8 text-sm"
          />
          {validationError ? (
            <p
              data-testid="review-decision-validation-error"
              className="text-xs text-red-600 dark:text-red-400"
            >
              {validationError}
            </p>
          ) : null}
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
