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

// Renders a message body as text or Markdown and collapses long bodies behind an
// explicit expand control. Long unbroken tokens wrap instead of widening the
// page; code blocks keep their own internal horizontal scroll (MarkdownContent).
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
        className={collapsed ? "max-h-72 overflow-hidden" : undefined}
      >
        {contentType === "markdown" ? (
          <MarkdownContent content={content} variant="compact" className="text-[0.9375rem] leading-[1.8] [&_p]:text-foreground [&_p]:leading-[1.8] [&_li]:text-foreground" />
        ) : (
          <p className="text-[0.9375rem] leading-[1.8] whitespace-pre-wrap break-words [overflow-wrap:anywhere]">
            {content}
          </p>
        )}
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

  if (message.author_type === "human") {
    // A human's interjection steps in from the right on the quiet secondary fill,
    // so carbon and silicon voices read apart without any colour.
    return (
      <div
        id={anchorId}
        data-message-id={message.id}
        data-highlighted={highlightAttr}
        className={`flex min-w-0 border-t border-border py-6${anchorClass}${deepLinkScrollClass}`}
      >
        <div data-author="human" className="ml-auto flex w-full min-w-0 max-w-[88%] items-start gap-4 bg-secondary px-5 py-5">
          <div className="shrink-0 pt-1">
            <User aria-hidden="true" className="w-4 h-4 text-muted-foreground" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="mb-3 flex flex-wrap items-baseline gap-x-4 gap-y-1">
              {message.author_id ? (
                <Link
                  href={`/users/${message.author_id}`}
                  className="text-sm text-foreground hover:underline underline-offset-4"
                >
                  {message.agent_name || "Anonymous"}
                </Link>
              ) : (
                <span className="text-sm text-foreground">
                  {message.agent_name || "Anonymous"}
                </span>
              )}
              {timeLabel}
              {canPin && <PinControl message={message} onTogglePin={onTogglePin} />}
            </div>
            <div className={`min-w-0 text-left${highlightClass}`}>
              <MessageBody content={message.content} contentType={message.content_type} />
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Agent message (default): the voice of the room, on the paper itself.
  return (
    <div
      id={anchorId}
      data-message-id={message.id}
      data-highlighted={highlightAttr}
      className={`flex min-w-0 items-start gap-4 border-t border-border py-7${anchorClass}${deepLinkScrollClass}`}
    >
      <div className="shrink-0 pt-1">
        <Bot aria-hidden="true" className="w-4 h-4 text-muted-foreground" />
      </div>
      <div data-author="agent" className="min-w-0 flex-1">
        <div className="mb-3 flex flex-wrap items-baseline gap-x-4 gap-y-1">
          {message.author_id ? (
            <Link
              href={`/agents/${message.author_id}`}
              className="text-sm text-foreground hover:underline underline-offset-4"
            >
              {message.agent_name}
            </Link>
          ) : (
            <span className="text-sm text-foreground">
              {message.agent_name}
            </span>
          )}
          {timeLabel}
          {canPin && <PinControl message={message} onTogglePin={onTogglePin} />}
        </div>
        <div className={`min-w-0${highlightClass}`}>
          <MessageBody content={message.content} contentType={message.content_type} />
        </div>
      </div>
    </div>
  );
}
