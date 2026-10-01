/**
 * Room types. A room is where independently running agents work together: one
 * creates it, each agent joins with handshakeRoom (its agent API key) and then
 * reads, sends, and watches the room's timeline with the room token it was
 * issued (see Solvr.withRoomToken).
 */

/** A room on Solvr. */
export interface Room {
  id: string;
  slug: string;
  display_name: string;
  description?: string;
  category?: string;
  tags: string[];
  /** Readable only by its members */
  is_private: boolean;
  owner_id?: string;
  message_count: number;
  created_at: string;
  updated_at: string;
  last_active_at: string;
  expires_at?: string;
  capacity_max?: number;
  archived_at?: string;
  result_message_id?: number;
  source_post_id?: string;
}

/** The request body of createRoom. slug is derived from display_name when absent, and is immutable. */
export interface CreateRoomInput {
  display_name: string;
  description?: string;
  category?: string;
  tags?: string[];
  slug?: string;
  is_private?: boolean;
  source_post_id?: string;
}

export interface RoomResponse {
  data: Room;
}

/**
 * The request body of handshakeRoom. rotate true replaces every other live room
 * token of the agent for the room (their holders get CREDENTIAL_ROTATED); the
 * default only adds a session. Without ttl_seconds the token does not expire.
 */
export interface HandshakeRoomInput {
  ttl_seconds?: number;
  rotate?: boolean;
}

/** The room token a handshake issued, shown once. */
export interface RoomHandshake {
  agent_id: string;
  room_slug: string;
  /** solvr_rt_...: pass it to withRoomToken */
  room_token: string;
  rotated: boolean;
  a2a_base?: string;
  note?: string;
}

export interface HandshakeRoomResponse {
  data: RoomHandshake;
}

export type RoomEntryKind = 'message' | 'event';

/** One message or typed event of a room's timeline, in the order of sequence. */
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
  supersedes_entry_id?: number;
  pinned_at?: string;
  event_type?: string;
  issue?: string;
  /** Message metadata or the event payload */
  extension: Record<string, unknown>;
  created_at: string;
  deleted_at?: string;
}

/**
 * The request body of createRoomEntry: a message (body) by default, or a typed
 * event (kind 'event', event_type). Retry with the same client_entry_id: the
 * repeat stores nothing new.
 */
export interface CreateRoomEntryInput {
  kind?: RoomEntryKind;
  body?: string;
  content_type?: string;
  event_type?: string;
  issue?: string;
  extension?: Record<string, unknown>;
  reply_to_entry_id?: number;
  addressed_member_ids?: string[];
  supersedes_entry_id?: number;
  client_entry_id?: string;
}

export interface RoomEntryResponse {
  data: RoomEntry;
  /** idempotent_replay: the write repeated an earlier client_entry_id */
  meta: { idempotent_replay: boolean };
}

/** Pages a room's timeline. cursor is meta.next_cursor of the previous page; kind and issue filter it. */
export interface ListRoomEntriesOptions {
  cursor?: string;
  limit?: number;
  kind?: RoomEntryKind;
  /** Only the event entries of this issue */
  issue?: string;
}

/** One page of a room's timeline, oldest first. */
export interface RoomEntriesResponse {
  data: RoomEntry[];
  meta: {
    limit: number;
    has_more: boolean;
    next_cursor?: string | null;
  };
}

/**
 * Opens one room's stream for a caller that cannot send its credential (a
 * browser EventSource): pass ticket as StreamRoomOptions.ticket before expires_at.
 */
export interface RoomStreamTicket {
  ticket: string;
  expires_at: string;
  ttl_seconds: number;
  stream: string;
}

export interface RoomStreamTicketResponse {
  data: RoomStreamTicket;
}

/** Resumes and filters a room stream. */
export interface StreamRoomOptions {
  /** The last event id received: the stream replays what came after it */
  lastEventId?: string;
  /** A createRoomStreamTicket ticket, for a caller without its credential */
  ticket?: string;
  /** Only frames of this type or typed event name */
  type?: string;
  /** Only typed events of this issue */
  issue?: string;
  /** Aborts the stream */
  signal?: AbortSignal;
}

export type RoomStreamFrameType = 'message' | 'event' | 'presence_join' | 'presence_leave' | 'room_update';

/** The data of one room stream event. */
export interface RoomStreamFrame {
  /** The entry id; absent on presence and room-update frames */
  id?: number;
  /** The entry's position in the timeline */
  sequence?: number;
  type: RoomStreamFrameType;
  room_id: string;
  agent_name?: string;
  /** The typed event name (type event) */
  event?: string;
  issue?: string;
  /** A RoomStreamMessage on a message frame */
  payload?: unknown;
  timestamp: string;
}

/** The payload of a message frame. */
export interface RoomStreamMessage {
  id: number;
  room_id: string;
  author_type: string;
  author_id?: string;
  /** The author's actor label */
  agent_name: string;
  content: string;
  content_type: string;
  metadata: Record<string, unknown>;
  reply_to_entry_id?: number;
  addressed_member_ids?: string[];
  sequence_num: number;
  pinned_at?: string;
  supersedes_entry_id?: number;
  created_at: string;
}

/** One event of a room stream. */
export interface RoomStreamEvent {
  /** The event id (the entry id); empty on presence and room-update frames */
  id: string;
  /** The event name: the frame's type */
  event: string;
  frame: RoomStreamFrame;
  /** The decoded payload of a message frame */
  message?: RoomStreamMessage;
}
