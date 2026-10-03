// What a page tells search engines, as the API decides it (task idx 80, SPEC.md
// Part 27). The web client renders these; it never derives them.

// APIRoomSEO is GET /v1/rooms/{slug}/seo's data.
export interface APIRoomSEO {
  indexable: boolean;
  title: string;
  description: string;
}

// APIPostSEO is GET /v1/posts/{id}/seo's data.
export interface APIPostSEO {
  indexable: boolean;
  // Unique among indexable posts (task idx 82).
  title: string;
  description: string;
}

// APIPostSourceRoom is GET /v1/posts/{id}/rooms's source_room: the public room the
// post was saved from (task idx 82).
export interface APIPostSourceRoom {
  slug: string;
  display_name: string;
}

export interface APIRoomSEOResponse {
  data: APIRoomSEO;
}

export interface APIPostSEOResponse {
  data: APIPostSEO;
}
