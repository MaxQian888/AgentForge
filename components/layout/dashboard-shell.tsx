"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/layout/sidebar";
import { Header } from "@/components/layout/header";
import { resolveBackendUrl } from "@/lib/backend-url";
import { useAuthStore } from "@/lib/stores/auth-store";
import { useNotificationStore } from "@/lib/stores/notification-store";
import { useWSStore } from "@/lib/stores/ws-store";

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const accessToken = useAuthStore((s) => s.accessToken);
  const status = useAuthStore((s) => s.status);
  const hasHydrated = useAuthStore((s) => s.hasHydrated);
  const bootstrapSession = useAuthStore((s) => s.bootstrapSession);
  const fetchNotifications = useNotificationStore((s) => s.fetchNotifications);
  const connectWS = useWSStore((s) => s.connect);
  const disconnectWS = useWSStore((s) => s.disconnect);

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
