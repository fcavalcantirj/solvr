package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// parseRoomListParams defines the filters, ordering and pagination of the
// canonical GET /v1/rooms room-discovery list ONCE (task idx 72 step 2). The
// clamps are deliberately lenient: an out-of-range, non-integer or unknown value
// falls back to the default rather than erroring, so a bookmarked discovery link
// never breaks. Visibility (private-room exclusion) lives in the single query
// path db.RoomRepository.ListFiltered.
func parseRoomListParams(r *http.Request) db.RoomListParams {
	q := r.URL.Query()

	limit := 20
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Sort control: Recent (default) or Active now. Unknown values fall back to
	// recent rather than erroring, matching the lenient limit/offset parsing.
	sort := "recent"
	if q.Get("sort") == "active" {
		sort = "active"
	}

	return db.RoomListParams{
		Limit:           limit,
		Offset:          offset,
		Sort:            sort,
		Query:           strings.TrimSpace(q.Get("q")),
		IncludeArchived: q.Get("include_archived") == "true",
	}
}
