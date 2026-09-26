package handlers

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// rejectUnknownQuery writes a 400 VALIDATION_ERROR naming the first query parameter not
// in allowed and returns false; a paging or filter parameter a route does not understand
// is refused rather than silently ignored. A credential in the URL never reaches here:
// RefuseURLCredentials answers it first.
func rejectUnknownQuery(w http.ResponseWriter, q url.Values, allowed ...string) bool {
	names := make([]string, 0, len(q))
	for name := range q {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !slices.Contains(allowed, name) {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR",
				"unknown query parameter: "+name+" (supported: "+strings.Join(allowed, ", ")+")")
			return false
		}
	}
	return true
}

// parseEntryPage reads ?cursor= and ?limit= with the API-wide default 50 and maximum 100.
// A malformed cursor or limit writes a 400 and returns ok=false.
func parseEntryPage(w http.ResponseWriter, q url.Values) (afterSequence, limit int, ok bool) {
	if c := q.Get("cursor"); c != "" {
		seq, valid := decodeEntryCursor(c)
		if !valid {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid cursor")
			return 0, 0, false
		}
		afterSequence = seq
	}
	limit = defaultEntryPageLimit
	if l := q.Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed <= 0 {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be a positive integer")
			return 0, 0, false
		}
		limit = min(parsed, maxEntryPageLimit)
	}
	return afterSequence, limit, true
}

// listEntryPage reads one forward page (p.Limit entries) and reports whether more follow.
// nextCursor is the opaque cursor after the page's last entry, nil when hasMore=false.
func listEntryPage(ctx context.Context, repo *db.RoomEntryRepository, p models.RoomEntryPageParams) (entries []models.RoomEntry, hasMore bool, nextCursor any, err error) {
	limit := p.Limit
	p.Limit = limit + 1
	entries, err = repo.ListPage(ctx, p)
	if err != nil {
		return nil, false, nil, err
	}
	if len(entries) > limit {
		entries = entries[:limit]
		return entries, true, encodeEntryCursor(entries[len(entries)-1].Sequence), nil
	}
	return entries, false, nil, nil
}
