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
      className="font-mono text-[10px] tracking-wider text-muted-foreground underline underline-offset-2 hover:text-foreground"
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
          <MarkdownContent content={content} variant="compact" />
        ) : (
          <p className="text-sm leading-relaxed whitespace-pre-wrap break-words">
            {content}
          </p>
        )}
      </div>
      {long && (
        <button
          type="button"
          onClick={() => setExpanded((e) => !e)}
          className="mt-2 font-mono text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
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
  // and rings it briefly; the data attribute is a stable scroll/fetch target.
  const highlightAttr = highlighted ? "true" : undefined;
  const highlightClass = highlighted
    ? " ring-2 ring-green-500 ring-offset-2 ring-offset-background rounded-lg"
    : "";
  // Deep links land the reader here regardless of which anchor form was used.
  const deepLinkScrollClass = highlighted && !hasAnchor ? " scroll-mt-24" : "";

  if (message.author_type === "system") {
    return (
      <div
        id={anchorId}
        data-message-id={message.id}
        data-highlighted={highlightAttr}
        className={`text-center text-xs text-muted-foreground font-mono py-2 px-4 border-y border-dashed border-border/50${anchorClass}${highlightClass}${deepLinkScrollClass}`}
      >
        {message.content}
      </div>
    );
  }

  if (message.author_type === "human") {
    return (
      <div
        id={anchorId}
        data-message-id={message.id}
        data-highlighted={highlightAttr}
        className={`flex items-start gap-3 max-w-[70%] ml-auto flex-row-reverse${anchorClass}${deepLinkScrollClass}`}
      >
        <div className="shrink-0 mt-1">
          <User className="w-4 h-4 text-muted-foreground" />
        </div>
        <div className="text-right">
          <div className="flex items-center gap-2 mb-1 justify-end">
            {message.author_id ? (
              <Link
                href={`/users/${message.author_id}`}
                className="font-mono text-xs text-muted-foreground hover:underline"
              >
                {message.agent_name || "Anonymous"}
              </Link>
            ) : (
              <span className="font-mono text-xs text-muted-foreground">
                {message.agent_name || "Anonymous"}
              </span>
            )}
            <span className="font-mono text-xs text-muted-foreground">
              {formatDistanceToNow(new Date(message.created_at), {
                addSuffix: true,
              })}
            </span>
            {canPin && <PinControl message={message} onTogglePin={onTogglePin} />}
          </div>
          <div
            className={`bg-green-50 dark:bg-green-950/30 border border-green-100 dark:border-green-900 rounded-lg p-3 text-left${highlightClass}`}
          >
            <MessageBody content={message.content} contentType={message.content_type} />
          </div>
        </div>
      </div>
    );
  }

  // Agent message (default)
  return (
    <div
      id={anchorId}
      data-message-id={message.id}
      data-highlighted={highlightAttr}
      className={`flex items-start gap-3 max-w-[70%]${anchorClass}${deepLinkScrollClass}`}
    >
      <div className="shrink-0 mt-1">
        <Bot className="w-4 h-4 text-muted-foreground" />
      </div>
      <div className="min-w-0">
        <div className="flex items-center gap-2 mb-1">
          {message.author_id ? (
            <Link
              href={`/agents/${message.author_id}`}
              className="font-mono text-xs text-muted-foreground hover:underline"
            >
              {message.agent_name}
            </Link>
          ) : (
            <span className="font-mono text-xs text-muted-foreground">
              {message.agent_name}
            </span>
          )}
          <span className="font-mono text-xs text-muted-foreground">
            {formatDistanceToNow(new Date(message.created_at), {
              addSuffix: true,
            })}
          </span>
          {canPin && <PinControl message={message} onTogglePin={onTogglePin} />}
        </div>
        <div
          className={`bg-blue-50 dark:bg-blue-950/30 border border-blue-100 dark:border-blue-900 rounded-lg p-3${highlightClass}`}
        >
          <MessageBody content={message.content} contentType={message.content_type} />
        </div>
      </div>
    </div>
  );
}
