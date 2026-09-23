"use client";

import { useState } from "react";
import Link from "next/link";
import { MarkdownContent } from "@/components/shared/markdown-content";
import type { APIRoomMessage } from "@/lib/api-types";

interface RoomContextPanelProps {
  // The room's first message — the initial task. May be absent for a brand-new room.
  initialTask?: APIRoomMessage | null;
  // The most recently pinned directive/result. The API decides which pin is latest.
  latestPinned?: APIRoomMessage | null;
}

// A stable anchor into the transcript. MessageBubble tags each message with
// id="message-<sequence_num>", so linking here scrolls the reader to that exact
// message without leaving the room.
function messageAnchor(message: APIRoomMessage): string | undefined {
  return typeof message.sequence_num === "number"
    ? `#message-${message.sequence_num}`
    : undefined;
}

function ContextItem({
  label,
  message,
}: {
  label: string;
  message: APIRoomMessage;
}) {
  // Compact by default; the reader expands it IN PLACE (a button, never a
  // navigation) when they want the full text.
  const [expanded, setExpanded] = useState(false);
  const anchor = messageAnchor(message);

  return (
    <div className="border border-border bg-muted/30 px-3 py-2">
      <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground uppercase mb-1">
        {label}
      </p>
      <div className={expanded ? undefined : "max-h-20 overflow-hidden"}>
        {message.content_type === "markdown" ? (
          <MarkdownContent content={message.content} variant="compact" />
        ) : (
          <p className="text-sm leading-relaxed whitespace-pre-wrap break-words">
            {message.content}
          </p>
        )}
      </div>
      <div className="mt-2 flex items-center gap-4">
        <button
          type="button"
          onClick={() => setExpanded((e) => !e)}
          className="font-mono text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
        >
          {expanded ? "Collapse" : "Expand"}
        </button>
        {anchor && (
          <Link
            href={anchor}
            className="font-mono text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
          >
            View in conversation
          </Link>
        )}
      </div>
    </div>
  );
}

/**
 * RoomContextPanel is the room page's compact context area (task 33, step 4). It
 * keeps the INITIAL TASK and the LATEST PINNED DIRECTIVE in view so a reader does
 * not have to scroll a long transcript to find them, and lets each be expanded in
 * place. Both values are chosen by the API (first message, newest pin); the client
 * only renders them. When the room has neither, the panel renders nothing.
 */
export function RoomContextPanel({ initialTask, latestPinned }: RoomContextPanelProps) {
  if (!initialTask && !latestPinned) return null;

  return (
    <div data-testid="room-context" className="grid gap-2 sm:grid-cols-2 mb-4">
      {initialTask && <ContextItem label="Task" message={initialTask} />}
      {latestPinned && <ContextItem label="Pinned directive" message={latestPinned} />}
    </div>
  );
}
