"use client";

import { useCallback, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Plus } from "lucide-react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { describeCron } from "@/lib/cron-description";
import { validateCronExpression } from "@/lib/cron-validation";
import type { CreateSchedulerJobInput } from "@/lib/stores/scheduler-store";

interface SchedulerJobCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (input: CreateSchedulerJobInput) => Promise<boolean>;
  actionLoading: boolean;
}

const CRON_EXAMPLES = ["*/5 * * * *", "0 * * * *", "0 0 * * *", "0 12 * * 1-5"];

export function SchedulerJobCreateDialog(props: SchedulerJobCreateDialogProps) {
  // Remount the inner form whenever the dialog toggles open so state resets
  // without needing a synchronous setState-in-effect.
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      {props.open && <SchedulerJobCreateForm key={String(props.open)} {...props} />}
    </Dialog>
  );
}

function SchedulerJobCreateForm({
  onOpenChange,
  onCreate,
  actionLoading,
}: SchedulerJobCreateDialogProps) {
  const t = useTranslations("scheduler");
  const [jobKey, setJobKey] = useState("");
  const [name, setName] = useState("");
  const [schedule, setSchedule] = useState("*/15 * * * *");
  const [scope, setScope] = useState("system");
  const [cronTouched, setCronTouched] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const cronError = useMemo(() => validateCronExpression(schedule), [schedule]);
  const cronPreview = useMemo(
    () => (cronError ? "" : describeCron(schedule)),
    [schedule, cronError],
  );
  const jobKeyError =
    jobKey.trim() && !/^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(jobKey.trim())
      ? t("createDialog.invalidJobKey")
      : null;
  const nameError = !name.trim() ? t("createDialog.nameRequired") : null;
  const canSubmit =
    !actionLoading && !cronError && !jobKeyError && !nameError && jobKey.trim().length > 0;

  const handleSubmit = useCallback(async () => {
    setCronTouched(true);
    setSubmitError(null);
    if (!canSubmit) {
      return;
    }
    const ok = await onCreate({
      jobKey: jobKey.trim(),
      name: name.trim(),
      schedule: schedule.trim(),
      scope: scope.trim() || "system",
    });
    if (ok) {
      onOpenChange(false);
    } else {
      setSubmitError(t("createDialog.submitFailed"));
    }
  }, [canSubmit, jobKey, name, onCreate, onOpenChange, schedule, scope, t]);

  return (
    <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("createDialog.title")}</DialogTitle>
          <DialogDescription>{t("createDialog.description")}</DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="scheduler-job-key">{t("createDialog.jobKey")}</Label>
            <Input
              id="scheduler-job-key"
              value={jobKey}
              onChange={(event) => setJobKey(event.target.value)}
              placeholder={t("createDialog.jobKeyPlaceholder")}
              aria-invalid={jobKeyError ? true : undefined}
            />
            {jobKeyError && (
              <p className="text-xs text-destructive" role="alert">
                {jobKeyError}
              </p>
            )}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="scheduler-job-name">{t("createDialog.name")}</Label>
            <Input
              id="scheduler-job-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("createDialog.namePlaceholder")}
              aria-invalid={nameError ? true : undefined}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="scheduler-job-scope">{t("createDialog.scope")}</Label>
            <Input
              id="scheduler-job-scope"
              value={scope}
              onChange={(event) => setScope(event.target.value)}
              placeholder={t("createDialog.scopePlaceholder")}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="scheduler-job-schedule">{t("createDialog.schedule")}</Label>
            <Input
              id="scheduler-job-schedule"
              value={schedule}
              onChange={(event) => {
                setSchedule(event.target.value);
                setCronTouched(true);
              }}
              onBlur={() => setCronTouched(true)}
              aria-invalid={cronTouched && cronError ? true : undefined}
              className="font-mono text-sm"
            />
            {cronTouched && cronError ? (
              <p className="text-xs text-destructive" role="alert">
                {cronError}
              </p>
            ) : cronPreview ? (
              <p className="text-xs text-muted-foreground">{cronPreview}</p>
            ) : null}
            <div className="flex flex-wrap gap-1 pt-1">
              {CRON_EXAMPLES.map((example) => (
                <button
                  key={example}
                  type="button"
                  className="rounded-md border bg-muted/50 px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground hover:bg-muted"
                  onClick={() => {
                    setSchedule(example);
                    setCronTouched(false);
                  }}
                >
                  {example}
                </button>
              ))}
            </div>
          </div>

          {submitError && (
            <div
              role="alert"
              className="rounded-md border border-destructive/30 bg-destructive/5 p-2 text-xs text-destructive"
            >
              {submitError}
            </div>
          )}
        </div>

        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" size="sm">
              {t("createDialog.cancel")}
            </Button>
          </DialogClose>
          <Button
            size="sm"
            className="gap-1.5"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            <Plus className="size-3.5" />
            {t("createDialog.create")}
          </Button>
        </DialogFooter>
    </DialogContent>
  );
}
