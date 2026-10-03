"use client";

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';

// What the caller may do in this room, as the API decides it (GET /v1/rooms/{slug}/viewer,
// sent with the caller's own credential). Until it answers — or when it cannot — nothing
// extra is offered.
export function useRoomViewer(slug: string) {
  const [canPin, setCanPin] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const read = api.getRoomViewer?.(slug);
    if (!read) return;
    read
      .then((res) => {
        if (!cancelled) setCanPin(Boolean(res?.data?.can_pin));
      })
      .catch(() => {
        if (!cancelled) setCanPin(false);
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  return { canPin };
}
