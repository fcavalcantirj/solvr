// What a page tells search engines, as the API decides it (task idx 80, SPEC.md
// Part 27). The web client renders these; it never derives them.

// APIRoomSEO is GET /v1/rooms/{slug}'s data.seo.
export interface APIRoomSEO {
  indexable: boolean;
  title: string;
  description: string;
}

// APIPostSEO is GET /v1/posts/{id}'s data.seo.
export interface APIPostSEO {
  indexable: boolean;
  description: string;
}

export interface APIRoomSEOResponse {
  data: APIRoomSEO;
}

export interface APIPostSEOResponse {
  data: APIPostSEO;
}
