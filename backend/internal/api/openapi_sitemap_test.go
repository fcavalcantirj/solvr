package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// SPEC.md 27.1 (Sitemaps): a failed database read of a sitemap list or of the counts is a
// retryable 503 with Retry-After, never a 500, and the contract says so, so a client knows to
// come back rather than treat the site as having no pages.
func TestOpenAPI_SitemapReadsDocumentTheirRetryableFailure(t *testing.T) {
	spec := servedSpec(t)
	for _, path := range []string{"/sitemap/urls", "/sitemap/counts"} {
		op := operation(t, spec, "get", path)
		assert.Equal(t, "#/components/responses/ServiceUnavailable", refName(at(t, op, "responses", "503")),
			"GET %s: a failed database read is 503", path)
		assert.Contains(t, op["description"], "Retry-After", "GET %s says when to come back", path)
		assert.NotContains(t, op["responses"], "500", "GET %s never documents a 500", path)
	}
	urls := operation(t, spec, "get", "/sitemap/urls")
	assert.Equal(t, "#/components/responses/BadRequest", refName(at(t, urls, "responses", "400")),
		"an unknown type or a page out of range is 400")
	for _, name := range []string{"type", "page", "per_page"} {
		assert.Equal(t, "query", param(t, spec, urls, name)["in"], name)
	}
}
