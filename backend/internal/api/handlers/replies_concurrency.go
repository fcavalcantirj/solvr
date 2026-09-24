package handlers

import (
	"net/http"
	"time"
)

// replyETag derives a strong entity tag for a canonical reply from its
// last-modified time, using the same shared format as every other conditional
// resource.
func replyETag(updatedAt time.Time) string {
	return entityETag(updatedAt)
}

// enforceReplyIfMatch writes a 412 response and returns true when the request
// carries a stale If-Match precondition for a reply whose current version is
// derived from updatedAt; it echoes the current validator so the client can
// refetch and retry. It returns false when the edit may proceed.
//
// If-Match is opt-in (an absent header never blocks), so a retry that carries
// the version it last read cannot silently overwrite a newer revision of the
// reply (idx 73 step 5). The precondition is checked before repo ownership
// enforcement: a reply's version is not secret (the public GET already exposes
// updated_at), so returning 412 to a non-owner leaks nothing, while the actual
// write remains blocked by the author-only check inside the repository.
func enforceReplyIfMatch(w http.ResponseWriter, r *http.Request, updatedAt time.Time) bool {
	current := replyETag(updatedAt)
	if ifMatchIsStale(r, current) {
		w.Header().Set("ETag", current)
		writeRepliesError(w, http.StatusPreconditionFailed, "PRECONDITION_FAILED",
			"reply was modified since you last read it; refetch and retry")
		return true
	}
	return false
}
