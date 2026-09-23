"use client";

import { useState, useCallback, useRef, useEffect, useLayoutEffect, useMemo } from 'react';
import type { APIRoom, APIRoomMessage, APIAgentPresenceRecord, RoomConnectionStatus } from '@/lib/api-types';
import { MessageList } from './message-list';
import { PresenceSidebar } from './presence-sidebar';
import { CommentInput } from './comment-input';
import { ConnectionStatusBadge } from './connection-status-badge';
import { SseStatusBadge } from './sse-status-badge';
import { NewMessagesBadge } from './new-messages-badge';
import { RoomHeader } from './room-header';
import { useRoomSse } from '@/hooks/use-room-sse';
import { api } from '@/lib/api';
import { mergeMessages, isNearBottom } from '@/lib/rooms/message-view';

interface RoomDetailClientProps {
  room: APIRoom;
  initialMessages: APIRoomMessage[];
  initialAgents: APIAgentPresenceRecord[];
  ownerDisplayName?: string;
  // Server-derived connection progress (waiting/started). The client renders it
  // as-is and never recomputes it from the presence list.
  connectionStatus?: RoomConnectionStatus;
  // Persistent id of a message to deep-link to (highlight + scroll into view).
  // In production this is read from the ?message= query param when not supplied.
  highlightMessageId?: number;
}

const OLDER_PAGE_SIZE = 50;

// Reads a deep-link target from the URL (?message=<id>) without pulling in the
// Next router, so the component stays trivially testable via the prop.
function readMessageParam(): number | undefined {
  if (typeof window === 'undefined') return undefined;
  const raw = new URLSearchParams(window.location.search).get('message');
  if (!raw) return undefined;
  const id = Number(raw);
  return Number.isFinite(id) && id > 0 ? id : undefined;
}

export function RoomDetailClient({ room, initialMessages, initialAgents, ownerDisplayName, connectionStatus, highlightMessageId }: RoomDetailClientProps) {
  // The transcript reads oldest -> newest (top -> bottom). All batches (initial
  // window, older-history pages, SSE pushes, deep-link fetches, local echoes) go
  // through mergeMessages, so the list is always ordered by server id and free
  // of duplicates.
  const [messages, setMessages] = useState<APIRoomMessage[]>(() => mergeMessages(initialMessages));
  const [agents, setAgents] = useState<APIAgentPresenceRecord[]>(initialAgents);
  const [unreadCount, setUnreadCount] = useState(0);
  const [highlightId, setHighlightId] = useState<number | undefined>(undefined);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [hasOlder, setHasOlder] = useState(
    () => initialMessages.length > 0 && room.message_count > initialMessages.length,
  );

  const scrollContainerRef = useRef<HTMLDivElement>(null);
  // The reader is "following the latest exchange" while near the bottom, where
  // the newest message lives. Default true: the page opens pinned to the latest.
  const isNearBottomRef = useRef(true);
  // When set, the next layout pass pins the viewport to the bottom (newest).
  const pinBottomRef = useRef(true);
  // When set (during LOAD OLDER), the next layout pass keeps the reading position
  // anchored after older messages are prepended, so nothing appears to jump.
  const olderAnchorRef = useRef<number | null>(null);
  const deepLinkDoneRef = useRef(false);
  const highlightScrolledRef = useRef(false);

  // Get the highest message ID from SSR data for Last-Event-ID replay (D-35)
  const lastKnownId = initialMessages.length > 0
    ? Math.max(...initialMessages.map(m => m.id))
    : undefined;

  const { status, newMessages, presenceJoins, presenceLeaves, clearNewMessages, clearPresenceEvents } = useRoomSse(
    room.slug,
    lastKnownId
  );

  const scrollToBottom = useCallback((smooth = false) => {
    const el = scrollContainerRef.current;
    if (!el) return;
    if (typeof el.scrollTo === 'function') {
      el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' });
    }
    el.scrollTop = el.scrollHeight;
  }, []);

  // Position the viewport after every message change:
  //   - LOAD OLDER prepends history -> keep the reading position anchored.
  //   - otherwise, if we should follow the latest -> pin to the bottom.
  useLayoutEffect(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    if (olderAnchorRef.current != null) {
      el.scrollTop += el.scrollHeight - olderAnchorRef.current;
      olderAnchorRef.current = null;
      return;
    }
    if (pinBottomRef.current) {
      pinBottomRef.current = false;
      el.scrollTop = el.scrollHeight;
    }
  }, [messages]);

  // Append new SSE messages. When the reader is following the latest exchange the
  // new message is scrolled into view; when they are reading earlier history the
  // unread counter grows and a "Jump to latest" indicator appears (D-34: never
  // yank the reader's position).
  useEffect(() => {
    if (newMessages.length === 0) return;
    const existingIds = new Set(messages.map(m => m.id));
    const added = newMessages.filter(m => !existingIds.has(m.id)).length;
    if (added > 0) {
      setMessages(prev => mergeMessages(prev, newMessages));
      if (isNearBottomRef.current) {
        pinBottomRef.current = true;
      } else {
        setUnreadCount(c => c + added);
      }
    }
    clearNewMessages();
  }, [newMessages, clearNewMessages, messages]);

  // Track scroll position so we know whether the reader is following the latest
  // exchange; clear the unread count the moment they return to the bottom.
  useEffect(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    const onScroll = () => {
      const near = isNearBottom(el);
      isNearBottomRef.current = near;
      if (near) setUnreadCount(0);
    };
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => el.removeEventListener('scroll', onScroll);
  }, []);

  // Resolve a deep-linked message once. If it is outside the loaded window we
  // fetch it by id (stable lookup) and merge it in; ordering keeps it in place.
  useEffect(() => {
    if (deepLinkDoneRef.current) return;
    deepLinkDoneRef.current = true;
    const target = highlightMessageId ?? readMessageParam();
    if (target == null) return;
    if (messages.some(m => m.id === target)) {
      setHighlightId(target);
      return;
    }
    api.fetchRoomMessage(room.slug, target)
      .then(res => {
        setMessages(prev => mergeMessages(prev, [res.data]));
        setHighlightId(target);
      })
      .catch(() => {
        // Message deleted, moved, or not readable — leave the transcript as-is.
      });
  // Intentionally mount-once: the deep link is resolved from the initial URL.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Scroll the highlighted message into view once it is in the DOM.
  useEffect(() => {
    if (highlightId == null || highlightScrolledRef.current) return;
    const el = scrollContainerRef.current?.querySelector(`[data-message-id="${highlightId}"]`);
    if (el) {
      highlightScrolledRef.current = true;
      // The deep-link fetch must never be pinned to the bottom instead.
      pinBottomRef.current = false;
      (el as HTMLElement).scrollIntoView({ block: 'center' });
    }
  }, [highlightId, messages]);

  const loadOlder = useCallback(async () => {
    if (loadingOlder || messages.length === 0) return;
    setLoadingOlder(true);
    const oldestId = Math.min(...messages.map(m => m.id));
    const el = scrollContainerRef.current;
    if (el) olderAnchorRef.current = el.scrollHeight;
    try {
      const res = await api.fetchRoomMessages(room.slug, { before: oldestId, limit: OLDER_PAGE_SIZE });
      const batch = res.data;
      if (batch.length < OLDER_PAGE_SIZE) setHasOlder(false);
      if (batch.length > 0) {
        setMessages(prev => mergeMessages(prev, batch));
      } else {
        olderAnchorRef.current = null;
      }
    } catch {
      // Transient failure — keep current history; the button stays available.
      olderAnchorRef.current = null;
    } finally {
      setLoadingOlder(false);
    }
  }, [loadingOlder, messages, room.slug]);

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

  // Comment sent callback — append confirmed message (D-28: no optimistic UI).
  // The author's own message is newest, so follow it to the bottom.
  const handleMessageSent = useCallback((msg: APIRoomMessage) => {
    setMessages(prev => mergeMessages(prev, [msg]));
    isNearBottomRef.current = true;
    pinBottomRef.current = true;
    setUnreadCount(0);
  }, []);

  // Click on the Jump to latest indicator: dismiss and scroll to the newest
  // message (bottom, since the transcript reads oldest -> newest).
  const handleDismissUnread = useCallback(() => {
    setUnreadCount(0);
    isNearBottomRef.current = true;
    scrollToBottom(true);
  }, [scrollToBottom]);

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

      {/* Connection progress (server-derived) + live SSE transport status */}
      <div className="mb-2 shrink-0 flex items-center gap-4">
        <ConnectionStatusBadge status={connectionStatus} />
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
          <div ref={scrollContainerRef} data-testid="room-scroll" className="flex-1 overflow-y-auto min-h-0">
            <MessageList
              messages={messages}
              slug={room.slug}
              highlightId={highlightId}
              hasOlder={hasOlder}
              loadingOlder={loadingOlder}
              onLoadOlder={loadOlder}
            />
          </div>

          {/* Comment input — pinned at bottom, always visible */}
          <div className="border-t border-border shrink-0">
            <CommentInput slug={room.slug} onMessageSent={handleMessageSent} archived={displayedRoom.archived_at != null} />
          </div>
        </div>

        {/* Sidebar — stacked below chat on mobile, right rail on desktop.
            Surfaces CONNECT AGENT (owner rotate+copy) + ROOM INFO on mobile. */}
        <aside className="w-full lg:w-72 shrink-0 lg:overflow-y-auto">
          <PresenceSidebar agents={agents} room={displayedRoom} layout="desktop" />
        </aside>
      </div>

      {/* Floating Jump to latest indicator (D-34: user-initiated only) */}
      <NewMessagesBadge count={unreadCount} onClick={handleDismissUnread} />
    </div>
  );
}
