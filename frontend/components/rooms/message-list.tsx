"use client";

import { useMemo } from "react";
import { MessageBubble } from "@/components/rooms/message-bubble";
import { mergeMessages } from "@/lib/rooms/message-view";
import type { APIRoomMessage } from "@/lib/api-types";

interface MessageListProps {
  messages: APIRoomMessage[];
  // Kept for the caller's contract; ordering/pagination now live in the parent
  // so this component stays a dumb renderer.
  slug: string;
  // Persistent id of a deep-linked message to visually highlight.
  highlightId?: number;
  // Earlier-history controls, driven by the parent (which owns the scroll box).
  hasOlder?: boolean;
  loadingOlder?: boolean;
  onLoadOlder?: () => void;
  // Pin control, shown only when the API said this viewer may pin (idx 92).
  canPin?: boolean;
  onTogglePin?: (message: APIRoomMessage) => void;
}

export function MessageList({
  messages,
  highlightId,
  hasOlder,
  loadingOlder,
  onLoadOlder,
  canPin,
  onTogglePin,
}: MessageListProps) {
  // Render oldest -> newest (top -> bottom) and drop duplicate ids so refreshed,
  // replayed, or locally echoed copies never appear twice — regardless of the
  // order the parent hands them in.
  const ordered = useMemo(() => mergeMessages(messages), [messages]);

  return (
    <div className="space-y-4 p-6">
      {hasOlder && (
        <div className="flex justify-center py-4">
          <button
            onClick={onLoadOlder}
            disabled={loadingOlder}
            className="font-mono text-xs tracking-wider border border-border px-6 py-2 hover:bg-foreground hover:text-background transition-colors disabled:opacity-50"
          >
            {loadingOlder ? "LOADING..." : "LOAD OLDER MESSAGES"}
          </button>
        </div>
      )}
      {ordered.map((msg) => (
        <MessageBubble
          key={msg.id}
          message={msg}
          highlighted={msg.id === highlightId}
          canPin={canPin}
          onTogglePin={onTogglePin}
        />
      ))}
    </div>
  );
}
