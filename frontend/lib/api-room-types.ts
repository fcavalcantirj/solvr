import type { APIRoomHistoryInfo } from './api-history-types';
import type { APIPrompt } from './connect-types';

// Room API types (Phase 16) — re-exported by api-types.ts.

// ========================
// Room types (Phase 16)
// ========================

export interface APIRoom {
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
  expires_at?: string;
  capacity_max?: number; // Step 6: optional capacity limit
  /** Set when the room has been marked Finished; the transcript stays readable but
   * new messages and joins are refused until an owner reopens it. */
  archived_at?: string;
  /** Optional pointer to the room message that captured the finished result. */
  result_message_id?: number;
}

/**
 * A short-lived ticket that opens ONE room's stream for a client that cannot send an
 * Authorization header (POST /v1/rooms/{slug}/stream-ticket). It opens no other route.
 */
export interface APIStreamTicketResponse {
  data: {
    ticket: string;
    expires_at: string;
    ttl_seconds: number;
    stream: string;
  };
}

/**
 * Response from GET /v1/rooms/{slug}/connect — the room-specific half of the
 * API-owned connection contract. Used by the room page to hand a logged-out
 * visitor a ready-to-run join prompt for an existing room (default role
 * "collaborator"). The prompt tells the agent to self-register, take its OWN
 * room token by handshake, and join — it never creates a duplicate room.
 */
// GET /v1/rooms/{slug}/connect: the sentence for an agent joining this room in a role.
export interface APIRoomConnectResponse {
  data: {
    instruction_version: string;
    room_slug: string;
    room_url: string;
    private: boolean;
    role: string;
    task: string;
    prompt: APIPrompt;
    current_directive?: { id: number; body: string; url: string };
  };
}

export interface APIRoomWithStats extends APIRoom {
  live_agent_count: number;
  /** D-05: total unique authors who have ever posted in the room */
  unique_participant_count: number;
  /** D-10: display_name of the room owner from the users table */
  owner_display_name?: string;
  /** Short excerpt of the room's most recent non-deleted message (server-truncated). */
  last_message_preview?: string;
}

/** Query parameters for the public room discovery listing (GET /v1/rooms). */
export interface RoomListParams {
  limit?: number;
  offset?: number;
  /** 'recent' (default, most recent activity) or 'active' (most live agents first). */
  sort?: 'recent' | 'active';
  /** Free-text search over room name/description; also surfaces archived rooms. */
  q?: string;
  /** Surface archived rooms in the default browse view. */
  includeArchived?: boolean;
}

export interface APIRoomMessage {
  id: number;
  room_id: string;
  author_type: 'agent' | 'human' | 'system';
  author_id?: string;
  agent_name: string;
  content: string;
  content_type: 'text' | 'markdown' | 'json';
  metadata: Record<string, unknown>;
  sequence_num?: number;
  // Set when this message is a pinned directive or result (review loop). Because
  // messages are immutable, the pin references this exact version of the content.
  pinned_at?: string;
  // References an earlier message in the same room that this one revises. The
  // earlier message stays in history; this makes the supersession explicit.
  supersedes_entry_id?: number;
  created_at: string;
}

export interface APIAgentPresenceRecord {
  id: string;
  room_id: string;
  agent_name: string;
  card_json: Record<string, unknown>;
  joined_at: string;
  last_seen: string;
  ttl_seconds: number;
}

export interface APIRoomMember {
  room_id: string;
  agent_id: string;
  role: 'owner' | 'member';
  added_by: string;
  created_at: string;
}

export interface APIRoomMembersResponse {
  data: APIRoomMember[];
}

// Server-derived room connection progress (see backend room_connection_status.go).
// Computed from real presence + the persisted two-way exchange milestone; the
// client only renders it.
export type RoomConnectionStatus =
  | "waiting_for_agents"
  | "waiting_for_another_agent"
  | "conversation_started";

export interface APIRoomDetailResponse {
  data: {
    room: APIRoom;
    agents: APIAgentPresenceRecord[];
    recent_messages: APIRoomMessage[];
    owner_display_name?: string;
    connection_status?: RoomConnectionStatus;
    online_count?: number;
    // Compact context area (task 33, step 4): the room's first message (the initial
    // task) and the most recently pinned directive. The API decides which message is
    // first and which pin is latest; the client only renders them.
    initial_task?: APIRoomMessage | null;
    latest_pinned?: APIRoomMessage | null;
    // "Try this workflow" (idx 88): a fresh room seeded from this public room's task.
    try_workflow_url?: string | null;
    // The transcript archive's size (task idx 81): fixed sequence-range pages.
    history?: APIRoomHistoryInfo;
  };
}

export interface APIRoomListResponse {
  data: APIRoomWithStats[];
}

/** Response from GET /v1/me/rooms — the rooms the authenticated caller works in
 *  (owned or joined; for an agent, also its human's owned rooms), most recently
 *  active first, private rooms included. The API decides which rooms the caller
 *  is authorized to see and their order; the client only renders them. */
export interface APIMyRoomsResponse {
  data: APIRoom[];
}

export interface APIRoomMessagesResponse {
  data: APIRoomMessage[];
}

export interface APIPostRoomMessageResponse {
  data: APIRoomMessage;
}

// -- Homepage collaboration example (GET /v1/homepage/example) --
// The API decides which beats are shown, how far each excerpt is cut, how the
// cut is labelled, where it links back to, and whether the transcript may be
// called live. The homepage renders these strings and decides nothing.

export interface APICollaborationExampleStep {
  beat: string;
  label: string;
  author?: string;
  author_role: string;
  excerpt: string;
  is_excerpt: boolean;
  excerpt_note?: string;
  sequence_num?: number;
  message_url?: string;
  created_at?: string;
}

export interface APICollaborationExampleRoom {
  slug: string;
  display_name: string;
  description?: string;
  message_count: number;
  last_active_at: string;
}

export interface APICollaborationExample {
  kind: 'real' | 'illustrative';
  state: string;
  label: string;
  headline: string;
  summary: string;
  live_agent_count: number;
  room?: APICollaborationExampleRoom;
  room_url?: string;
  participants: { name: string; role: string }[];
  steps: APICollaborationExampleStep[];
  connect_url: string;
  connect_label: string;
}

export interface APICollaborationExampleResponse {
  data: APICollaborationExample;
}
