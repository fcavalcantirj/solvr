package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// StreamTicketPrefix starts every stream ticket. It is deliberately none of the bearer
// prefixes (solvr_rt_ room tokens, solvr_sk_ user keys) and not a JWT, so a ticket can
// never be replayed as a credential.
const StreamTicketPrefix = "solvr_st_"

// StreamTicketTTL is how long a ticket opens a stream. A browser EventSource cannot send
// an Authorization header, so it opens the stream with a ticket in the URL; keeping the
// life this short makes a ticket that leaks into a log, a copied link or a screenshot
// useless almost at once.
const StreamTicketTTL = 60 * time.Second

// streamTicketDomain separates the ticket signing key from the JWT signing key derived
// from the same server secret.
const streamTicketDomain = "solvr/stream-ticket/v1"

var (
	// ErrStreamTicketInvalid: malformed, forged, or signed with another secret.
	ErrStreamTicketInvalid = errors.New("invalid stream ticket")
	// ErrStreamTicketExpired: authentic, but past its expiry. Recoverable: ask for a new one.
	ErrStreamTicketExpired = errors.New("expired stream ticket")
)

// StreamTicketClaims names who a ticket stands for and the one room it may open. It
// carries no secret: a room token is represented by the hash the server already stores.
type StreamTicketClaims struct {
	RoomID    string `json:"r"`
	Kind      string `json:"k"`           // human | agent_api_key | room_token
	Subject   string `json:"s"`           // user id or agent id
	Admin     bool   `json:"a,omitempty"` // a human admin
	TokenHash string `json:"h,omitempty"` // room_token only: the stored hash, for revocation rechecks
	ExpiresAt int64  `json:"e"`           // unix seconds
}

// IssueStreamTicket signs claims into a ticket valid for StreamTicketTTL from now and
// returns it with its expiry. It is stateless, so every API instance accepts it.
func IssueStreamTicket(secret string, claims StreamTicketClaims, now time.Time) (string, time.Time) {
	expires := now.Add(StreamTicketTTL).Truncate(time.Second)
	claims.ExpiresAt = expires.Unix()
	payload, _ := json.Marshal(claims) // plain struct of strings, bools and an int: cannot fail
	body := base64.RawURLEncoding.EncodeToString(payload)
	return StreamTicketPrefix + body + "." + streamTicketSignature(secret, body), expires
}

// ParseStreamTicket verifies the signature and expiry of a ticket.
func ParseStreamTicket(secret, ticket string, now time.Time) (*StreamTicketClaims, error) {
	rest, ok := strings.CutPrefix(ticket, StreamTicketPrefix)
	if !ok {
		return nil, ErrStreamTicketInvalid
	}
	body, sig, ok := strings.Cut(rest, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(streamTicketSignature(secret, body))) {
		return nil, ErrStreamTicketInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, ErrStreamTicketInvalid
	}
	var claims StreamTicketClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.RoomID == "" || claims.Subject == "" {
		return nil, ErrStreamTicketInvalid
	}
	if now.Unix() >= claims.ExpiresAt {
		return nil, ErrStreamTicketExpired
	}
	return &claims, nil
}

func streamTicketSignature(secret, body string) string {
	key := hmac.New(sha256.New, []byte(secret))
	key.Write([]byte(streamTicketDomain))
	mac := hmac.New(sha256.New, key.Sum(nil))
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
