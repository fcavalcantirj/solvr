package middleware

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// ErrorEnvelope makes every error response carry the public error envelope:
// {"error": {"code", "message", "request_id", ...}}. It must run AFTER the
// request-ID middleware (it reads X-Request-ID off the response headers) and
// OUTSIDE chi's Recoverer so a panic's bare 500 also gets an envelope.
//
// Only 4xx/5xx responses are buffered and rewritten; success responses and
// streams pass straight through, so SSE keeps flushing in real time.
//
//   - A JSON error object gains request_id (and retry_after_seconds on 429,
//     taken from Retry-After) unless the handler already set them. Every
//     other byte of the body is preserved as the handler wrote it.
//   - An empty or plain-text error body becomes an envelope with a code
//     derived from the status and the text (or status text) as message.
//   - HTML bodies and JSON whose "error" is not an object are left untouched.
func ErrorEnvelope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ew := &errorEnvelopeWriter{ResponseWriter: w, head: r.Method == http.MethodHead}
		next.ServeHTTP(ew, r)
		ew.finish()
	})
}

type errorEnvelopeWriter struct {
	http.ResponseWriter
	head        bool
	status      int
	wroteHeader bool
	buffering   bool
	buf         bytes.Buffer
}

func (ew *errorEnvelopeWriter) WriteHeader(status int) {
	if ew.wroteHeader {
		return
	}
	ew.wroteHeader = true
	ew.status = status
	if status >= 400 && !ew.head {
		ew.buffering = true
		return
	}
	ew.ResponseWriter.WriteHeader(status)
}

func (ew *errorEnvelopeWriter) Write(b []byte) (int, error) {
	if !ew.wroteHeader {
		ew.WriteHeader(http.StatusOK)
	}
	if ew.buffering {
		return ew.buf.Write(b)
	}
	return ew.ResponseWriter.Write(b)
}

// Flush keeps streaming handlers working. A buffered error is flushed as a
// whole in finish, never piecemeal.
func (ew *errorEnvelopeWriter) Flush() {
	if ew.buffering {
		return
	}
	if f, ok := ew.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (ew *errorEnvelopeWriter) Unwrap() http.ResponseWriter {
	return ew.ResponseWriter
}

func (ew *errorEnvelopeWriter) finish() {
	if !ew.buffering {
		return
	}
	h := ew.Header()
	body, rewritten := envelopeErrorBody(ew.status, h, ew.buf.Bytes())
	if rewritten {
		h.Set("Content-Type", "application/json")
		h.Del("Content-Length")
	}
	ew.ResponseWriter.WriteHeader(ew.status)
	ew.ResponseWriter.Write(body)
}

// envelopeErrorBody returns the body to send and whether it differs from raw.
func envelopeErrorBody(status int, h http.Header, raw []byte) ([]byte, bool) {
	if ct := h.Get("Content-Type"); ct != "" {
		mediaType, _, _ := mime.ParseMediaType(ct)
		if mediaType != "application/json" && mediaType != "text/plain" {
			return raw, false
		}
	}
	requestID := h.Get("X-Request-ID")

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err == nil && top != nil {
		errRaw, ok := top["error"]
		if !ok {
			return raw, false
		}
		var errObj map[string]json.RawMessage
		if err := json.Unmarshal(errRaw, &errObj); err != nil || errObj == nil {
			return raw, false
		}
		changed := false
		if _, has := errObj["request_id"]; !has && requestID != "" {
			errObj["request_id"], _ = json.Marshal(requestID)
			changed = true
		}
		if secs, ok := retryAfterSeconds(status, h); ok {
			if _, has := errObj["retry_after_seconds"]; !has {
				errObj["retry_after_seconds"], _ = json.Marshal(secs)
				changed = true
			}
		}
		if !changed {
			return raw, false
		}
		top["error"], _ = json.Marshal(errObj)
		out, err := json.Marshal(top)
		if err != nil {
			return raw, false
		}
		return append(out, '\n'), true
	}

	// Empty or non-JSON body: synthesize the envelope.
	message := strings.TrimSpace(string(raw))
	if message == "" {
		message = http.StatusText(status)
	}
	errObj := map[string]interface{}{
		"code":    errorCodeForStatus(status),
		"message": message,
	}
	if requestID != "" {
		errObj["request_id"] = requestID
	}
	if secs, ok := retryAfterSeconds(status, h); ok {
		errObj["retry_after_seconds"] = secs
	}
	out, err := json.Marshal(map[string]interface{}{"error": errObj})
	if err != nil {
		return raw, false
	}
	return append(out, '\n'), true
}

func retryAfterSeconds(status int, h http.Header) (int, bool) {
	if status != http.StatusTooManyRequests && status != http.StatusServiceUnavailable {
		return 0, false
	}
	secs, err := strconv.Atoi(strings.TrimSpace(h.Get("Retry-After")))
	if err != nil || secs < 0 {
		return 0, false
	}
	return secs, true
}

// errorCodeForStatus maps a status to the stable codes documented in
// SPEC.md Part 5.4, falling back to the upper-snake status text.
func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusRequestEntityTooLarge:
		return "PAYLOAD_TOO_LARGE"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	}
	text := http.StatusText(status)
	if text == "" {
		if status >= 500 {
			return "INTERNAL_ERROR"
		}
		return "BAD_REQUEST"
	}
	return strings.ToUpper(strings.NewReplacer(" ", "_", "-", "_", "'", "").Replace(text))
}
