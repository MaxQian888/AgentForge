---
title: "Spec 3: Notification Center UI"
date: 2026-04-23
status: draft
---

# Notification Center UI

## Problem

The backend notification system is 70% complete — `notification_service.go` supports multi-channel delivery, real-time push via WebSocket, and read/unread tracking. But there is no frontend notification inbox. Users cannot view, manage, or configure their notifications.

## Current State

**Backend (complete):**
- `notification_service.go` — creation, delivery, read tracking
- Channels: in_app, feishu, slack, im
- Types: tasks, agents, reviews, automation, budgets
- WebSocket fanout for real-time delivery
- `notification-store.ts` — Zustand store with fetch/mark-read/bulk-read

**Frontend (missing):**
- No notification bell icon in the header
- No notification inbox/drawer component
- No notification preferences UI
- No rich notification templates (basic title/body only)

## Design

### UI Architecture

**Notification Bell** — added to the main app header bar:
- Bell icon with unread count badge
- Click opens notification drawer (slide-in from right)
- Pulse animation on new notification

**Notification Drawer** — slide-in panel:
```
┌─────────────────────────────────┐
│ Notifications        [Mark All] │
├─────────────────────────────────┤
│ [All] [Unread] [Mentions]       │
├─────────────────────────────────┤
│ ● Task assigned to you          │
│   "Implement auth flow"         │
│   Project: AgentForge · 5m ago  │
├─────────────────────────────────┤
│   Agent completed task          │
│   "Fix login bug" · 1h ago     │
├─────────────────────────────────┤
│ ● Review requested             │
│   PR #42 needs your review     │
│   3 findings · 2h ago          │
├─────────────────────────────────┤
│         [Load More]             │
└─────────────────────────────────┘
```

**Notification Preferences** — new tab in Settings:
- Per-type toggle (task updates, agent events, review requests, budget alerts, etc.)
- Per-channel preference (in-app always on; IM/email configurable)
- Quiet hours (suppress non-urgent notifications during specified hours)
- Digest mode (batch notifications into hourly/daily summary)

### Notification Types & Templates

Expand the existing type system with rich templates:

```typescript
interface NotificationTemplate {
  type: string;
  icon: string;          // lucide icon name
  color: string;         // semantic color (success/warning/danger/info)
  titleTemplate: string; // e.g. "Task assigned: {taskTitle}"
  bodyTemplate: string;  // e.g. "{assignerName} assigned you a task in {projectName}"
  actionUrl: string;     // deep link, e.g. "/projects/{projectId}/tasks/{taskId}"
  priority: "urgent" | "normal" | "low";
}
```

Templates for key notification types:

| Type | Icon | Priority | Action |
|------|------|----------|--------|
| `task.assigned` | UserPlus | normal | Navigate to task |
| `task.status_changed` | ArrowRight | normal | Navigate to task |
| `task.comment` | MessageSquare | normal | Navigate to comment |
| `task.overdue` | AlertTriangle | urgent | Navigate to task |
| `agent.spawned` | Bot | low | Navigate to agent |
| `agent.completed` | CheckCircle | normal | Navigate to agent run |
| `agent.failed` | XCircle | urgent | Navigate to agent logs |
| `review.requested` | Eye | normal | Navigate to review |
| `review.approved` | ThumbsUp | normal | Navigate to review |
| `review.rejected` | ThumbsDown | urgent | Navigate to review |
| `budget.warning` | AlertTriangle | urgent | Navigate to cost page |
| `budget.exceeded` | AlertOctagon | urgent | Navigate to cost page |
| `automation.triggered` | Zap | low | Navigate to automation log |
| `invitation.received` | Mail | normal | Navigate to invitation |
| `mention` | AtSign | normal | Navigate to context |

### Real-Time Updates

WebSocket already delivers notifications. The frontend needs:

1. **`notification-store.ts` enhancement:**
   - `subscribeToLive()` — listens to WebSocket `notification` events
   - `addNotification()` — prepends to list, increments unread count
   - `markRead(id)` — marks single notification, decrements unread
   - `markAllRead()` — bulk update
   - `fetchPage(cursor)` — paginated loading

2. **Browser Notification API:**
   - When a notification arrives via WebSocket and the app is in background
   - Request browser notification permission on first login
   - Show native OS notification with title/body
   - Click on native notification brings app to foreground and navigates to action URL

### Notification Preferences API

Extend the existing notification backend:

```
GET    /api/v1/notifications/preferences        — Get user preferences
PUT    /api/v1/notifications/preferences        — Update preferences

GET    /api/v1/notifications                    — List (paginated, filterable)
PUT    /api/v1/notifications/:id/read           — Mark read
PUT    /api/v1/notifications/read-all           — Mark all read
DELETE /api/v1/notifications/:id                — Dismiss
```

Preferences data model:

```typescript
interface NotificationPreferences {
  types: {
    [type: string]: {
      enabled: boolean;
      channels: ("in_app" | "im" | "email")[];
    };
  };
  quietHours: {
    enabled: boolean;
    start: string; // "22:00"
    end: string;   // "08:00"
    timezone: string;
  };
  digest: "none" | "hourly" | "daily";
}
```

### Frontend Components

**New files:**
- `components/notifications/notification-bell.tsx` — header bell with badge
- `components/notifications/notification-drawer.tsx` — slide-in panel
- `components/notifications/notification-item.tsx` — single notification card
- `components/notifications/notification-preferences.tsx` — settings tab
- `lib/stores/notification-preferences-store.ts` — preferences state

**Modified files:**
- `app/(dashboard)/layout.tsx` — add notification bell to header
- `app/(dashboard)/settings/page.tsx` — add notifications tab
- `lib/stores/notification-store.ts` — add live subscription, pagination

### Testing

- Unit: notification-store state transitions, preference management
- Integration: WebSocket → notification arrives → badge updates → mark read
- E2E: assign task → notification appears → click navigates to task → mark read
- Accessibility: keyboard navigation in drawer, screen reader announcements
