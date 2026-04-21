import {
  Bot,
  Building2,
  Hash,
  Mail,
  MessageCircleMore,
  MessagesSquare,
  Send,
  Speech,
  Webhook,
  type LucideIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { IMPlatform } from "@/lib/stores/im-store";

export type PlatformConfigField = {
  key: string;
  label: string;
  placeholder?: string;
  type?: "text" | "password" | "url";
};

type PlatformDefinition = {
  label: string;
  icon: LucideIcon;
  configFields: PlatformConfigField[];
};

const GENERIC_PLATFORM: PlatformDefinition = {
  label: "Unknown",
  icon: Webhook,
  configFields: [],
};

export const PLATFORM_DEFINITIONS: Record<IMPlatform, PlatformDefinition> = {
  feishu: {
    label: "Feishu",
    icon: MessageCircleMore,
    configFields: [],
  },
  dingtalk: {
    label: "DingTalk",
    icon: MessagesSquare,
    configFields: [],
  },
  slack: {
    label: "Slack",
    icon: Hash,
    configFields: [],
  },
  telegram: {
    label: "Telegram",
    icon: Send,
    configFields: [],
  },
  discord: {
    label: "Discord",
    icon: Speech,
    configFields: [],
  },
  wecom: {
    label: "WeCom",
    icon: Building2,
    configFields: [
      { key: "corpId", label: "Corp ID", placeholder: "ww1234567890" },
      { key: "agentId", label: "Agent ID", placeholder: "1000002" },
      {
        key: "callbackToken",
        label: "Callback Token",
        placeholder: "wecom-callback-token",
        type: "password",
      },
    ],
  },
  wechat: {
    label: "WeChat (iLinks)",
    icon: MessageCircleMore,
    configFields: [
      { key: "appId", label: "App ID", placeholder: "wx1234567890" },
      {
        key: "appSecret",
        label: "App Secret",
        placeholder: "wechat-app-secret",
        type: "password",
      },
      {
        key: "callbackToken",
        label: "Callback Token",
        placeholder: "wechat-callback-token",
        type: "password",
      },
    ],
  },
  qq: {
    label: "QQ",
    icon: MessageCircleMore,
    configFields: [
      {
        key: "onebotEndpoint",
        label: "OneBot Endpoint",
        placeholder: "ws://localhost:6700",
        type: "url",
      },
      {
        key: "accessToken",
        label: "Access Token",
        placeholder: "onebot-access-token",
        type: "password",
      },
    ],
  },
  qqbot: {
    label: "QQ Bot",
    icon: Bot,
    configFields: [
      { key: "appId", label: "App ID", placeholder: "1024" },
      {
        key: "appSecret",
        label: "App Secret",
        placeholder: "qqbot-app-secret",
        type: "password",
      },
    ],
  },
  email: {
    label: "Email",
    icon: Mail,
    configFields: [
      {
        key: "smtpHost",
        label: "SMTP Host",
        placeholder: "smtp.example.com",
      },
      { key: "smtpPort", label: "SMTP Port", placeholder: "587" },
      {
        key: "fromAddress",
        label: "From Address",
        placeholder: "noreply@example.com",
      },
      { key: "smtpTls", label: "TLS", placeholder: "true" },
    ],
  },
};

type PlatformBadgeProps = {
  platform: IMPlatform | string;
  className?: string;
};

export function PlatformBadge({ platform, className }: PlatformBadgeProps) {
  const definition =
    PLATFORM_DEFINITIONS[platform as IMPlatform] ?? GENERIC_PLATFORM;
  const Icon = definition.icon;

  return (
    <Badge
      variant="outline"
      className={cn("gap-1.5 text-xs", className)}
      data-platform={platform}
      data-testid={`platform-badge-${platform}`}
    >
      <Icon className="size-3.5" />
      <span>{definition.label}</span>
    </Badge>
  );
}
