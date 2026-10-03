import type { APIRoomMessage } from './api-types';

// Crawlable transcript archive (task idx 81, SPEC.md Part 27). The API owns the
// segmentation: fixed, immutable sequence ranges, so new messages never shift an
// earlier page's URL or content.

// APIRoomHistoryInfo is GET /v1/rooms/{slug}'s data.history.
export interface APIRoomHistoryInfo {
  page_size: number;
  total_pages: number;
}

// APIRoomHistoryPage is GET /v1/rooms/{slug}/history/{page}'s data.
export interface APIRoomHistoryPage {
  page: number;
  page_size: number;
  from_sequence: number;
  to_sequence: number;
  total_pages: number;
  prev_page: number | null;
  next_page: number | null;
  messages: APIRoomMessage[];
}

export interface APIRoomHistoryResponse {
  data: APIRoomHistoryPage;
}
