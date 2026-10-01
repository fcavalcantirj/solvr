package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// entityETag derives a strong entity tag from a resource's last-modified time.
// UnixMicro matches PostgreSQL timestamptz precision, so the validator a client
// receives from GET or PATCH is byte-stable across a round trip and can be
// returned unchanged as an If-Match precondition. It is shared by every
// canonical resource that supports conditional edits (posts, replies, rooms).
func entityETag(updatedAt time.Time) string {
	return `"` + strconv.FormatInt(updatedAt.UTC().UnixMicro(), 10) + `"`
}

// postETag derives a strong entity tag for a post from its last-modified time.
func postETag(updatedAt time.Time) string {
	return entityETag(updatedAt)
}

// ifMatchIsStale reports whether the request carries an If-Match precondition
// that no longer matches the resource's current entity tag. A request without
// the header is not stale (requireIfMatch answers that one 428 first), and
// "If-Match: *" matches any existing resource. Weak validators are compared by
// value.
func ifMatchIsStale(r *http.Request, currentETag string) bool {
	candidates := ifMatchCandidates(r)
	if len(candidates) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if candidate == "*" || candidate == currentETag {
			return false
		}
	}
	return true
}

// ifMatchCandidates lists the entity tags of the request's If-Match header,
// with weak prefixes dropped; empty when the header is absent or blank.
func ifMatchCandidates(r *http.Request) []string {
	header := strings.TrimSpace(r.Header.Get("If-Match"))
	if header == "" {
		return nil
	}
	var candidates []string
	for _, raw := range strings.Split(header, ",") {
		candidates = append(candidates, strings.TrimPrefix(strings.TrimSpace(raw), "W/"))
	}
	return candidates
}

// requireIfMatch applies the required If-Match precondition of an edit
// (spec.json idx 74 step 5; owner decision 8 of 2026-09-30) to a resource whose
// current version is updatedAt. Without the header it answers 428
// PRECONDITION_REQUIRED and hands out no ETag, so a client must read the
// resource before it can overwrite it; a stale header is 412
// PRECONDITION_FAILED with the current ETag. Either way it returns ok=false and
// the edit stops. Otherwise it returns the version the write must still find
// (the conditional write's expected updated_at), or nil for "If-Match: *", the
// explicit unconditional edit.
func requireIfMatch(w http.ResponseWriter, r *http.Request, updatedAt time.Time, resource string,
	writeError func(http.ResponseWriter, int, string, string)) (expected *time.Time, ok bool) {
	candidates := ifMatchCandidates(r)
	if len(candidates) == 0 {
		writeError(w, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED",
			"If-Match is required: send the ETag from your last read of this "+resource+" so the edit cannot overwrite a newer version")
		return nil, false
	}
	current := entityETag(updatedAt)
	if ifMatchIsStale(r, current) {
		w.Header().Set("ETag", current)
		writeError(w, http.StatusPreconditionFailed, "PRECONDITION_FAILED",
			resource+" was modified since you last read it; refetch and retry")
		return nil, false
	}
	for _, candidate := range candidates {
		if candidate == "*" {
			return nil, true
		}
	}
	return &updatedAt, true
}

// answerVersionConflict answers 412 PRECONDITION_FAILED, with the current ETag,
// when err is a conditional write that lost to a concurrent writer after the
// precondition passed; it reports whether it answered.
func answerVersionConflict(w http.ResponseWriter, err error, resource string,
	writeError func(http.ResponseWriter, int, string, string)) bool {
	var conflict *models.VersionConflictError
	if !errors.As(err, &conflict) {
		return false
	}
	w.Header().Set("ETag", entityETag(conflict.Current))
	writeError(w, http.StatusPreconditionFailed, "PRECONDITION_FAILED",
		resource+" was modified since you last read it; refetch and retry")
	return true
}

// enforceIfMatch applies the required If-Match precondition of a post edit
// (see requireIfMatch).
func enforceIfMatch(w http.ResponseWriter, r *http.Request, updatedAt time.Time) (*time.Time, bool) {
	return requireIfMatch(w, r, updatedAt, "post", writePostsError)
}
