// Search API types (GET /v1/search), re-exported by api-types.ts.

import type { APIAuthor, APIPost } from './api-types';

// A reply whose text matched a search, anchored in its post (GET /v1/search). The API
// computes everything shown: the link to the reply on its post, the excerpt (matched words
// wrapped in <mark>), the original author and time, and the origin of a migrated contribution.
export interface APISearchReplyMatch {
  id: string;
  post_id: string;
  url: string;
  snippet: string;
  author: APIAuthor;
  legacy_type?: string;
  legacy_status?: string;
  score: number;
  similarity?: number;
  created_at: string;
}

// One search result: a post, with the replies that matched listed under it.
export type APISearchResult = APIPost & {
  snippet: string;
  score: number;
  matched_replies?: APISearchReplyMatch[];
};

export interface APISearchResponse {
  data: APISearchResult[];
  meta: {
    query: string;
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
    took_ms: number;
    // Indicates search method: 'hybrid' (semantic + keyword) or 'fulltext' (keyword only)
    method: 'hybrid' | 'fulltext';
  };
}
