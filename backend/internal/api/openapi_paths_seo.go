package api

// Search-visibility operations of the OpenAPI contract (task idx 80, SPEC.md Part 27):
// what a post or room page tells search engines. Served apart from the post and room
// reads, so the canonical read operations and their consumers are unchanged.

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
			"indexable", indexable, "description", description,
		), "indexable", "description")), "data"),
		"RoomSEOResponse", objectOf(obj("data", objectOf(obj(
			"indexable", indexable,
			"title", typed("string", "description", "The page title: the room's name."),
			"description", description,
		), "indexable", "title", "description")), "data"),
	)
}
