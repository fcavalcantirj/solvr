package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// SPEC.md 27.1 (Sitemaps): a sitemap read whose database read fails answers a retryable 503
// with Retry-After, never a 500. The web's sitemaps turn it into their own 503, so a crawler
// that asks in that minute comes back instead of being told the site has no pages.
func TestSitemapHandler_DatabaseFailureIsRetryable(t *testing.T) {
	failing := &MockSitemapRepository{
		Err:          context.DeadlineExceeded,
		CountsErr:    context.DeadlineExceeded,
		PaginatedErr: context.DeadlineExceeded,
	}
	h := NewSitemapHandler(failing)
	reads := []struct {
		name  string
		path  string
		serve http.HandlerFunc
	}{
		{"flat list", "/v1/sitemap/urls", h.GetSitemapURLs},
		{"one type", "/v1/sitemap/urls?type=users&per_page=5000", h.GetSitemapURLs},
		{"counts", "/v1/sitemap/counts", h.GetSitemapCounts},
	}
	for _, read := range reads {
		t.Run(read.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			read.serve(rec, httptest.NewRequest(http.MethodGet, read.path, nil))

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "120" {
				t.Errorf("Retry-After = %q, want \"120\"", got)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			var body struct {
				Error struct{ Code, Message string } `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			if body.Error.Code != "SERVICE_UNAVAILABLE" {
				t.Errorf("error.code = %q, want SERVICE_UNAVAILABLE", body.Error.Code)
			}
		})
	}
}

// A refusal of the caller's own request stays a 400 and carries no Retry-After: asking again
// would not help.
func TestSitemapHandler_BadRequestIsNotRetryable(t *testing.T) {
	rec := httptest.NewRecorder()
	NewSitemapHandler(&MockSitemapRepository{}).GetSitemapURLs(rec, httptest.NewRequest(http.MethodGet, "/v1/sitemap/urls?type=nope", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want none", got)
	}
}
