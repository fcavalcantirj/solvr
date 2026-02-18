package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// BodyLimit returns a middleware that limits the size of request bodies.
// Requests with Content-Length exceeding maxBytes will be rejected with 413.
// Requests without Content-Length will have their body wrapped in a LimitReader.
// Multipart/form-data requests are exempt — upload handlers enforce their own
// size limits via http.MaxBytesReader with configurable MAX_UPLOAD_SIZE_BYTES.
// Per FIX-028: Prevent accepting arbitrarily large payloads.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip body check for requests without bodies
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Skip body limit for multipart uploads — they have their own
			// size limits via http.MaxBytesReader in the upload handler.
			contentType := r.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "multipart/form-data") {
				next.ServeHTTP(w, r)
				return
			}

			// If Content-Length is provided, check it first (fast path)
			if r.ContentLength > maxBytes {
				writeBodyLimitError(w)
				return
			}

			// Wrap body reader to enforce limit during read
			// This handles cases where Content-Length is 0 or not set
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeBodyLimitError writes a 413 Payload Too Large error response.
func writeBodyLimitError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "PAYLOAD_TOO_LARGE",
			"message": "request body exceeds maximum allowed size",
		},
	})
}
