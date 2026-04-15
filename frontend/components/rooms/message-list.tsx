"use client";

import { useState, useRef, useMemo } from "react";
import { MessageBubble } from "@/components/rooms/message-bubble";
import { api } from "@/lib/api";
import type { APIRoomMessage } from "@/lib/api-types";

interface MessageListProps {
  messages: APIRoomMessage[];
  slug: string;
  onMessagesLoaded?: (msgs: APIRoomMessage[]) => void;
}

export function MessageList({
  messages,
  slug,
  onMessagesLoaded,
}: MessageListProps) {
  // Older messages fetched via "LOAD OLDER" are kept locally and merged with the
  // live `messages` prop. The prop itself is the source of truth for new arrivals,
  // so SSE updates from the parent flow through without being shadowed.
  const [olderMessages, setOlderMessages] = useState<APIRoomMessage[]>([]);
  const [loadingOlder, setLoadingOlder] = useState(false);
  // Assume there are older messages unless a batch returns fewer than 50
  const [hasOlder, setHasOlder] = useState(messages.length >= 50);
  const bottomRef = useRef<HTMLDivElement>(null);

  const allMessages = useMemo(
    () => (olderMessages.length > 0 ? [...olderMessages, ...messages] : messages),
    [olderMessages, messages],
  );

  async function loadOlder() {
    if (loadingOlder || allMessages.length === 0) return;

    setLoadingOlder(true);
    try {
      const oldestId = Math.min(...allMessages.map((m) => m.id));
      const response = await api.fetchRoomMessages(slug, oldestId, 50);
      const olderBatch = response.data;

      if (olderBatch.length < 50) {
        setHasOlder(false);
      }

      if (olderBatch.length > 0) {
        // API returns newest first; reverse so older-than-oldest is prepended
        const reversed = [...olderBatch].reverse();
        setOlderMessages((prev) => [...reversed, ...prev]);
        onMessagesLoaded?.([...reversed, ...allMessages]);
      }
    } catch {
      // Silent failure — user can retry
    } finally {
      setLoadingOlder(false);
    }
  }

  return (
    <div className="space-y-4 p-6">
      {hasOlder && (
        <div className="flex justify-center py-4">
          <button
            onClick={loadOlder}
            disabled={loadingOlder}
            className="font-mono text-xs tracking-wider border border-border px-6 py-2 hover:bg-foreground hover:text-background transition-colors disabled:opacity-50"
          >
            {loadingOlder ? "LOADING..." : "LOAD OLDER MESSAGES"}
          </button>
        </div>
      )}
      {allMessages.map((msg) => (
        <MessageBubble key={msg.id} message={msg} />
      ))}
      <div ref={bottomRef} />
    </div>
  );
}
