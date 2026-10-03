package api

// Search-visibility operations of the OpenAPI contract (tasks idx 80-81, SPEC.md Part 27):
// what a post or room page tells search engines, and the crawlable transcript archive.
// Served apart from the post and room reads, so the canonical read operations and their
// consumers are unchanged.

func seoPaths() map[string]interface{} {
	return obj(
		"/posts/{id}/seo", obj(
			"get", obj(
				"summary", "A post page's search verdict", "operationId", "getPostSEO", "tags", []string{"Posts"},
				"security", anonymousOrBearer(),
				"description", "Whether search engines may index the post's page (exactly the posts the sitemap lists) and its search description. Answers 404 exactly when GET /posts/{id} does for the same caller.",
				"parameters", []map[string]interface{}{idParam("Post ID")},
				"responses", withErrors(obj("200", jsonOK("Search verdict", "PostSEOResponse", nil)), "401", "404"),
			),
		),
		"/rooms/{slug}/history/{page}", obj(
			"get", obj(
				"summary", "One page of a room's transcript archive", "operationId", "getRoomHistoryPage", "tags", []string{"Rooms"},
				"security", anonymousOrBearer(),
				"description", "Page N holds the room's live messages with sequence numbers (N-1)*100+1 through N*100, in sequence order. Ranges are immutable: new messages never move an earlier page; a deleted message leaves a gap. A page number not written as a positive integer without leading zeros, or beyond the last page, is 404. Same read policy as GET /rooms/{slug}.",
				"parameters", []map[string]interface{}{slugParam(), pathParam("page", "Page number, from 1.", obj("type", "integer", "minimum", 1))},
				"responses", withErrors(obj("200", jsonOK("Transcript page", "RoomHistoryResponse", nil)), "401", "403", "404"),
			),
		),
		"/rooms/{slug}/seo", obj(
			"get", obj(
				"summary", "A room page's search verdict", "operationId", "getRoomSEO", "tags", []string{"Rooms"},
				"security", anonymousOrBearer(),
				"description", "Whether search engines may index the room's page (public, live, and carrying a two-way exchange: messages from two distinct authors; the rooms sitemap lists by the same rule), its title and its search description. Same read policy as GET /rooms/{slug}.",
				"parameters", []map[string]interface{}{slugParam()},
				"responses", withErrors(obj("200", jsonOK("Search verdict", "RoomSEOResponse", nil)), "401", "403", "404"),
			),
		),
	)
}

func seoSchemas() map[string]interface{} {
	indexable := typed("boolean", "description", "Whether search engines may index the page.")
	description := typed("string", "description", "The page's search description from its visible content, at most 160 characters.")
	return obj(
		"PostSEOResponse", objectOf(obj("data", objectOf(obj(
			"indexable", indexable,
			"title", typed("string", "description", "The page title, unique among indexable posts: a title another indexable post shares names its author, and one the same author reused also names its date."),
			"description", description,
		), "indexable", "title", "description")), "data"),
		"RoomHistoryResponse", objectOf(obj("data", objectOf(obj(
			"page", typed("integer"), "page_size", typed("integer"),
			"from_sequence", typed("integer"), "to_sequence", typed("integer"),
			"total_pages", typed("integer"),
			"prev_page", nullable("integer"), "next_page", nullable("integer"),
			"messages", typed("array", "items", typed("object"), "description", "The range's live message entries, in sequence order."),
		), "page", "page_size", "from_sequence", "to_sequence", "total_pages", "prev_page", "next_page", "messages")), "data"),
		"RoomSEOResponse", objectOf(obj("data", objectOf(obj(
			"indexable", indexable,
			"title", typed("string", "description", "The page title: the room's name."),
			"description", description,
		), "indexable", "title", "description")), "data"),
	)
}
