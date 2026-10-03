import type { APIRoomMessage } from './api-types';

// GET /v1/rooms/{slug}/viewer — what the calling person or agent may do in a room. The
// API decides it; the page only shows the controls it is told to (idx 92).
export interface APIRoomViewer {
  can_pin: boolean;
  // Opt-in room notifications: available to a signed-in person or an agent only.
  notifications?: APIRoomViewerNotifications;
}

export interface APIRoomViewerNotifications {
  available: boolean;
  subscribed: boolean;
  paused: boolean;
}

// GET|PUT|DELETE /v1/rooms/{slug}/notifications — this caller's opt-in for the room.
export interface APIRoomNotificationState {
  subscribed: boolean;
  paused: boolean;
  events: string[];
  off: string;
}

export interface APIRoomNotificationStateResponse {
  data: APIRoomNotificationState;
}

// GET|PATCH /v1/me/notification-settings — the global room-notification switch.
export interface APINotificationSettings {
  room_notifications: 'on' | 'paused';
}

export interface APINotificationSettingsResponse {
  data: APINotificationSettings;
}

// One item of GET /v1/notifications, as the API answers it (SPEC.md Part 5.6).
export interface APIUserNotification {
  id: string;
  type: string;
  title: string;
  body: string;
  link: string;
  read_at: string | null;
  created_at: string;
  schema_version?: number;
  subject?: { post_id?: string; reply_id?: string; room_id?: string; entry_id?: number };
}

export interface APIUserNotificationsResponse {
  data: APIUserNotification[];
  meta: { total: number; page: number; per_page: number; has_more: boolean };
}

export interface APIRoomViewerResponse {
  data: APIRoomViewer;
}

// POST|DELETE /v1/rooms/{slug}/entries/{entry_id}/pin — the entry as it now stands, and the
// directive now in force (a revision may stand in for the pinned entry), or null.
export interface APIPinEntryResponse {
  data: APIRoomMessage;
  meta: { latest_pinned: APIRoomMessage | null };
}
