import type { APIRoomMessage } from './api-types';

// GET /v1/rooms/{slug}/viewer — what the calling person or agent may do in a room. The
// API decides it; the page only shows the controls it is told to (idx 92).
export interface APIRoomViewer {
  can_pin: boolean;
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
