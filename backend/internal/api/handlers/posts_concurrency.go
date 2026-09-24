package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// entityETag derives a strong entity tag from a resource's last-modified time.
// UnixMicro matches PostgreSQL timestamptz precision, so the validator a client
// receives from GET or PATCH is byte-stable across a round trip and can be
// returned unchanged as an If-Match precondition. It is shared by every
// canonical resource that supports conditional edits (posts, replies).
func entityETag(updatedAt time.Time) string {
	return `"` + strconv.FormatInt(updatedAt.UTC().UnixMicro(), 10) + `"`
}

// postETag derives a strong entity tag for a post from its last-modified time.
func postETag(updatedAt time.Time) string {
	return entityETag(updatedAt)
}

// ifMatchIsStale reports whether the request carries an If-Match precondition
// that no longer matches the resource's current entity tag. If-Match is
// optional: a request without the header is never stale, and "If-Match: *"
// matches any existing resource. Weak validators are compared by value. This
// lets a client opt in to lost-update protection so a retry cannot silently
// overwrite a newer revision (idx 73 step 5).
func ifMatchIsStale(r *http.Request, currentETag string) bool {
	header := strings.TrimSpace(r.Header.Get("If-Match"))
	if header == "" {
		return false
	}
	for _, raw := range strings.Split(header, ",") {
		candidate := strings.TrimPrefix(strings.TrimSpace(raw), "W/")
		if candidate == "*" || candidate == currentETag {
			return false
		}
	}
	return true
}

// enforceIfMatch writes a 412 response and returns true when the request
// carries a stale If-Match precondition for a post whose current version is
// derived from updatedAt; it echoes the current validator so the client can
// refetch and retry. It returns false when the edit may proceed.
func enforceIfMatch(w http.ResponseWriter, r *http.Request, updatedAt time.Time) bool {
	currentETag := postETag(updatedAt)
	if ifMatchIsStale(r, currentETag) {
		w.Header().Set("ETag", currentETag)
		writePostsError(w, http.StatusPreconditionFailed, "PRECONDITION_FAILED",
			"post was modified since you last read it; refetch and retry")
		return true
	}
	return false
}
