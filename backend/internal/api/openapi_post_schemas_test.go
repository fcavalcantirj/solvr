package api

import (
	"reflect"
	"sort"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
)

// idx 78 step 1 (slice 2): the post and search schemas every client reads are the shapes the
// handlers serialize, pinned to their Go types like the room and reply schemas, so a field
// cannot be added, renamed or dropped without the published contract changing with it.
func TestOpenAPIPosts_SchemasDescribeTheJSONTheHandlersReturn(t *testing.T) {
	spec := servedSpec(t)
	for schema, typ := range map[string]reflect.Type{
		"Post":              reflect.TypeOf(models.PostWithAuthor{}),
		"PostAuthor":        reflect.TypeOf(models.PostAuthor{}),
		"CreatePostRequest": reflect.TypeOf(handlers.CreatePostRequest{}),
		"PostsListResponse": reflect.TypeOf(handlers.PostsListResponse{}),
		"PostsListMeta":     reflect.TypeOf(handlers.PostsListMeta{}),
		"SearchResponse":    reflect.TypeOf(handlers.SearchResponse{}),
		"SearchMeta":        reflect.TypeOf(handlers.SearchResponseMeta{}),
		"SearchResult":      reflect.TypeOf(models.SearchResultResponse{}),
		"SearchAuthor":      reflect.TypeOf(models.SearchAuthor{}),
		"SearchReplyMatch":  reflect.TypeOf(models.SearchReplyMatch{}),
	} {
		want := jsonFields(typ)
		sort.Strings(want)
		sameSet(t, schema+" properties", propertyNames(t, spec, schema), want)
	}
}

// A post is created without a type (or with type post): the legacy problem, question and idea
// types were retired (idx 68). Create answers the stored post; the reads add its author and counts, so those
// are documented but not required.
func TestOpenAPIPosts_CreateTakesNoTypeAndThePostShowsTheCanonicalStates(t *testing.T) {
	spec := servedSpec(t)
	create := at(t, spec, "components", "schemas", "CreatePostRequest").(map[string]interface{})
	assert.ElementsMatch(t, []interface{}{"title", "description"}, create["required"])
	assert.ElementsMatch(t, []interface{}{"post"}, at(t, create, "properties", "type", "enum"))

	post := at(t, spec, "components", "schemas", "Post").(map[string]interface{})
	for _, field := range []string{"id", "type", "title", "description", "status", "publication_state",
		"moderation_state", "posted_by_type", "posted_by_id", "created_at", "updated_at"} {
		assert.Contains(t, post["required"], field, "Post must require %s", field)
	}
	for _, field := range []string{"author", "vote_score", "reply_count", "user_vote"} {
		assert.NotContains(t, post["required"], field, "create answers no %s, so Post may not require it", field)
	}
	assert.ElementsMatch(t, []interface{}{"draft", "published", "archived"}, at(t, post, "properties", "publication_state", "enum"))
	assert.ElementsMatch(t, []interface{}{"pending", "approved", "rejected"}, at(t, post, "properties", "moderation_state", "enum"))
	assert.Equal(t, true, at(t, post, "properties", "user_vote", "nullable"))
}

// GET /v1/search documents exactly the query parameters the handler reads: a documented one it
// ignores, or one it reads that a client cannot find, both fail.
func TestOpenAPISearch_DocumentsExactlyTheParametersTheHandlerReads(t *testing.T) {
	spec := servedSpec(t)
	search := operation(t, spec, "get", "/search")
	var documented []string
	for _, raw := range search["parameters"].([]interface{}) {
		p := deref(t, spec, raw).(map[string]interface{})
		if assert.Equal(t, "query", p["in"]) {
			documented = append(documented, p["name"].(string))
		}
	}
	sameSet(t, "GET /search query parameters", documented, handlers.SearchParamNames())
	assert.Equal(t, "#/components/schemas/SearchResponse",
		at(t, search, "responses", "200", "content", "application/json", "schema", "$ref"))
}

// SPEC.md 27.2: GET /v1/posts documents exactly the query parameters the handler reads
// (handlers.PostListParamNames), indexable among them, and its meta names total_pages, which
// the post archive pages read to know their last page.
func TestOpenAPIPosts_ListDocumentsExactlyTheParametersTheHandlerReads(t *testing.T) {
	spec := servedSpec(t)
	list := operation(t, spec, "get", "/posts")
	var documented []string
	described := map[string]string{}
	for _, raw := range list["parameters"].([]interface{}) {
		p := deref(t, spec, raw).(map[string]interface{})
		if assert.Equal(t, "query", p["in"]) {
			documented = append(documented, p["name"].(string))
			described[p["name"].(string)], _ = p["description"].(string)
		}
	}
	sameSet(t, "GET /posts query parameters", documented, handlers.PostListParamNames())
	assert.Contains(t, described["indexable"], "sitemap", "indexable is described by the rule it applies")

	assert.Equal(t, "#/components/schemas/PostsListResponse",
		at(t, list, "responses", "200", "content", "application/json", "schema", "$ref"))
	assert.Equal(t, "#/components/schemas/PostsListMeta",
		at(t, spec, "components", "schemas", "PostsListResponse", "properties", "meta", "$ref"))
	meta := at(t, spec, "components", "schemas", "PostsListMeta").(map[string]interface{})
	assert.ElementsMatch(t, []interface{}{"total", "page", "per_page", "total_pages", "has_more"}, meta["required"])
	assert.Equal(t, "integer", at(t, meta, "properties", "total_pages", "type"))
}

// GET /v1/me/posts answers another meta (no has_more, no total_pages), so it keeps the
// shared pagination schema: only the canonical list promises total_pages.
func TestOpenAPIPosts_OnlyTheCanonicalListPromisesTotalPages(t *testing.T) {
	spec := servedSpec(t)
	assert.NotContains(t, propertyNames(t, spec, "PaginationMeta"), "total_pages")
	assert.Equal(t, "#/components/schemas/PostsResponse",
		at(t, operation(t, spec, "get", "/me/posts"), "responses", "200", "content", "application/json", "schema", "$ref"))
}
