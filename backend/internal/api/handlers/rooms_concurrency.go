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

// enforceRoomIfMatch applies the required If-Match precondition of a room
// edit (see requireIfMatch): 428 without it, 412 with the current ETag when it
// is stale, else the version the conditional write must still find (nil for
// "If-Match: *"). Callers run it AFTER the manage-permission check, so a
// non-manager of a private room gets 403 and learns nothing about the room's
// version.
func enforceRoomIfMatch(w http.ResponseWriter, r *http.Request, updatedAt time.Time) (*time.Time, bool) {
	return requireIfMatch(w, r, updatedAt, "room", roomWriteError)
}
