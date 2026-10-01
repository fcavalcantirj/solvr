package handlers

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Opaque cursor pagination for the canonical reply list (idx 73 step 2). The
// contract matches the room entries surface: default limit 50, maximum 100, an
// opaque cursor, and next_cursor/has_more in the response meta.
const (
	defaultReplyPageLimit = 50
	maxReplyPageLimit     = 100
	// replyCursorPrefix versions the opaque cursor so a future keyset change can
	// reject stale cursors instead of silently mis-paging.
	replyCursorPrefix = "rpc1:"
)

// parseReplyPage reads ?cursor= and ?limit= for the reply lists (a post's replies, and
// one author's replies, GET /v1/replies, which pages the same keyset newest first). A malformed
// cursor or a non-positive/invalid limit writes a 400 VALIDATION_ERROR and
// returns ok=false. The cursor is opaque: afterCreatedAt/afterID are the decoded
// keyset position, nil when no cursor was supplied.
func parseReplyPage(w http.ResponseWriter, q url.Values) (afterCreatedAt *time.Time, afterID string, limit int, ok bool) {
	limit = defaultReplyPageLimit
	if l := q.Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed <= 0 {
			writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be a positive integer")
			return nil, "", 0, false
		}
		limit = min(parsed, maxReplyPageLimit)
	}
	if c := q.Get("cursor"); c != "" {
		t, id, valid := decodeReplyCursor(c)
		if !valid {
			writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid cursor")
			return nil, "", 0, false
		}
		afterCreatedAt = &t
		afterID = id
	}
	return afterCreatedAt, afterID, limit, true
}

// encodeReplyCursor encodes the (created_at, id) keyset of the last reply on a
// page into an opaque, URL-safe cursor. created_at is stored at microsecond
// precision, so UnixMicro round-trips exactly against the timestamptz column.
func encodeReplyCursor(createdAt time.Time, id string) string {
	raw := replyCursorPrefix + strconv.FormatInt(createdAt.UnixMicro(), 10) + ":" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeReplyCursor reverses encodeReplyCursor. It reports valid=false for any
// cursor that is not a base64 payload with the current prefix, a parseable
// microsecond timestamp, and a non-empty id.
func decodeReplyCursor(cursor string) (time.Time, string, bool) {
	rawBytes, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", false
	}
	raw := string(rawBytes)
	if !strings.HasPrefix(raw, replyCursorPrefix) {
		return time.Time{}, "", false
	}
	rest := strings.TrimPrefix(raw, replyCursorPrefix)
	micros, id, found := strings.Cut(rest, ":")
	if !found || id == "" {
		return time.Time{}, "", false
	}
	usec, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return time.Time{}, "", false
	}
	return time.UnixMicro(usec), id, true
}
