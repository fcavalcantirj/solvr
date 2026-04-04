"use client";

import { useEffect, useRef, useState, useCallback } from 'react';
import type { APIRoomMessage, APIAgentPresenceRecord } from '@/lib/api-types';

export type SseStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

interface UseRoomSseReturn {
  status: SseStatus;
  newMessages: APIRoomMessage[];
  presenceJoins: APIAgentPresenceRecord[];
  presenceLeaves: string[]; // agent_names that left
  clearNewMessages: () => void;
}

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export function useRoomSse(slug: string, lastKnownMessageId?: number): UseRoomSseReturn {
  const [status, setStatus] = useState<SseStatus>('connecting');
  const [newMessages, setNewMessages] = useState<APIRoomMessage[]>([]);
  const [presenceJoins, setPresenceJoins] = useState<APIAgentPresenceRecord[]>([]);
  const [presenceLeaves, setPresenceLeaves] = useState<string[]>([]);
  const lastEventIdRef = useRef<string | null>(
    lastKnownMessageId ? String(lastKnownMessageId) : null
  );
  const esRef = useRef<EventSource | null>(null);

  const clearNewMessages = useCallback(() => setNewMessages([]), []);

  useEffect(() => {
    let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;

    const connect = () => {
      // Build URL with optional lastEventId query param for initial connect.
      // EventSource sends Last-Event-ID header automatically on browser reconnects,
      // but for the first connect after SSR we use the query param.
      const url = new URL(`${API_BASE_URL}/v1/rooms/${encodeURIComponent(slug)}/stream`);
      if (lastEventIdRef.current) {
        url.searchParams.set('lastEventId', lastEventIdRef.current);
      }

      const es = new EventSource(url.toString());
      esRef.current = es;

      es.onopen = () => {
        setStatus('connected');
      };

      es.addEventListener('message', (e: MessageEvent) => {
        if (e.lastEventId) {
          lastEventIdRef.current = e.lastEventId;
        }
        try {
          const evt = JSON.parse(e.data) as {
            id?: number;
            type?: string;
            payload?: APIRoomMessage;
            agent_name?: string;
            timestamp?: string;
          };
          // The hub sends RoomEvent{Type: "message", Payload: <models.Message>}
          // payload contains the full APIRoomMessage fields
          const msg: APIRoomMessage = (evt.payload as APIRoomMessage) || (evt as unknown as APIRoomMessage);
          if (msg && (msg.id || evt.id)) {
            setNewMessages(prev => [...prev, msg]);
          }
        } catch {
          // Ignore malformed events (T-16-16)
        }
      });

      es.addEventListener('presence_join', (e: MessageEvent) => {
        try {
          const evt = JSON.parse(e.data) as {
            payload?: APIAgentPresenceRecord;
          };
          const agent: APIAgentPresenceRecord = (evt.payload as APIAgentPresenceRecord) || (evt as unknown as APIAgentPresenceRecord);
          if (agent) {
            setPresenceJoins(prev => [...prev, agent]);
          }
        } catch {
          // Ignore malformed events
        }
      });

      es.addEventListener('presence_leave', (e: MessageEvent) => {
        try {
          const evt = JSON.parse(e.data) as {
            agent_name?: string;
            payload?: { agent_name?: string };
          };
          const agentName: string = evt.agent_name || evt.payload?.agent_name || '';
          if (agentName) {
            setPresenceLeaves(prev => [...prev, agentName]);
          }
        } catch {
          // Ignore malformed events
        }
      });

      es.onerror = () => {
        setStatus('reconnecting');
        es.close();
        // Reconnect after 3 seconds (T-16-17: controlled reconnect delay)
        reconnectTimeout = setTimeout(connect, 3000);
      };
    };

    connect();

    return () => {
      if (esRef.current) {
        esRef.current.close();
      }
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
      }
      setStatus('disconnected');
    };
  }, [slug]);

  return { status, newMessages, presenceJoins, presenceLeaves, clearNewMessages };
}
