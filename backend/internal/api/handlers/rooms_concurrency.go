package handlers

import (
	"net/http"
	"time"
)

// roomETag derives a strong entity tag for a room from its last-modified time,
// using the same shared format as every other conditional resource. A room's
// updated_at also advances on activity (with each message, migration 000112), so
// the validator tracks the whole representation GET returns, including
// last_active_at: an owner editing settings in a busy room refetches and
// retries rather than overwriting a revision they never saw.
func roomETag(updatedAt time.Time) string {
	return entityETag(updatedAt)
}

// enforceRoomIfMatch writes a 412 response and returns true when the request
// carries a stale If-Match precondition for a room whose current version is
// derived from updatedAt; it echoes the current validator so the client can
// refetch and retry. It returns false when the edit may proceed.
//
// If-Match is opt-in (an absent header never blocks). Callers run it AFTER the
// manage-permission check, so a non-manager of a private room gets 403 and
// learns nothing about the room's version (idx 73 step 5).
func enforceRoomIfMatch(w http.ResponseWriter, r *http.Request, updatedAt time.Time) bool {
	current := roomETag(updatedAt)
	if ifMatchIsStale(r, current) {
		w.Header().Set("ETag", current)
		roomWriteError(w, http.StatusPreconditionFailed, "PRECONDITION_FAILED",
			"room was modified since you last read it; refetch and retry")
		return true
	}
	return false
}
