"use client";

import { useCallback, useState } from "react";
import { Plus, Save, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import {
  useIMStore,
  type IMChannel,
  type IMPlatform,
} from "@/lib/stores/im-store";

const PLATFORMS: { value: IMPlatform; label: string }[] = [
  { value: "feishu", label: "Feishu" },
  { value: "dingtalk", label: "DingTalk" },
  { value: "slack", label: "Slack" },
  { value: "telegram", label: "Telegram" },
  { value: "discord", label: "Discord" },
];

const EVENT_OPTIONS = [
  "task.created",
  "task.completed",
  "review.completed",
  "agent.started",
  "agent.completed",
  "budget.warning",
];

const EMPTY_FORM: Omit<IMChannel, "id"> & { id?: string } = {
  platform: "feishu",
  name: "",
  channelId: "",
  webhookUrl: "",
  events: [],
  active: true,
};

export function IMChannelConfig() {
  const channels = useIMStore((s) => s.channels);
  const loading = useIMStore((s) => s.loading);
  const saveChannel = useIMStore((s) => s.saveChannel);
  const deleteChannel = useIMStore((s) => s.deleteChannel);

  const [form, setForm] = useState<Omit<IMChannel, "id"> & { id?: string }>(
    EMPTY_FORM
  );
  const [editing, setEditing] = useState(false);

  const handleEdit = useCallback((channel: IMChannel) => {
    setForm(channel);
    setEditing(true);
  }, []);

  const handleNew = useCallback(() => {
    setForm(EMPTY_FORM);
    setEditing(true);
  }, []);

  const handleCancel = useCallback(() => {
    setForm(EMPTY_FORM);
    setEditing(false);
  }, []);

  const handleSave = useCallback(async () => {
    await saveChannel(form);
    setForm(EMPTY_FORM);
    setEditing(false);
  }, [form, saveChannel]);

  const handleDelete = useCallback(
    async (id: string) => {
      await deleteChannel(id);
      if (form.id === id) {
        setForm(EMPTY_FORM);
        setEditing(false);
      }
    },
    [deleteChannel, form.id]
  );

  const toggleEvent = useCallback((event: string) => {
    setForm((prev) => ({
      ...prev,
      events: prev.events.includes(event)
        ? prev.events.filter((e) => e !== event)
        : [...prev.events, event],
    }));
  }, []);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">IM Channels</h2>
        <Button size="sm" onClick={handleNew}>
          <Plus className="mr-1 size-3.5" />
          New Channel
        </Button>
      </div>

      {editing ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">
              {form.id ? "Edit Channel" : "New Channel"}
            </CardTitle>
            <CardDescription>
              Configure the IM channel connection and event subscriptions.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="im-platform">Platform</Label>
                <Select
                  value={form.platform}
                  onValueChange={(value: string) =>
                    setForm((prev) => ({
                      ...prev,
                      platform: value as IMPlatform,
                    }))
                  }
                >
                  <SelectTrigger id="im-platform" className="w-full">
                    <SelectValue placeholder="Select platform" />
                  </SelectTrigger>
                  <SelectContent>
                    {PLATFORMS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="im-name">Channel Name</Label>
                <Input
                  id="im-name"
                  value={form.name}
                  onChange={(e) =>
                    setForm((prev) => ({ ...prev, name: e.target.value }))
                  }
                  placeholder="e.g. #dev-notifications"
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="im-channel-id">Channel ID</Label>
                <Input
                  id="im-channel-id"
                  value={form.channelId}
                  onChange={(e) =>
                    setForm((prev) => ({ ...prev, channelId: e.target.value }))
                  }
                  placeholder="Platform-specific channel ID"
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="im-webhook">Webhook URL</Label>
                <Input
                  id="im-webhook"
                  value={form.webhookUrl}
                  onChange={(e) =>
                    setForm((prev) => ({
                      ...prev,
                      webhookUrl: e.target.value,
                    }))
                  }
                  placeholder="https://..."
                />
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <Label>Event Subscriptions</Label>
              <div className="flex flex-wrap gap-3">
                {EVENT_OPTIONS.map((event) => (
                  <label
                    key={event}
                    className="flex items-center gap-2 text-sm"
                  >
                    <input
                      type="checkbox"
                      checked={form.events.includes(event)}
                      onChange={() => toggleEvent(event)}
                      className="size-4 rounded border-input accent-primary"
                    />
                    {event}
                  </label>
                ))}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Label htmlFor="im-active">Active</Label>
              <button
                id="im-active"
                type="button"
                role="switch"
                aria-checked={form.active}
                onClick={() =>
                  setForm((prev) => ({ ...prev, active: !prev.active }))
                }
                className={cn(
                  "relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors",
                  form.active ? "bg-primary" : "bg-muted"
                )}
              >
                <span
                  className={cn(
                    "pointer-events-none block size-4 rounded-full bg-background shadow-sm transition-transform",
                    form.active ? "translate-x-4" : "translate-x-0"
                  )}
                />
              </button>
              <span className="text-sm text-muted-foreground">
                {form.active ? "Enabled" : "Disabled"}
              </span>
            </div>

            <div className="flex items-center gap-2">
              <Button size="sm" onClick={() => void handleSave()} disabled={loading}>
                <Save className="mr-1 size-3.5" />
                Save
              </Button>
              <Button variant="outline" size="sm" onClick={handleCancel}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {channels.length === 0 ? (
        <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
          {loading
            ? "Loading channels..."
            : "No IM channels configured. Click New Channel to get started."}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {channels.map((channel) => (
            <Card
              key={channel.id}
              className={cn(
                "cursor-pointer transition-colors hover:border-primary/40",
                form.id === channel.id && "border-primary"
              )}
              onClick={() => handleEdit(channel)}
            >
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between gap-2">
                  <CardTitle className="text-base">{channel.name}</CardTitle>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-xs capitalize">
                      {channel.platform}
                    </Badge>
                    <Badge
                      variant="secondary"
                      className={cn(
                        "text-xs",
                        channel.active
                          ? "bg-green-500/15 text-green-700 dark:text-green-400"
                          : "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400"
                      )}
                    >
                      {channel.active ? "Active" : "Inactive"}
                    </Badge>
                  </div>
                </div>
                <CardDescription className="text-xs">
                  {channel.channelId}
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-3">
                <div className="flex flex-wrap gap-1">
                  {channel.events.map((event) => (
                    <Badge key={event} variant="secondary" className="text-xs">
                      {event}
                    </Badge>
                  ))}
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleEdit(channel);
                    }}
                  >
                    Edit
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation();
                      void handleDelete(channel.id);
                    }}
                    disabled={loading}
                  >
                    <Trash2 className="mr-1 size-3.5" />
                    Delete
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
