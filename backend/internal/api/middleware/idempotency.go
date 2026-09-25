package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

const (
	// IdempotencyKeyHeader is the client-chosen key that makes a create retry-safe.
	IdempotencyKeyHeader = "Idempotency-Key"
	// IdempotentReplayedHeader marks a response replayed from a stored create result.
	IdempotentReplayedHeader = "Idempotent-Replayed"
	// maxIdempotencyKeyLen bounds the key (it is part of the primary key).
	maxIdempotencyKeyLen = 255
)

// IdempotencyStore is the durable store of keyed create results
// (db.IdempotencyRepository in production).
type IdempotencyStore interface {
	// Reserve claims the scope for a new request. It returns reserved=true when the
	// caller now owns the key, otherwise the existing record (pending or completed).
	Reserve(ctx context.Context, scope models.IdempotencyScope, requestHash string) (*models.IdempotencyRecord, bool, error)
	// Complete stores the successful response of a reserved key for replay.
	Complete(ctx context.Context, scope models.IdempotencyScope, status int, contentType string, body []byte) error
	// Release drops a reserved key whose request did not succeed, so it can be retried.
	Release(ctx context.Context, scope models.IdempotencyScope) error
}

// Idempotency makes a create route honor the Idempotency-Key header (idx 73 step 4).
// A key is scoped to the authenticated actor and to operation. The first request with
// a key runs the handler; a 2xx result is stored and every retry with the same key and
// the same method, path and body replays it (Idempotent-Replayed: true) without running
// the handler again. Reusing a key with a different payload is 409
// IDEMPOTENCY_KEY_REUSED; a retry while the original is still running is 409
// IDEMPOTENCY_REQUEST_IN_PROGRESS with Retry-After. A non-2xx result is not stored,
// so the key stays usable for a corrected retry.
//
// Requests without the header, anonymous requests (the handler answers 401) and a nil
// store pass straight through. A store failure fails closed with 503: the handler is
// never run without the dedupe the client asked for.
func Idempotency(store IdempotencyStore, operation string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if store == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, present := r.Header[http.CanonicalHeaderKey(IdempotencyKeyHeader)]
			if !present {
				next.ServeHTTP(w, r)
				return
			}
			actorType, actorID := idempotencyActor(r)
			if actorID == "" {
				next.ServeHTTP(w, r)
				return
			}
			k := strings.TrimSpace(strings.Join(key, ","))
			if k == "" || len(k) > maxIdempotencyKeyLen {
				writeIdempotencyError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Idempotency-Key must be 1-255 characters")
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					writeBodyLimitError(w)
					return
				}
				writeIdempotencyError(w, http.StatusBadRequest, "VALIDATION_ERROR", "failed to read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			scope := models.IdempotencyScope{ActorType: actorType, ActorID: actorID, Operation: operation, Key: k}
			hash := requestHash(r, body)
			existing, reserved, err := store.Reserve(r.Context(), scope, hash)
			if err != nil {
				slog.Error("idempotency reserve failed", "error", err, "operation", operation)
				w.Header().Set("Retry-After", "1")
				writeIdempotencyError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "could not record the Idempotency-Key; retry the request")
				return
			}
			if !reserved {
				replayOrConflict(w, existing, hash)
				return
			}
			runReserved(w, r, next, store, scope)
		})
	}
}

// runReserved runs the handler for a freshly reserved key and stores or releases it.
func runReserved(w http.ResponseWriter, r *http.Request, next http.Handler, store IdempotencyStore, scope models.IdempotencyScope) {
	// The outcome is recorded even if the client disconnects mid-request.
	ctx := context.WithoutCancel(r.Context())
	rec := &idempotencyRecorder{ResponseWriter: w}
	finished := false
	defer func() {
		if !finished {
			// Panic: free the key so the retry is not locked out, then re-panic.
			_ = store.Release(ctx, scope)
		}
	}()
	next.ServeHTTP(rec, r)
	finished = true

	if rec.status() >= 200 && rec.status() < 300 {
		if err := store.Complete(ctx, scope, rec.status(), rec.Header().Get("Content-Type"), rec.body.Bytes()); err != nil {
			slog.Error("idempotency complete failed", "error", err, "operation", scope.Operation)
			_ = store.Release(ctx, scope)
		}
		return
	}
	if err := store.Release(ctx, scope); err != nil {
		slog.Error("idempotency release failed", "error", err, "operation", scope.Operation)
	}
}

// replayOrConflict answers a request whose key is already recorded.
func replayOrConflict(w http.ResponseWriter, existing *models.IdempotencyRecord, hash string) {
	if existing.RequestHash != hash {
		writeIdempotencyError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different request")
		return
	}
	if existing.Status != models.IdempotencyCompleted {
		w.Header().Set("Retry-After", "1")
		writeIdempotencyError(w, http.StatusConflict, "IDEMPOTENCY_REQUEST_IN_PROGRESS", "a request with this Idempotency-Key is still in progress")
		return
	}
	if existing.ResponseContentType != "" {
		w.Header().Set("Content-Type", existing.ResponseContentType)
	}
	w.Header().Set(IdempotentReplayedHeader, "true")
	w.WriteHeader(existing.ResponseStatus)
	_, _ = w.Write(existing.ResponseBody)
}

// idempotencyActor returns the authenticated actor the key is scoped to.
func idempotencyActor(r *http.Request) (string, string) {
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		return string(models.AuthorTypeAgent), agent.ID
	}
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		return string(models.AuthorTypeHuman), claims.UserID
	}
	return "", ""
}

// requestHash fingerprints what a retry must repeat exactly: method, path and body.
// The path matters for nested creates (the same key on another post's replies is a
// different request).
func requestHash(r *http.Request, body []byte) string {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte{0})
	h.Write([]byte(r.URL.Path))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func writeIdempotencyError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{"code": code, "message": message},
	})
}

// idempotencyRecorder passes the response through while keeping a copy to store.
type idempotencyRecorder struct {
	http.ResponseWriter
	code int
	body bytes.Buffer
}

func (rw *idempotencyRecorder) WriteHeader(status int) {
	if rw.code == 0 {
		rw.code = status
	}
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *idempotencyRecorder) Write(b []byte) (int, error) {
	if rw.code == 0 {
		rw.code = http.StatusOK
	}
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func (rw *idempotencyRecorder) status() int {
	if rw.code == 0 {
		return http.StatusOK
	}
	return rw.code
}
