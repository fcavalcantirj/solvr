package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// idx 75 step 5, HTTP logging. Query parameters were already redacted; a secret that
// rides in the PATH (the claim link GET /v1/claim/{token}) was written to the access log
// verbatim, so anyone who can read the log could claim the agent.

func servedLogLine(t *testing.T, target string) string {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	r := chi.NewRouter()
	r.Use(Logging)
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	r.Get("/v1/claim/{token}", ok)
	r.Get("/v1/posts/{id}", ok)
	r.Get("/v1/rooms/{slug}/entries", ok)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	return buf.String()
}

func TestLogging_RedactsASecretRoutedInThePath(t *testing.T) {
	const secret = "claimSECRETvalue0123456789"
	line := servedLogLine(t, "/v1/claim/"+secret)

	if strings.Contains(line, secret) {
		t.Errorf("the claim token reached the access log: %s", line)
	}
	if !strings.Contains(line, `/v1/claim/***REDACTED***`) {
		t.Errorf("the log line must keep the route and mark the secret: %s", line)
	}
}

func TestLogging_KeepsOrdinaryPathValuesReadable(t *testing.T) {
	for target, want := range map[string]string{
		"/v1/posts/8f14e45f-ceea-467f-a0e6-1c2d3e4f5a6b": "/v1/posts/8f14e45f-ceea-467f-a0e6-1c2d3e4f5a6b",
		"/v1/rooms/launch-room/entries?limit=5":          "/v1/rooms/launch-room/entries?limit=5",
	} {
		if line := servedLogLine(t, target); !strings.Contains(line, want) {
			t.Errorf("%s: an ordinary id or slug must stay in the log: %s", target, line)
		}
	}
}
