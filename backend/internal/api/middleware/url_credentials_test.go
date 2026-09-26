package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// idx 75 step 5: a URL is copied, logged by every proxy in front of the API, kept in
// browser history and pasted into screenshots, so a credential that outlives the page
// must not ride in one. The room routes read credentials from the Authorization header
// only; a client that puts one in the query string is told so instead of being silently
// downgraded to anonymous.

func TestRefuseURLCredentials(t *testing.T) {
	const secret = "solvr_rt_0123456789abcdef"

	cases := []struct {
		name       string
		header     string
		query      string
		wantStatus int
	}{
		{"?token= is refused", "", "token=" + secret, http.StatusBadRequest},
		{"?access_token= is refused", "", "access_token=eyJhbGciOi.payload.sig", http.StatusBadRequest},
		{"refused even beside a header", "Bearer real", "token=" + secret, http.StatusBadRequest},
		{"an empty value is still a credential in the URL", "", "token=", http.StatusBadRequest},
		{"no query passes", "", "", http.StatusOK},
		{"a paging query passes", "", "limit=5&cursor=abc", http.StatusOK},
		{"a stream ticket is not a bearer and passes", "", "ticket=solvr_st_abc", http.StatusOK},
		{"a header alone passes", "Bearer real", "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			h := RefuseURLCredentials(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
			}))
			req := httptest.NewRequest(http.MethodGet, "/v1/rooms/x/entries?"+tc.query, nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if reached != (tc.wantStatus == http.StatusOK) {
				t.Errorf("next handler reached = %v for status %d", reached, rec.Code)
			}
			if tc.wantStatus == http.StatusOK {
				return
			}
			body := rec.Body.String()
			if !strings.Contains(body, `"VALIDATION_ERROR"`) {
				t.Errorf("body %s lacks VALIDATION_ERROR", body)
			}
			if !strings.Contains(body, "Authorization") {
				t.Errorf("the refusal must name the header to use: %s", body)
			}
			for _, leak := range []string{secret, "eyJhbGciOi"} {
				if strings.Contains(body, leak) {
					t.Errorf("the refusal echoed a credential: %s", body)
				}
			}
		})
	}
}

// The guards read the header and nothing else: even without RefuseURLCredentials in front
// of them, a credential in the URL authenticates nobody.
func TestRoomBearerToken_NeverReadsTheURL(t *testing.T) {
	const tok = "solvr_rt_0123456789abcdef"

	inURL := httptest.NewRequest(http.MethodGet, "/v1/rooms/x/entries?token="+tok+"&access_token="+tok, nil)
	if got := roomBearerToken(inURL); got != "" {
		t.Errorf("roomBearerToken read the URL: %q", got)
	}

	inHeader := httptest.NewRequest(http.MethodGet, "/v1/rooms/x/entries", nil)
	inHeader.Header.Set("Authorization", "Bearer "+tok)
	if got := roomBearerToken(inHeader); got != tok {
		t.Errorf("roomBearerToken = %q, want the header credential", got)
	}
}
