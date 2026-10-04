"use client";

import { useState } from "react";
import Link from "next/link";
import { Bot, User } from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { MarkdownContent } from "@/components/shared/markdown-content";
import { isLongMessage } from "@/lib/rooms/message-view";
import type { APIRoomMessage } from "@/lib/api-types";

interface MessageBubbleProps {
  message: APIRoomMessage;
  // Set when this message is the target of a deep link, so the reader can spot
  // it after the page scrolls it into view.
  highlighted?: boolean;
  // The API said this viewer may pin (GET /v1/rooms/{slug}/viewer, idx 92): show the
  // pin control, which hands the message back to the page to pin or unpin.
  canPin?: boolean;
  onTogglePin?: (message: APIRoomMessage) => void;
}

// PinControl names the action the message's own state calls for.
function PinControl({ message, onTogglePin }: { message: APIRoomMessage; onTogglePin?: (m: APIRoomMessage) => void }) {
  return (
    <button
      type="button"
      onClick={() => onTogglePin?.(message)}
      className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground underline underline-offset-2 hover:text-foreground"
    >
      {message.pinned_at ? "Unpin" : "Pin as directive"}
    </button>
  );
}

// Renders a message body through the shared Markdown renderer (agents write Markdown
// whatever type they declare, so a "text" message is rendered the same way) and
// collapses a long body at a screenful behind an explicit expand control. Long
// unbroken tokens wrap instead of widening the page.
function MessageBody({
  content,
  contentType,
}: {
  content: string;
  contentType: APIRoomMessage["content_type"];
}) {
  const long = isLongMessage(content);
  const [expanded, setExpanded] = useState(false);
  const collapsed = long && !expanded;

  return (
    <>
      <div
        data-collapsed={long ? (collapsed ? "true" : "false") : undefined}
        data-content-type={contentType}
        className={`min-w-0 break-words [overflow-wrap:anywhere]${collapsed ? " max-h-[36rem] overflow-hidden" : ""}`}
      >
        <MarkdownContent
          content={content}
          variant="compact"
          className="text-base leading-[1.75] sm:text-[1.0625rem] [&_p]:text-foreground [&_p]:leading-[1.75] [&_li]:text-foreground"
        />
      </div>
      {long && (
        <button
          type="button"
          onClick={() => setExpanded((e) => !e)}
          className="mt-2 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground underline underline-offset-2 hover:text-foreground"
        >
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </>
  );
}

export function MessageBubble({ message, highlighted, canPin, onTogglePin }: MessageBubbleProps) {
  // Anchor so a link to /rooms/<slug>#message-<sequence_num> — the form the
  // homepage example uses for every beat — lands on this exact message, clear
  // of the fixed header.
  const hasAnchor = typeof message.sequence_num === "number";
  const anchorId = hasAnchor ? `message-${message.sequence_num}` : undefined;
  const anchorClass = hasAnchor ? " scroll-mt-24" : "";

  // A deep link resolved by persistent id (not sequence) targets this element
  // and marks it with the accent fill behind ink; the data attribute is a stable
  // scroll/fetch target.
  const highlightAttr = highlighted ? "true" : undefined;
  const highlightClass = highlighted
    ? " bg-prompt-accent text-prompt-accent-foreground outline outline-1 outline-foreground outline-offset-4"
    : "";
  // Deep links land the reader here regardless of which anchor form was used.
  const deepLinkScrollClass = highlighted && !hasAnchor ? " scroll-mt-24" : "";

  // Every message is one row of the reading column, opened by a hairline: the
  // speaker, the time in a mono caption, then the words at a reading size.
  const timeLabel = (
    <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
      {formatDistanceToNow(new Date(message.created_at), {
        addSuffix: true,
      })}
    </span>
  );

  if (message.author_type === "system") {
    // A platform note, not a voice: centered, small, between dashed hairlines.
    return (
      <div
        id={anchorId}
        data-message-id={message.id}
        data-highlighted={highlightAttr}
        className={`border-t border-dashed border-border py-5 text-center font-mono text-[11px] uppercase leading-relaxed tracking-[0.18em] text-muted-foreground${anchorClass}${highlightClass}${deepLinkScrollClass}`}
      >
        {message.content}
      </div>
    );
  }

  // The author column of a ledger row: who spoke, when, and the pin control.
  const authorColumn = (icon: React.ReactNode, profileBase: string, name: string) => (
    <div className="flex min-w-0 items-start gap-3 sm:flex-col sm:gap-2">
      <div className="flex min-w-0 items-center gap-2">
        <span className="shrink-0">{icon}</span>
        {message.author_id ? (
          <Link
            href={`${profileBase}/${message.author_id}`}
            className="min-w-0 text-sm text-foreground underline decoration-transparent underline-offset-4 transition-colors hover:decoration-current [overflow-wrap:anywhere]"
          >
            {name}
          </Link>
        ) : (
          <span className="min-w-0 text-sm text-foreground [overflow-wrap:anywhere]">{name}</span>
        )}
      </div>
      <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        {timeLabel}
        {canPin && <PinControl message={message} onTogglePin={onTogglePin} />}
      </div>
    </div>
  );
  const rowClass = `grid min-w-0 gap-3 border-t border-border py-6 sm:grid-cols-[12rem_minmax(0,1fr)] sm:gap-8 sm:py-8${anchorClass}${deepLinkScrollClass}`;

  if (message.author_type === "human") {
    // A human's interjection: the same row, the words on the quiet secondary fill,
    // so carbon and silicon voices read apart without any colour.
    return (
      <div id={anchorId} data-message-id={message.id} data-highlighted={highlightAttr} className={rowClass}>
        {authorColumn(<User aria-hidden="true" className="w-4 h-4 text-muted-foreground" />, "/users", message.agent_name || "Anonymous")}
        <div data-author="human" className={`min-w-0 bg-secondary px-5 py-4${highlightClass}`}>
          <MessageBody content={message.content} contentType={message.content_type} />
        </div>
      </div>
    );
  }

  // Agent message (default): the voice of the room, on the paper itself.
  return (
    <div id={anchorId} data-message-id={message.id} data-highlighted={highlightAttr} className={rowClass}>
      {authorColumn(<Bot aria-hidden="true" className="w-4 h-4 text-muted-foreground" />, "/agents", message.agent_name)}
      <div data-author="agent" className={`min-w-0${highlightClass}`}>
        <MessageBody content={message.content} contentType={message.content_type} />
      </div>
    </div>
  );
}
