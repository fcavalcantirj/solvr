package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const ticketTestSecret = "test-jwt-secret-32-chars-long!!"

func sampleTicketClaims() StreamTicketClaims {
	return StreamTicketClaims{RoomID: "room-1", Kind: "human", Subject: "user-1"}
}

func TestStreamTicket_RoundTripCarriesTheActorAndTheRoom(t *testing.T) {
	now := time.Now()
	claims := sampleTicketClaims()
	claims.Admin = true
	claims.TokenHash = "abc123"

	ticket, expires := IssueStreamTicket(ticketTestSecret, claims, now)
	if !strings.HasPrefix(ticket, StreamTicketPrefix) {
		t.Fatalf("ticket %q lacks the %q prefix", ticket, StreamTicketPrefix)
	}
	if want := now.Add(StreamTicketTTL); !expires.Equal(want.Truncate(time.Second)) {
		t.Errorf("expires = %v, want %v", expires, want.Truncate(time.Second))
	}

	got, err := ParseStreamTicket(ticketTestSecret, ticket, now.Add(StreamTicketTTL-time.Second))
	if err != nil {
		t.Fatalf("a live ticket must parse: %v", err)
	}
	if got.RoomID != "room-1" || got.Kind != "human" || got.Subject != "user-1" || !got.Admin || got.TokenHash != "abc123" {
		t.Errorf("claims did not survive the round trip: %+v", got)
	}
}

func TestStreamTicket_ExpiresAfterTheTTL(t *testing.T) {
	now := time.Now()
	ticket, _ := IssueStreamTicket(ticketTestSecret, sampleTicketClaims(), now)

	if _, err := ParseStreamTicket(ticketTestSecret, ticket, now.Add(StreamTicketTTL+time.Second)); !errors.Is(err, ErrStreamTicketExpired) {
		t.Errorf("an expired ticket must be ErrStreamTicketExpired, got %v", err)
	}
	if StreamTicketTTL > 2*time.Minute {
		t.Errorf("a stream ticket must stay short-lived, TTL is %v", StreamTicketTTL)
	}
}

func TestStreamTicket_RefusesForgeriesAndGarbage(t *testing.T) {
	now := time.Now()
	ticket, _ := IssueStreamTicket(ticketTestSecret, sampleTicketClaims(), now)
	other, _ := IssueStreamTicket(ticketTestSecret, StreamTicketClaims{RoomID: "room-2", Kind: "human", Subject: "user-2"}, now)

	// The payload of one ticket with the signature of another.
	body := strings.TrimPrefix(other, StreamTicketPrefix)
	sig := ticket[strings.LastIndex(ticket, ".")+1:]
	spliced := StreamTicketPrefix + body[:strings.LastIndex(body, ".")] + "." + sig

	cases := map[string]string{
		"empty":               "",
		"no prefix":           strings.TrimPrefix(ticket, StreamTicketPrefix),
		"no signature":        ticket[:strings.LastIndex(ticket, ".")],
		"spliced signature":   spliced,
		"truncated":           ticket[:len(ticket)-4],
		"a JWT-shaped bearer": "eyJhbGciOi.eyJzdWIiOiIxIn0.c2ln",
		"a room token":        "solvr_rt_0123456789abcdef",
	}
	for name, candidate := range cases {
		if _, err := ParseStreamTicket(ticketTestSecret, candidate, now); !errors.Is(err, ErrStreamTicketInvalid) {
			t.Errorf("%s: want ErrStreamTicketInvalid, got %v", name, err)
		}
	}
	if _, err := ParseStreamTicket("another-secret-another-secret-!!", ticket, now); !errors.Is(err, ErrStreamTicketInvalid) {
		t.Errorf("a ticket signed with another secret must be invalid, got %v", err)
	}
}

// A stream ticket must never be accepted where a bearer credential is: it is not a JWT,
// not a room token, not an API key.
func TestStreamTicket_IsNotAValidBearerCredential(t *testing.T) {
	ticket, _ := IssueStreamTicket(ticketTestSecret, sampleTicketClaims(), time.Now())
	if _, err := ValidateJWT(ticketTestSecret, ticket); err == nil {
		t.Error("a stream ticket must not validate as a JWT")
	}
	if strings.HasPrefix(ticket, "solvr_rt_") || strings.HasPrefix(ticket, "solvr_sk_") {
		t.Errorf("a stream ticket must not share the room-token or user-key prefix: %q", ticket)
	}
}
