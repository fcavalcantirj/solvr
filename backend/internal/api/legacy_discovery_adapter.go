package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// legacyTypedListAdapter serves a legacy typed discovery list (GET /v1/problems,
// /v1/questions, /v1/ideas — family "legacy-typed-discovery" in RouteFamilies) through the
// canonical GET /v1/posts list, so filters, ordering, pagination and visibility are
// defined once (task idx 71 step 3). The adapter only translates the legacy query shape:
// it pins the path's post type and keeps the old lenient pagination (invalid values fall
// back to defaults, per_page above the cap is clamped) instead of the canonical 400.
func legacyTypedListAdapter(canonicalList http.HandlerFunc, postType models.PostType) http.HandlerFunc {
	successor := fmt.Sprintf(`</v1/posts?type=%s>; rel="successor-version"`, postType)
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		q.Set("type", string(postType))
		if n, err := strconv.Atoi(q.Get("page")); err != nil || n < 1 {
			q.Del("page")
		}
		if n, err := strconv.Atoi(q.Get("per_page")); err != nil || n < 1 {
			q.Del("per_page")
		} else if n > handlers.MaxPerPage {
			q.Set("per_page", strconv.Itoa(handlers.MaxPerPage))
		}

		r2 := r.Clone(r.Context())
		r2.URL.RawQuery = q.Encode()

		w.Header().Set("Deprecation", "true")
		w.Header().Set("Link", successor)
		canonicalList(w, r2)
	}
}
