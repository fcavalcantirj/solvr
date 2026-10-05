package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/seo"
)

// servedBlogPost is a blog post as every answer that carries one serves it (SPEC.md 27.1):
// meta_description is the author's, or one composed from the body, and the excerpt is plain
// text. It is composed at read time, so posts written before the rule are served the same way;
// it works on a copy, so what a handler writes back (an update) is what was stored.
func servedBlogPost(p models.BlogPost) models.BlogPost {
	p.MetaDescription = seo.BlogDescription(p.MetaDescription, p.Title, p.Body)
	p.Excerpt = seo.BlogExcerpt(p.Excerpt, p.Body)
	return p
}

// servedBlogPostWithAuthor is servedBlogPost for a read that carries the author.
func servedBlogPostWithAuthor(p models.BlogPostWithAuthor) models.BlogPostWithAuthor {
	p.BlogPost = servedBlogPost(p.BlogPost)
	return p
}

// writeBlogJSON writes a JSON response for blog endpoints.
func writeBlogJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeBlogError writes an error JSON response for blog endpoints.
func writeBlogError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// validSlugRegex matches URL-friendly slugs: lowercase alphanumeric and hyphens.
var validSlugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validateSlug checks if a slug is valid for use in URLs.
func validateSlug(slug string) bool {
	if slug == "" || len(slug) > 200 {
		return false
	}
	return validSlugRegex.MatchString(slug)
}
