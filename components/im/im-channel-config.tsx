"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Plus, Save, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EventBadgeList } from "@/components/shared/event-badge-list";
import {
  PLATFORM_DEFINITIONS,
  PlatformBadge,
} from "@/components/shared/platform-badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { useIMStore, type IMChannel, type IMPlatform } from "@/lib/stores/im-store";

function createEmptyForm(
  preferredPlatform?: string | null,
): Omit<IMChannel, "id"> & { id?: string } {
  const platform =
    preferredPlatform && preferredPlatform in PLATFORM_DEFINITIONS
      ? (preferredPlatform as IMPlatform)
      : "feishu";
  return {
    platform,
    name: "",
    channelId: "",
    webhookUrl: "",
    platformConfig: {},
    events: [],
    active: true,
  };
}

export function IMChannelConfig({
  preferredPlatform,
}: {
  preferredPlatform?: string | null;
}) {
  const t = useTranslations("im");
  const channels = useIMStore((s) => s.channels);
  const loading = useIMStore((s) => s.loading);
  const eventTypes = useIMStore((s) => s.eventTypes);
  const saveChannel = useIMStore((s) => s.saveChannel);
  const deleteChannel = useIMStore((s) => s.deleteChannel);
  const fetchEventTypes = useIMStore((s) => s.fetchEventTypes);

  const [form, setForm] = useState<Omit<IMChannel, "id"> & { id?: string }>(() =>
    createEmptyForm(preferredPlatform),
  );
  const [editing, setEditing] = useState(false);

  useEffect(() => {
    void fetchEventTypes();
  }, [fetchEventTypes]);

  const platformOptions = useMemo(
    () =>
      Object.entries(PLATFORM_DEFINITIONS).map(([value, definition]) => ({
        value: value as IMPlatform,
        label: definition.label,
      })),
    [],
  );

  const configFields = PLATFORM_DEFINITIONS[form.platform]?.configFields ?? [];
  const availableEventTypes = eventTypes.length > 0 ? eventTypes : [];

  const handleEdit = useCallback((channel: IMChannel) => {
    setForm({
      ...channel,
      platformConfig: channel.platformConfig ?? {},
    });
    setEditing(true);
  }, []);

  const handleNew = useCallback(() => {
    setForm(createEmptyForm(preferredPlatform));
    setEditing(true);
  }, [preferredPlatform]);

  const handleCancel = useCallback(() => {
    setForm(createEmptyForm(preferredPlatform));
    setEditing(false);
  }, [preferredPlatform]);

  const handleSave = useCallback(async () => {
    await saveChannel(form);
    setForm(createEmptyForm(preferredPlatform));
    setEditing(false);
  }, [form, preferredPlatform, saveChannel]);

  const handleDelete = useCallback(
    async (id: string) => {
      await deleteChannel(id);
      if (form.id === id) {
        setForm(createEmptyForm(preferredPlatform));
        setEditing(false);
      }
    },
    [deleteChannel, form.id, preferredPlatform],
  );

  const toggleEvent = useCallback((event: string) => {
    setForm((prev) => ({
      ...prev,
      events: prev.events.includes(event)
        ? prev.events.filter((entry) => entry !== event)
        : [...prev.events, event],
    }));
  }, []);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("channels.title")}</h2>
        <Button size="sm" onClick={handleNew}>
          <Plus className="mr-1 size-3.5" />
          {t("channels.newChannel")}
        </Button>
      </div>

      {editing ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">
              {form.id ? t("channels.editChannel") : t("channels.newChannelTitle")}
            </CardTitle>
            <CardDescription>{t("channels.channelDesc")}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="im-platform">{t("channels.platform")}</Label>
                <Select
                  value={form.platform}
                  onValueChange={(value: string) =>
                    setForm((prev) => ({
                      ...prev,
                      platform: value as IMPlatform,
                      platformConfig:
                        prev.platform === value ? prev.platformConfig : {},
                    }))
                  }
                >
                  <SelectTrigger id="im-platform" className="w-full">
                    <SelectValue placeholder={t("channels.platformPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {platformOptions.map((platform) => (
                      <SelectItem key={platform.value} value={platform.value}>
                        {platform.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="im-name">{t("channels.channelName")}</Label>
                <Input
                  id="im-name"
                  value={form.name}
                  onChange={(event) =>
                    setForm((prev) => ({ ...prev, name: event.target.value }))
                  }
                  placeholder={t("channels.channelNamePlaceholder")}
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="im-channel-id">{t("channels.channelId")}</Label>
                <Input
                  id="im-channel-id"
                  value={form.channelId}
                  onChange={(event) =>
                    setForm((prev) => ({ ...prev, channelId: event.target.value }))
                  }
                  placeholder={t("channels.channelIdPlaceholder")}
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="im-webhook">{t("channels.webhookUrl")}</Label>
                <Input
                  id="im-webhook"
                  value={form.webhookUrl}
                  onChange={(event) =>
                    setForm((prev) => ({ ...prev, webhookUrl: event.target.value }))
                  }
                  placeholder={t("channels.webhookUrlPlaceholder")}
                />
              </div>
            </div>

            {configFields.length > 0 ? (
              <div className="grid gap-4 md:grid-cols-2">
                {configFields.map((field) => (
                  <div key={field.key} className="flex flex-col gap-1.5">
                    <Label htmlFor={`im-platform-${field.key}`}>{field.label}</Label>
                    <Input
                      id={`im-platform-${field.key}`}
                      type={field.type ?? "text"}
                      value={form.platformConfig[field.key] ?? ""}
                      onChange={(event) =>
                        setForm((prev) => ({
                          ...prev,
                          platformConfig: {
                            ...prev.platformConfig,
                            [field.key]: event.target.value,
                          },
                        }))
                      }
                      placeholder={field.placeholder}
                    />
                  </div>
                ))}
              </div>
            ) : null}

            <div className="flex flex-col gap-2">
              <Label>{t("channels.eventSubscriptions")}</Label>
              <div className="flex flex-wrap gap-3">
                {availableEventTypes.map((event) => (
                  <div key={event} className="flex items-center gap-2 text-sm">
                    <Checkbox
                      id={`event-${event}`}
                      checked={form.events.includes(event)}
                      onCheckedChange={() => toggleEvent(event)}
                    />
                    <label htmlFor={`event-${event}`} className="cursor-pointer">
                      {t(`eventLabels.${event}`, { defaultValue: event })}
                    </label>
                  </div>
                ))}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Label htmlFor="im-active">{t("channels.active")}</Label>
              <Switch
                id="im-active"
                checked={form.active}
                onCheckedChange={(checked) =>
                  setForm((prev) => ({ ...prev, active: checked === true }))
                }
              />
              <span className="text-sm text-muted-foreground">
                {form.active ? t("channels.enabled") : t("channels.disabled")}
              </span>
            </div>

            <div className="flex items-center gap-2">
              <Button size="sm" onClick={() => void handleSave()} disabled={loading}>
                <Save className="mr-1 size-3.5" />
                {t("channels.save")}
              </Button>
              <Button variant="outline" size="sm" onClick={handleCancel}>
                {t("channels.cancel")}
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {channels.length === 0 ? (
        <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
          {loading ? t("channels.loadingChannels") : t("channels.noChannels")}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {channels.map((channel) => (
            <Card
              key={channel.id}
              className={cn(
                "cursor-pointer transition-colors hover:border-primary/40",
                form.id === channel.id && "border-primary",
              )}
              onClick={() => handleEdit(channel)}
            >
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between gap-2">
                  <CardTitle className="text-base">{channel.name}</CardTitle>
                  <div className="flex items-center gap-2">
                    <PlatformBadge platform={channel.platform} />
                    <Badge
                      variant="secondary"
                      className={cn(
                        "text-xs",
                        channel.active
                          ? "bg-green-500/15 text-green-700 dark:text-green-400"
                          : "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
                      )}
                    >
                      {channel.active ? t("channels.statusActive") : t("channels.statusInactive")}
                    </Badge>
                  </div>
                </div>
                <CardDescription className="text-xs">{channel.channelId}</CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-3">
                <EventBadgeList
                  events={channel.events}
                  getEventLabel={(event) =>
                    t(`eventLabels.${event}`, { defaultValue: event })
                  }
                />
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(event) => {
                      event.stopPropagation();
                      handleEdit(channel);
                    }}
                  >
                    {t("channels.edit")}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(event) => {
                      event.stopPropagation();
                      void handleDelete(channel.id);
                    }}
                    disabled={loading}
                  >
                    <Trash2 className="mr-1 size-3.5" />
                    {t("channels.delete")}
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
