package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/go-chi/chi/v5"
)

const streamTicketTestSecret = "test-jwt-secret-32-chars-long!!"

// streamTicketRoute serves the stream route behind SSEStreamTicket and reports what the
// next handler saw.
func streamTicketRoute(seen **auth.StreamTicketClaims, authorization *string) http.Handler {
	r := chi.NewRouter()
	r.With(SSEStreamTicket(streamTicketTestSecret)).Get("/v1/rooms/{slug}/stream", func(w http.ResponseWriter, r *http.Request) {
		*seen = StreamTicketFromContext(r.Context())
		*authorization = r.Header.Get("Authorization")
	})
	return r
}

func TestSSEStreamTicket(t *testing.T) {
	claims := auth.StreamTicketClaims{RoomID: "room-1", Kind: RoomCredentialHuman, Subject: "user-1"}
	live, _ := auth.IssueStreamTicket(streamTicketTestSecret, claims, time.Now())
	expired, _ := auth.IssueStreamTicket(streamTicketTestSecret, claims, time.Now().Add(-2*auth.StreamTicketTTL))

	cases := []struct {
		name       string
		header     string // Authorization header sent with the request
		query      string
		wantStatus int
		wantCode   string
		wantClaims bool
	}{
		{"a live ticket is verified and left for the room guard", "", "ticket=" + live, http.StatusOK, "", true},
		{"a header wins: the ticket is not read", "Bearer real", "ticket=" + live, http.StatusOK, "", false},
		{"no query, no header: anonymous, untouched", "", "", http.StatusOK, "", false},
		{"an expired ticket is a recoverable 401", "", "ticket=" + expired, http.StatusUnauthorized, CodeStreamTicketExpired, false},
		{"a garbage ticket is a recoverable 401", "", "ticket=solvr_st_nonsense", http.StatusUnauthorized, CodeStreamTicketInvalid, false},
		{"?access_token= is retired", "", "access_token=eyJsecret.value.here", http.StatusBadRequest, "VALIDATION_ERROR", false},
		{"?token= is retired", "", "token=solvr_rt_secretvalue", http.StatusBadRequest, "VALIDATION_ERROR", false},
		{"retired even beside a header", "Bearer real", "access_token=eyJsecret.value.here", http.StatusBadRequest, "VALIDATION_ERROR", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen *auth.StreamTicketClaims
			var authorization string
			req := httptest.NewRequest(http.MethodGet, "/v1/rooms/x/stream?"+tc.query, nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			streamTicketRoute(&seen, &authorization).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode != "" && !strings.Contains(rec.Body.String(), `"`+tc.wantCode+`"`) {
				t.Errorf("body %s lacks code %s", rec.Body.String(), tc.wantCode)
			}
			if tc.wantStatus != http.StatusOK {
				if !strings.Contains(rec.Body.String(), "/v1/rooms/x/stream-ticket") {
					t.Errorf("the refusal must say where to get a ticket: %s", rec.Body.String())
				}
				for _, secret := range []string{"eyJsecret", "solvr_rt_secretvalue", "solvr_st_nonsense", expired} {
					if strings.Contains(rec.Body.String(), secret) {
						t.Errorf("the refusal echoed a credential: %s", rec.Body.String())
					}
				}
				return
			}
			if (seen != nil) != tc.wantClaims {
				t.Errorf("claims in context = %v, want present=%v", seen, tc.wantClaims)
			}
			if tc.wantClaims && (seen.RoomID != "room-1" || seen.Subject != "user-1") {
				t.Errorf("wrong claims: %+v", seen)
			}
			if authorization != tc.header {
				t.Errorf("Authorization = %q, want it untouched (%q): a ticket is never promoted to a bearer", authorization, tc.header)
			}
		})
	}
}
