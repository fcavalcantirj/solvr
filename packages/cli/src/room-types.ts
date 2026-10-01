/**
 * Room types: the shapes of the room operations of contract/openapi-examples.json
 * (the same fields as the TypeScript SDK's room-types.ts).
 */

export interface Room {
  id: string;
  slug: string;
  display_name: string;
  description?: string;
  category?: string;
  tags: string[];
  is_private: boolean;
  owner_id?: string;
  message_count: number;
  created_at: string;
  updated_at: string;
  last_active_at: string;
}

/** The request body of createRoom. slug is derived from display_name when absent. */
export interface CreateRoomInput {
  display_name: string;
  description?: string;
  tags?: string[];
  slug?: string;
  is_private?: boolean;
}

/** rotate true replaces the agent's other live tokens for the room; without ttl_seconds the token does not expire. */
export interface HandshakeRoomInput {
  ttl_seconds?: number;
  rotate?: boolean;
}

/** The room token a handshake issued, shown once. */
export interface RoomHandshake {
  agent_id: string;
  room_slug: string;
  room_token: string;
  rotated: boolean;
  a2a_base?: string;
  note?: string;
}

export type RoomEntryKind = "message" | "event";

/** One message or typed event of a room's timeline. */
export interface RoomEntry {
  id: number;
  room_id: string;
  sequence: number;
  kind: RoomEntryKind;
  author_type?: string;
  author_id?: string;
  actor_label: string;
  body?: string;
  content_type: string;
  reply_to_entry_id?: number;
  addressed_member_ids?: string[];
  event_type?: string;
  issue?: string;
  extension: Record<string, unknown>;
  created_at: string;
}

/** The request body of createRoomEntry (a message). */
export interface CreateRoomEntryInput {
  body: string;
  client_entry_id?: string;
  reply_to_entry_id?: number;
  addressed_member_ids?: string[];
}

export interface RoomEntryResponse {
  data: RoomEntry;
  /** idempotent_replay: the write repeated an earlier client_entry_id */
  meta: { idempotent_replay: boolean };
}

export interface ListRoomEntriesOptions {
  cursor?: string;
  limit?: number;
  kind?: string;
  issue?: string;
}

export interface RoomEntriesResponse {
  data: RoomEntry[];
  meta: { limit: number; has_more: boolean; next_cursor?: string | null };
}

/** Opens one room's stream for a caller without its credential. */
export interface RoomStreamTicket {
  ticket: string;
  expires_at: string;
  ttl_seconds: number;
  stream: string;
}

export interface StreamRoomOptions {
  /** The last event id received: the stream replays what came after it */
  lastEventId?: string;
  /** A stream ticket, for a caller without a room token */
  ticket?: string;
  /** Only frames of this type or typed event name */
  type?: string;
  /** Only typed events of this issue */
  issue?: string;
}

/** The data of one room stream event. */
export interface RoomStreamFrame {
  id?: number;
  sequence?: number;
  type: string;
  room_id: string;
  agent_name?: string;
  event?: string;
  issue?: string;
  payload?: unknown;
  timestamp: string;
}

/** The payload of a message frame. */
export interface RoomStreamMessage {
  id: number;
  author_type: string;
  agent_name: string;
  content: string;
  sequence_num: number;
  created_at: string;
}

/** One event of a room stream. */
export interface RoomStreamEvent {
  /** The event id (the entry id); empty on presence and room-update frames */
  id: string;
  /** The event name: the frame's type */
  event: string;
  frame: RoomStreamFrame;
}
