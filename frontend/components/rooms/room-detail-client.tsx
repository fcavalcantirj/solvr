"use client";

import styles from "./rooms-layout.module.css";
import { useState, useCallback, useRef, useEffect, useLayoutEffect, useMemo } from 'react';
import type { APIRoom, APIRoomMessage, APIAgentPresenceRecord, RoomConnectionStatus } from '@/lib/api-types';
import { MessageList } from './message-list';
import { PresenceSidebar } from './presence-sidebar';
import { CommentInput } from './comment-input';
import { ConnectAgentPanel } from './connect-agent-panel';
import { RoomStarterPrompts } from './room-starter-prompts';
import { RoomContextPanel } from './room-context-panel';
import { ConnectionStatusBadge } from './connection-status-badge';
import { SseStatusBadge } from './sse-status-badge';
import { NewMessagesBadge } from './new-messages-badge';
import { RoomHeader } from './room-header';
import { useRoomSse } from '@/hooks/use-room-sse';
import { api } from '@/lib/api';
import { mergeMessages, isNearBottom } from '@/lib/rooms/message-view';
import { recordRoomView } from '@/lib/recently-viewed-rooms';
import { useShareVisit } from '@/hooks/use-share-visit';
import { useRoomViewer } from '@/hooks/use-room-viewer';
import { RoomNotifyToggle } from './room-notify-toggle';

interface RoomDetailClientProps {
  room: APIRoom;
  initialMessages: APIRoomMessage[];
  initialAgents: APIAgentPresenceRecord[];
  ownerDisplayName?: string;
  // Server-derived connection progress (waiting/started). The client renders it
  // as-is and never recomputes it from the presence list.
  connectionStatus?: RoomConnectionStatus;
  // Compact context area (task 33, step 4): the initial task (first message) and
  // the latest pinned directive, both chosen by the API. Rendered as-is.
  initialTask?: APIRoomMessage | null;
  latestPinned?: APIRoomMessage | null;
  // Persistent id of a message to deep-link to (highlight + scroll into view).
  // In production this is read from the ?message= query param when not supplied.
  highlightMessageId?: number;
  // The API's "Try this workflow" link (GET /v1/rooms/{slug} try_workflow_url): a fresh
  // room seeded from this public room's task. null for a private room (idx 88).
  tryWorkflowUrl?: string | null;
  // Server-rendered links to the transcript archive and outcome posts (task idx 81),
  // shown in the sidebar so crawlers and readers reach every earlier message.
  archive?: React.ReactNode;
}

const OLDER_PAGE_SIZE = 50;

// Reads a deep-link target from the URL (?message=<id>) without pulling in the
// Next router, so the component stays trivially testable via the prop.
// v1.3.9: a room is a transcript page. The document scrolls the conversation, so
// every scroll read and write goes to the page's own scroller.
function pageScroller(): HTMLElement {
  return (document.scrollingElement as HTMLElement | null) ?? document.documentElement;
}

function readMessageParam(): number | undefined {
  if (typeof window === 'undefined') return undefined;
  const raw = new URLSearchParams(window.location.search).get('message');
  if (!raw) return undefined;
  const id = Number(raw);
  return Number.isFinite(id) && id > 0 ? id : undefined;
}

export function RoomDetailClient({ room, initialMessages, initialAgents, ownerDisplayName, connectionStatus, initialTask, latestPinned, highlightMessageId, tryWorkflowUrl, archive }: RoomDetailClientProps) {
  // The transcript reads oldest -> newest (top -> bottom). All batches (initial
  // window, older-history pages, SSE pushes, deep-link fetches, local echoes) go
  // through mergeMessages, so the list is always ordered by server id and free
  // of duplicates.
  const [messages, setMessages] = useState<APIRoomMessage[]>(() => mergeMessages(initialMessages));
  const [agents, setAgents] = useState<APIAgentPresenceRecord[]>(initialAgents);
  // Names of participants that were present and stopped heartbeating (presence
  // expired). Kept distinct from "never joined" and from a browser/API transport
  // failure so the reader can tell a stopped partner from a lost connection.
  const [offlineNames, setOfflineNames] = useState<string[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [highlightId, setHighlightId] = useState<number | undefined>(undefined);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [hasOlder, setHasOlder] = useState(
    () => initialMessages.length > 0 && room.message_count > initialMessages.length,
  );

  // Remember this browser opened the room so the Rooms page can offer a quick way
  // back. Only PUBLIC rooms are recorded — a private room is never a non-secret
  // public reference — and only the slug + display name are stored locally.
  useEffect(() => {
    if (!room.is_private) {
      recordRoomView({ slug: room.slug, displayName: room.display_name });
    }
  }, [room.is_private, room.slug, room.display_name]);

  // room_viewed: a room page was opened. Reported as a bare browser funnel step —
  // it carries no room identity, so a private room's title never leaks through it.
  useEffect(() => {
    void api.postFunnelEvent?.({ event: 'room_viewed' });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [room.slug]);

  // share_visit: opened through a share link (?via=share) — counted once per tab for a
  // PUBLIC room only; a private room's link is cleaned but never attributed (idx 88).
  useShareVisit(room.is_private ? null : { kind: 'room', ref: room.slug }, 'room_page');

  // Pinned directives (idx 92): the API says whether this viewer may pin, and answers a
  // pin change with the entry as it now stands and the directive now in force.
  const { canPin, notifications } = useRoomViewer(room.slug);
  const [directive, setDirective] = useState<APIRoomMessage | null | undefined>(latestPinned);
  const togglePin = useCallback(async (message: APIRoomMessage) => {
    try {
      const res = message.pinned_at
        ? await api.unpinEntry(room.slug, message.id)
        : await api.pinEntry(room.slug, message.id);
      setMessages((prev) => prev.map((m) => (m.id === res.data.id ? { ...m, pinned_at: res.data.pinned_at } : m)));
      setDirective(res.meta?.latest_pinned ?? null);
    } catch {
      // Refused or failed: the control keeps showing the state the API last confirmed.
    }
  }, [room.slug]);

  // The transcript element (deep-link lookups); the page itself does the scrolling.
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  // The reader is "following the latest exchange" while near the bottom of the page,
  // where the newest message lives. The page opens on the room's header, so this is
  // measured on mount rather than assumed.
  const isNearBottomRef = useRef(false);
  // When set, the next layout pass scrolls the page to the bottom (newest).
  const pinBottomRef = useRef(false);
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
    const el = pageScroller();
    if (typeof window.scrollTo === 'function') {
      window.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' });
    } else {
      el.scrollTop = el.scrollHeight;
    }
  }, []);

  // Position the viewport after every message change:
  //   - LOAD OLDER prepends history -> keep the reading position anchored.
  //   - otherwise, if we should follow the latest -> pin to the bottom.
  useLayoutEffect(() => {
    const el = pageScroller();
    if (olderAnchorRef.current != null) {
      el.scrollTop += el.scrollHeight - olderAnchorRef.current;
      olderAnchorRef.current = null;
      return;
    }
    if (pinBottomRef.current) {
      pinBottomRef.current = false;
      scrollToBottom(true);
    }
  }, [messages, scrollToBottom]);

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
    const onScroll = () => {
      const near = isNearBottom(pageScroller());
      isNearBottomRef.current = near;
      if (near) setUnreadCount(0);
    };
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
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
    olderAnchorRef.current = pageScroller().scrollHeight;
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
    // Track who went offline: a leave adds the name, a (re)join clears it. This
    // is the "participant offline" signal, separate from the transport badge.
    setOfflineNames(prev => {
      let next = prev;
      if (presenceLeaves.length > 0) {
        const additions = presenceLeaves.filter(n => !next.includes(n));
        if (additions.length > 0) next = [...next, ...additions];
      }
      if (presenceJoins.length > 0) {
        const rejoined = new Set(presenceJoins.map(a => a.agent_name));
        next = next.filter(n => !rejoined.has(n));
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
    <div className={styles.page}>
      {/* Room header — lives inside the client component so message_count
          reflects SSE arrivals immediately instead of the stale ISR snapshot. */}
      <RoomHeader
        room={displayedRoom}
        ownerDisplayName={ownerDisplayName}
        onlineCount={agents.length}
        tryWorkflowUrl={tryWorkflowUrl}
        liveStatus={
          <>
            <ConnectionStatusBadge status={connectionStatus} />
            {/* The transport speaks only when it is not live: the header already says LIVE. */}
            {status !== 'connected' ? <SseStatusBadge status={status} /> : null}
          </>
        }
      />

      <div className={styles.conversation}>
        {/* The conversation itself: the transcript reads down the page, one ledger
            row per message, and the page scrolls it (no box inside the page). */}
        <div className={styles.reading}>

          {/* Participant offline — a partner that was present has stopped responding.
              Kept distinct from the transport badge above (which reports the reader's
              own browser/API connection) and from "waiting for another agent". */}
          {offlineNames.length > 0 && (
            <div
              data-testid="participant-offline"
              role="status"
              className="shrink-0 border-b border-border py-4 font-mono text-[11px] uppercase leading-relaxed tracking-[0.18em] text-amber-700 dark:text-amber-400"
            >
              {offlineNames.join(", ")} went offline — Solvr is holding the room; they can resume anytime.
            </div>
          )}

          {/* Messages */}
          <div ref={scrollContainerRef} data-testid="room-scroll" className="min-w-0">
            <MessageList
              messages={messages}
              slug={room.slug}
              highlightId={highlightId}
              hasOlder={hasOlder}
              loadingOlder={loadingOlder}
              onLoadOlder={loadOlder}
              canPin={canPin}
              onTogglePin={togglePin}
            />
          </div>

          {/* Comment input — sticks to the bottom of the screen under the newest message */}
          <div className="sticky bottom-0 z-10 w-full border-t border-border bg-background/95 py-3 backdrop-blur supports-[backdrop-filter]:bg-background/85">
            <CommentInput slug={room.slug} onMessageSent={handleMessageSent} archived={displayedRoom.archived_at != null} />
          </div>
        </div>

        {/* The quiet side column: the room's context (initial task + latest pinned
            directive, so a reader never hunts the transcript for them), the
            connect tools, presence and the archive. */}
        <aside className={styles.rail}>
          <RoomContextPanel initialTask={initialTask} latestPinned={directive} />
          <RoomNotifyToggle slug={room.slug} initial={notifications} />
          {/* The header's Connect an agent action anchors here (#connect-agent).
              The ?created=1 landing reads the URL and self-hides on every other
              view; the recruit control serves logged-out visitors too (task 26). */}
          <div id="connect-agent" className={styles.anchor}>
            <RoomStarterPrompts room={displayedRoom} />
            <ConnectAgentPanel room={displayedRoom} tryWorkflowUrl={tryWorkflowUrl} />
          </div>
          <PresenceSidebar agents={agents} room={displayedRoom} layout="desktop" />
          {archive}
        </aside>
      </div>

      {/* Floating Jump to latest indicator (D-34: user-initiated only) */}
      <NewMessagesBadge count={unreadCount} onClick={handleDismissUnread} />
    </div>
  );
}
