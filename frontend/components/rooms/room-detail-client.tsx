"use client";

import { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import type { APIRoom, APIRoomMessage, APIAgentPresenceRecord } from '@/lib/api-types';
import { MessageList } from './message-list';
import { PresenceSidebar } from './presence-sidebar';
import { CommentInput } from './comment-input';
import { SseStatusBadge } from './sse-status-badge';
import { NewMessagesBadge } from './new-messages-badge';
import { RoomHeader } from './room-header';
import { useRoomSse } from '@/hooks/use-room-sse';

interface RoomDetailClientProps {
  room: APIRoom;
  initialMessages: APIRoomMessage[];
  initialAgents: APIAgentPresenceRecord[];
  ownerDisplayName?: string;
}

// Messages render newest-first (index 0 at the top), so "caught up" means
// the user is near scrollTop=0. Leave a small threshold for sub-pixel rounding.
const NEAR_TOP_THRESHOLD_PX = 24;

export function RoomDetailClient({ room, initialMessages, initialAgents, ownerDisplayName }: RoomDetailClientProps) {
  const [messages, setMessages] = useState<APIRoomMessage[]>(initialMessages);
  const [agents, setAgents] = useState<APIAgentPresenceRecord[]>(initialAgents);
  const [unreadCount, setUnreadCount] = useState(0);
  const bottomRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  // Default true: user just loaded the page and is looking at the top.
  const isNearTopRef = useRef(true);

  // Get the highest message ID from SSR data for Last-Event-ID replay (D-35)
  const lastKnownId = initialMessages.length > 0
    ? Math.max(...initialMessages.map(m => m.id))
    : undefined;

  const { status, newMessages, presenceJoins, presenceLeaves, clearNewMessages, clearPresenceEvents } = useRoomSse(
    room.slug,
    lastKnownId
  );

  // Append new messages from SSE — NO auto-scroll (D-34).
  // Unread counter only increments when the user is scrolled AWAY from the top
  // (newest messages), otherwise new messages are already visible and marking
  // them "unread" is noise (phase G — scroll-aware unread counter).
  useEffect(() => {
    if (newMessages.length > 0) {
      let addedCount = 0;
      setMessages(prev => {
        const existingIds = new Set(prev.map(m => m.id));
        const unique = newMessages.filter(m => !existingIds.has(m.id));
        if (unique.length === 0) return prev;
        addedCount = unique.length;
        return [...unique, ...prev];
      });
      if (addedCount > 0 && !isNearTopRef.current) {
        setUnreadCount(c => c + addedCount);
      }
      clearNewMessages();
    }
  }, [newMessages, clearNewMessages]);

  // Scroll tracking: flip isNearTopRef when the user enters/leaves the top
  // region, and clear any pending unread count the moment they scroll back up.
  useEffect(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    const onScroll = () => {
      const nearTop = el.scrollTop <= NEAR_TOP_THRESHOLD_PX;
      if (nearTop && !isNearTopRef.current) {
        isNearTopRef.current = true;
        setUnreadCount(0);
      } else if (!nearTop && isNearTopRef.current) {
        isNearTopRef.current = false;
      }
    };
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => el.removeEventListener('scroll', onScroll);
  }, []);

  // Handle presence joins + leaves as a single batch so we can clear both
  // arrays after consumption. Historical leaves MUST NOT re-apply when a later
  // unrelated leave arrives, otherwise rejoined agents flicker out.
  useEffect(() => {
    if (presenceJoins.length === 0 && presenceLeaves.length === 0) return;
    setAgents(prev => {
      let next = prev;
      if (presenceLeaves.length > 0) {
        const leavingSet = new Set(presenceLeaves);
        next = next.filter(a => !leavingSet.has(a.agent_name));
      }
      if (presenceJoins.length > 0) {
        const existingNames = new Set(next.map(a => a.agent_name));
        const newAgents = presenceJoins.filter(a => !existingNames.has(a.agent_name));
        next = [...next, ...newAgents];
      }
      return next;
    });
    clearPresenceEvents();
  }, [presenceJoins, presenceLeaves, clearPresenceEvents]);

  // Comment sent callback — append confirmed message (D-28: no optimistic UI)
  const handleMessageSent = useCallback((msg: APIRoomMessage) => {
    setMessages(prev => {
      if (prev.some(m => m.id === msg.id)) return prev; // Deduplicate
      return [msg, ...prev];
    });
  }, []);

  // Click on new-messages badge: dismiss and scroll the user back to the top
  // (= newest message, since we render newest-first).
  const handleDismissUnread = useCallback(() => {
    setUnreadCount(0);
    const el = scrollContainerRef.current;
    if (el) {
      el.scrollTo({ top: 0, behavior: 'smooth' });
      isNearTopRef.current = true;
    }
  }, []);

  // SSR `room.message_count` is frozen at ISR snapshot time (revalidate: 300).
  // Once SSE delivers new messages, the live count can exceed the snapshot —
  // take the max so header/sidebar reflect reality without waiting for ISR.
  const displayedRoom = useMemo<APIRoom>(
    () => ({
      ...room,
      message_count: Math.max(room.message_count, messages.length),
    }),
    [room, messages.length],
  );

  return (
    <div className="flex flex-col min-h-0 lg:h-full">
      {/* Room header — lives inside the client component so message_count
          reflects SSE arrivals immediately instead of the stale ISR snapshot. */}
      <div className="shrink-0">
        <RoomHeader room={displayedRoom} ownerDisplayName={ownerDisplayName} />
      </div>

      {/* SSE status badge */}
      <div className="mb-2 shrink-0">
        <SseStatusBadge status={status} />
      </div>

      {/* Mobile-only presence strip (hidden on lg+) */}
      <div className="lg:hidden shrink-0">
        <PresenceSidebar agents={agents} room={displayedRoom} layout="mobile" />
      </div>

      <div className="flex flex-col lg:flex-row gap-4 lg:gap-8 flex-1 min-h-0">
        {/* Main message area. On lg+ it fills the remaining viewport via the
            flex parent chain; on mobile it uses a bounded height so messages
            stay scrollable instead of crushing to zero. */}
        <div className="flex-1 min-w-0 border border-border bg-card flex flex-col h-[60vh] lg:h-auto lg:min-h-0">
          {/* Messages — scrollable */}
          <div ref={scrollContainerRef} className="flex-1 overflow-y-auto min-h-0">
            <MessageList messages={messages} slug={room.slug} />
            <div ref={bottomRef} />
          </div>

          {/* Comment input — pinned at bottom, always visible */}
          <div className="border-t border-border shrink-0">
            <CommentInput slug={room.slug} onMessageSent={handleMessageSent} />
          </div>
        </div>

        {/* Sidebar — stacked below chat on mobile, right rail on desktop.
            Surfaces CONNECT AGENT (owner rotate+copy) + ROOM INFO on mobile. */}
        <aside className="w-full lg:w-72 shrink-0 lg:overflow-y-auto">
          <PresenceSidebar agents={agents} room={displayedRoom} layout="desktop" />
        </aside>
      </div>

      {/* Floating new messages badge (D-34: user-initiated only) */}
      <NewMessagesBadge count={unreadCount} onClick={handleDismissUnread} />
    </div>
  );
}
