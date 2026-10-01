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

// enforceReplyIfMatch applies the required If-Match precondition of a reply
// edit (see requireIfMatch): 428 without it, 412 with the current ETag when it
// is stale, else the version the conditional write must still find (nil for
// "If-Match: *"), so a retry carrying the version it last read cannot overwrite
// a newer revision of the reply. Callers run it AFTER the author check, like
// posts and rooms, so a non-author is told 403 rather than to fetch an ETag.
func enforceReplyIfMatch(w http.ResponseWriter, r *http.Request, updatedAt time.Time) (*time.Time, bool) {
	return requireIfMatch(w, r, updatedAt, "reply", writeRepliesError)
}
