package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSSEAccessTokenToHeader(t *testing.T) {
	cases := []struct {
		name       string
		header     string // existing Authorization header
		query      string // ?access_token= value
		wantHeader string // expected Authorization seen by next handler
	}{
		{"promotes query when no header", "", "jwt123", "Bearer jwt123"},
		{"leaves existing header untouched", "Bearer real", "other", "Bearer real"},
		{"no query, no header → empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen string
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = r.Header.Get("Authorization")
			})
			url := "/v1/rooms/x/stream"
			if tc.query != "" {
				url += "?access_token=" + tc.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			SSEAccessTokenToHeader(next).ServeHTTP(httptest.NewRecorder(), req)
			if seen != tc.wantHeader {
				t.Errorf("Authorization = %q, want %q", seen, tc.wantHeader)
			}
		})
	}
}
