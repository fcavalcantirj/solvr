"use client";

import { useState, useCallback, useRef, useEffect } from 'react';
import type { APIRoom, APIRoomMessage, APIAgentPresenceRecord } from '@/lib/api-types';
import { MessageList } from './message-list';
import { PresenceSidebar } from './presence-sidebar';
import { CommentInput } from './comment-input';
import { SseStatusBadge } from './sse-status-badge';
import { NewMessagesBadge } from './new-messages-badge';
import { useRoomSse } from '@/hooks/use-room-sse';

interface RoomDetailClientProps {
  room: APIRoom;
  initialMessages: APIRoomMessage[];
  initialAgents: APIAgentPresenceRecord[];
}

export function RoomDetailClient({ room, initialMessages, initialAgents }: RoomDetailClientProps) {
  const [messages, setMessages] = useState<APIRoomMessage[]>(initialMessages);
  const [agents, setAgents] = useState<APIAgentPresenceRecord[]>(initialAgents);
  const [unreadCount, setUnreadCount] = useState(0);
  const bottomRef = useRef<HTMLDivElement>(null);

  // Get the highest message ID from SSR data for Last-Event-ID replay (D-35)
  const lastKnownId = initialMessages.length > 0
    ? Math.max(...initialMessages.map(m => m.id))
    : undefined;

  const { status, newMessages, presenceJoins, presenceLeaves, clearNewMessages } = useRoomSse(
    room.slug,
    lastKnownId
  );

  // Append new messages from SSE — NO auto-scroll (D-34)
  useEffect(() => {
    if (newMessages.length > 0) {
      setMessages(prev => {
        // Deduplicate by message id (T-16-16: dedup for replay safety)
        const existingIds = new Set(prev.map(m => m.id));
        const unique = newMessages.filter(m => !existingIds.has(m.id));
        if (unique.length === 0) return prev;
        setUnreadCount(c => c + unique.length);
        return [...prev, ...unique];
      });
      clearNewMessages();
    }
  }, [newMessages, clearNewMessages]);

  // Handle presence joins
  useEffect(() => {
    if (presenceJoins.length > 0) {
      setAgents(prev => {
        const existingNames = new Set(prev.map(a => a.agent_name));
        const newAgents = presenceJoins.filter(a => !existingNames.has(a.agent_name));
        return [...prev, ...newAgents];
      });
    }
  }, [presenceJoins]);

  // Handle presence leaves
  useEffect(() => {
    if (presenceLeaves.length > 0) {
      setAgents(prev => prev.filter(a => !presenceLeaves.includes(a.agent_name)));
    }
  }, [presenceLeaves]);

  // Comment sent callback — append confirmed message (D-28: no optimistic UI)
  const handleMessageSent = useCallback((msg: APIRoomMessage) => {
    setMessages(prev => {
      if (prev.some(m => m.id === msg.id)) return prev; // Deduplicate
      return [...prev, msg];
    });
    // Auto-scroll to own message (exception to D-34: user's own message)
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  // Click on new-messages badge: scroll to bottom and reset counter
  const handleScrollToBottom = useCallback(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    setUnreadCount(0);
  }, []);

  return (
    <div className="flex flex-col h-full min-h-0">
      {/* SSE status badge */}
      <div className="mb-2 shrink-0">
        <SseStatusBadge status={status} />
      </div>

      {/* Mobile-only presence strip (hidden on lg+) */}
      <div className="lg:hidden shrink-0">
        <PresenceSidebar agents={agents} room={room} layout="mobile" />
      </div>

      <div className="flex gap-8 flex-1 min-h-0">
        {/* Main message area — fills remaining viewport via flex parent chain */}
        <div className="flex-1 min-w-0 border border-border bg-card flex flex-col min-h-0">
          {/* Messages — scrollable */}
          <div className="flex-1 overflow-y-auto min-h-0">
            <MessageList messages={messages} slug={room.slug} />
            <div ref={bottomRef} />
          </div>

          {/* Comment input — pinned at bottom, always visible */}
          <div className="border-t border-border shrink-0">
            <CommentInput slug={room.slug} onMessageSent={handleMessageSent} />
          </div>
        </div>

        {/* Desktop-only sidebar */}
        <aside className="hidden lg:block w-72 shrink-0 overflow-y-auto">
          <PresenceSidebar agents={agents} room={room} layout="desktop" />
        </aside>
      </div>

      {/* Floating new messages badge (D-34: user-initiated only) */}
      <NewMessagesBadge count={unreadCount} onClick={handleScrollToBottom} />
    </div>
  );
}
