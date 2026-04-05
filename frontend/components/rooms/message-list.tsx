"use client";

import { useState, useRef } from "react";
import { MessageBubble } from "@/components/rooms/message-bubble";
import { api } from "@/lib/api";
import type { APIRoomMessage } from "@/lib/api-types";

interface MessageListProps {
  messages: APIRoomMessage[];
  slug: string;
  onMessagesLoaded?: (msgs: APIRoomMessage[]) => void;
}

export function MessageList({
  messages: initialMessages,
  slug,
  onMessagesLoaded,
}: MessageListProps) {
  const [messages, setMessages] = useState<APIRoomMessage[]>(initialMessages);
  const [loadingOlder, setLoadingOlder] = useState(false);
  // Assume there are older messages unless a batch returns fewer than 50
  const [hasOlder, setHasOlder] = useState(initialMessages.length >= 50);
  const bottomRef = useRef<HTMLDivElement>(null);

  async function loadOlder() {
    if (loadingOlder || messages.length === 0) return;

    setLoadingOlder(true);
    try {
      const oldestId = Math.min(...messages.map((m) => m.id));
      const response = await api.fetchRoomMessages(slug, oldestId, 50);
      const olderMessages = response.data;

      if (olderMessages.length < 50) {
        setHasOlder(false);
      }

      if (olderMessages.length > 0) {
        // API returns newest first; reverse to get oldest-first for prepending
        const reversed = [...olderMessages].reverse();
        const merged = [...reversed, ...messages];
        setMessages(merged);
        onMessagesLoaded?.(merged);
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
      {messages.map((msg) => (
        <MessageBubble key={msg.id} message={msg} />
      ))}
      <div ref={bottomRef} />
    </div>
  );
}
