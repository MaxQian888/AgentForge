"use client";

import { useEffect, useEffectEvent, useRef } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/layout/sidebar";
import { Header } from "@/components/layout/header";
import { resolveBackendUrl } from "@/lib/backend-url";
import { usePlatformCapability } from "@/hooks/use-platform-capability";
import type { NotificationDeliveryPolicy } from "@/lib/platform-runtime";
import { useAuthStore } from "@/lib/stores/auth-store";
import {
  useNotificationStore,
  type Notification,
} from "@/lib/stores/notification-store";
import { useWSStore } from "@/lib/stores/ws-store";

function resolveNotificationDeliveryPolicy(
  notification: Notification
): NotificationDeliveryPolicy {
  if (!notification.data) {
    return "always";
  }

  try {
    const parsed = JSON.parse(notification.data) as { deliveryPolicy?: unknown };
    return parsed.deliveryPolicy === "suppress_if_focused"
      ? "suppress_if_focused"
      : "always";
  } catch {
    return "always";
  }
}

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const accessToken = useAuthStore((s) => s.accessToken);
  const status = useAuthStore((s) => s.status);
  const hasHydrated = useAuthStore((s) => s.hasHydrated);
  const bootstrapSession = useAuthStore((s) => s.bootstrapSession);
  const fetchNotifications = useNotificationStore((s) => s.fetchNotifications);
  const notifications = useNotificationStore((s) => s.notifications);
  const unreadCount = useNotificationStore((s) => s.unreadCount);
  const connectWS = useWSStore((s) => s.connect);
  const disconnectWS = useWSStore((s) => s.disconnect);
  const { isDesktop, sendNotification, syncNotificationTraySummary } =
    usePlatformCapability();
  const deliveryLedgerRef = useRef<
    Map<string, "delivered" | "failed" | "suppressed">
  >(new Map());

  useEffect(() => {
    if (!hasHydrated) {
      return;
    }

    if (status === "idle") {
      void bootstrapSession();
      return;
    }

    if (status === "unauthenticated") {
      router.replace("/login");
    }
  }, [bootstrapSession, hasHydrated, router, status]);

  useEffect(() => {
    if (!hasHydrated || status !== "authenticated" || !accessToken) {
      disconnectWS();
      return;
    }

    let active = true;

    void resolveBackendUrl()
      .then((backendUrl) => {
        if (!active) {
          return;
        }
        connectWS(backendUrl.replace(/^http/, "ws") + "/ws", accessToken);
      })
      .catch(() => {
        if (active) {
          disconnectWS();
        }
      });

    return () => {
      active = false;
      disconnectWS();
    };
  }, [accessToken, connectWS, disconnectWS, hasHydrated, status]);

  useEffect(() => {
    if (!hasHydrated || status !== "authenticated" || !accessToken) {
      return;
    }

    void Promise.resolve(fetchNotifications()).catch(() => undefined);
  }, [accessToken, fetchNotifications, hasHydrated, status]);

  const bridgeDesktopNotifications = useEffectEvent(async () => {
    if (!hasHydrated || status !== "authenticated" || !accessToken || !isDesktop) {
      return;
    }

    const latestUnread = notifications.find((notification) => !notification.read) ?? null;
    await Promise.resolve(
      syncNotificationTraySummary({
        latestTitle: latestUnread?.title,
        unreadCount,
      })
    ).catch(() => undefined);

    for (const notification of notifications) {
      if (notification.read || deliveryLedgerRef.current.has(notification.id)) {
        continue;
      }

      const deliveryPolicy = resolveNotificationDeliveryPolicy(notification);
      const isFocusedSession =
        typeof document !== "undefined" && document.visibilityState === "visible";

      if (deliveryPolicy === "suppress_if_focused" && isFocusedSession) {
        deliveryLedgerRef.current.set(notification.id, "suppressed");
        continue;
      }

      const result = await Promise.resolve(
        sendNotification({
          notificationId: notification.id,
          type: notification.type,
          title: notification.title,
          body: notification.message,
          href: notification.href,
          createdAt: notification.createdAt,
          deliveryPolicy,
        })
      ).catch(() => ({ ok: false as const }));

      deliveryLedgerRef.current.set(
        notification.id,
        result.ok ? result.status : "failed"
      );
    }
  });

  useEffect(() => {
    void bridgeDesktopNotifications();
  }, [
    accessToken,
    hasHydrated,
    isDesktop,
    notifications,
    status,
    unreadCount,
  ]);

  if (!hasHydrated || status === "idle" || status === "checking") {
    return null;
  }

  if (status !== "authenticated") {
    return null;
  }

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header />
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
