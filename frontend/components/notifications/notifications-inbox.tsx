"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api, formatRelativeTime } from "@/lib/api";
import type { APIUserNotification } from "@/lib/api-types";

/**
 * NotificationsInbox (idx 92): the signed-in person's notifications as the API lists them,
 * their read state, and the global switch that pauses every room notification. Per-room
 * opt-ins live on each room page.
 */
export function NotificationsInbox() {
  const [items, setItems] = useState<APIUserNotification[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [setting, setSetting] = useState<"on" | "paused" | null>(null);

  const load = useCallback(async () => {
    try {
      const [list, settings] = await Promise.all([api.listNotifications(), api.getNotificationSettings()]);
      setItems(list.data ?? []);
      setSetting(settings.data.room_notifications);
      setFailed(false);
    } catch {
      setFailed(true);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const markRead = (id: string) => {
    setItems((prev) => prev?.map((n) => (n.id === id ? { ...n, read_at: n.read_at ?? new Date().toISOString() } : n)) ?? prev);
    void api.markNotificationRead(id).catch(() => undefined);
  };

  const markAll = async () => {
    try {
      await api.markAllNotificationsRead();
      await load();
    } catch {
      setFailed(true);
    }
  };

  const toggleRooms = async () => {
    if (!setting) return;
    try {
      const res = await api.updateNotificationSettings(setting === "on" ? "paused" : "on");
      setSetting(res.data.room_notifications);
    } catch {
      setFailed(true);
    }
  };

  if (failed && !items) {
    return <p role="alert" className="font-mono text-sm text-muted-foreground">Your notifications could not be read.</p>;
  }
  if (!items) {
    return <p className="font-mono text-sm text-muted-foreground">Loading…</p>;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={markAll}
          className="font-mono text-xs tracking-wider border border-border px-3 py-1.5 hover:bg-muted transition-colors"
        >
          Mark all read
        </button>
        {setting && (
          <button
            type="button"
            onClick={toggleRooms}
            className="font-mono text-xs tracking-wider border border-border px-3 py-1.5 hover:bg-muted transition-colors"
          >
            {setting === "on" ? "Pause room notifications" : "Resume room notifications"}
          </button>
        )}
        <p className="text-xs text-muted-foreground">
          Room notifications are opt-in per room: turn them on or off on each room page.
        </p>
      </div>

      {items.length === 0 ? (
        <p className="font-mono text-sm text-muted-foreground">No notifications yet.</p>
      ) : (
        <ul className="divide-y divide-border border-y border-border">
          {items.map((n) => (
            <li key={n.id} data-read={n.read_at ? "true" : "false"} className="py-3 space-y-1">
              <div className="flex items-center gap-2">
                {!n.read_at && <span aria-label="unread" className="w-1.5 h-1.5 bg-foreground inline-block" />}
                <p className="font-mono text-sm">{n.title}</p>
                <time dateTime={n.created_at} className="font-mono text-xs text-muted-foreground">
                  {formatRelativeTime(n.created_at)}
                </time>
              </div>
              {n.body && <p className="text-xs text-muted-foreground leading-relaxed">{n.body}</p>}
              {n.link && (
                <Link href={n.link} onClick={() => markRead(n.id)} className="font-mono text-xs underline underline-offset-4">
                  Open
                </Link>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
