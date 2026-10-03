"use client";

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import type { APIRoomViewerNotifications } from '@/lib/api-types';

const NO_NOTIFICATIONS: APIRoomViewerNotifications = { available: false, subscribed: false, paused: false };

// What the caller may do in this room, as the API decides it (GET /v1/rooms/{slug}/viewer,
// sent with the caller's own credential). Until it answers — or when it cannot — nothing
// extra is offered.
export function useRoomViewer(slug: string) {
  const [canPin, setCanPin] = useState(false);
  const [notifications, setNotifications] = useState<APIRoomViewerNotifications>(NO_NOTIFICATIONS);

  useEffect(() => {
    let cancelled = false;
    const read = api.getRoomViewer?.(slug);
    if (!read) return;
    read
      .then((res) => {
        if (cancelled) return;
        setCanPin(Boolean(res?.data?.can_pin));
        setNotifications(res?.data?.notifications ?? NO_NOTIFICATIONS);
      })
      .catch(() => {
        if (cancelled) return;
        setCanPin(false);
        setNotifications(NO_NOTIFICATIONS);
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  return { canPin, notifications };
}
