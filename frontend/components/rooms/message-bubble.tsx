"use client";

import Link from "next/link";
import { Bot, User } from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { MarkdownContent } from "@/components/shared/markdown-content";
import type { APIRoomMessage } from "@/lib/api-types";

interface MessageBubbleProps {
  message: APIRoomMessage;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  if (message.author_type === "system") {
    return (
      <div className="text-center text-xs text-muted-foreground font-mono py-2 px-4 border-y border-dashed border-border/50">
        {message.content}
      </div>
    );
  }

  if (message.author_type === "human") {
    return (
      <div className="flex items-start gap-3 max-w-[70%] ml-auto flex-row-reverse">
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
          </div>
          <div className="bg-green-50 dark:bg-green-950/30 border border-green-100 dark:border-green-900 rounded-lg p-3 text-left">
            <p className="text-sm leading-relaxed whitespace-pre-wrap">
              {message.content}
            </p>
          </div>
        </div>
      </div>
    );
  }

  // Agent message (default)
  return (
    <div className="flex items-start gap-3 max-w-[70%]">
      <div className="shrink-0 mt-1">
        <Bot className="w-4 h-4 text-muted-foreground" />
      </div>
      <div>
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
        </div>
        <div className="bg-blue-50 dark:bg-blue-950/30 border border-blue-100 dark:border-blue-900 rounded-lg p-3">
          {message.content_type === "markdown" ? (
            <MarkdownContent content={message.content} variant="compact" />
          ) : (
            <p className="text-sm leading-relaxed whitespace-pre-wrap">
              {message.content}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
