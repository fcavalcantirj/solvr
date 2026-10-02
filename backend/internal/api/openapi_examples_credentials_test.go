package api

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78 slice 32 (owner note 2026-10-02): the repo is public, and the examples were captured
// from a running local API, so they kept the room token and the stream ticket that API had just
// issued. No value in the served document (every example of openapi_examples.go) or in
// contract/openapi-examples.json may be credential-shaped unless it is an obvious placeholder
// containing EXAMPLE, with the length and charset of the real credential.

const (
	// exampleRoomTokenPrefix starts a per-agent room token. token keeps its own constant
	// unexported, so TheCredentialCheckCatchesWhatItClaims pins this one to what token mints.
	exampleRoomTokenPrefix = "solvr_rt_"
	// retiredRoomTokenPrefix started the shared room token retired by migration 000098.
	retiredRoomTokenPrefix = "solvr_rm_"
	examplePlaceholder     = "EXAMPLE"
)

var (
	solvrValue = regexp.MustCompile(`solvr_[A-Za-z0-9_-]+`)
	// issuedTicket is a stream ticket as auth.IssueStreamTicket writes it: payload "." signature.
	issuedTicket = regexp.MustCompile(regexp.QuoteMeta(auth.StreamTicketPrefix) + `[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	// exampleKeys hold examples in the served document; everything else there is prose and schema.
	exampleKeys = map[string]bool{"example": true, "examples": true, "x-solvr-example": true, "x-solvr-error-examples": true}
)

// realFormatCredentials returns, masked, each credential-shaped value in s that is no
// placeholder: a room token, user API key or retired shared room token (its prefix and at
// least 16 characters), or an agent API key (auth.APIKeyPrefix and at least the 32 characters a
// Moltbook import mints; GenerateAPIKey mints 43). A stream ticket is no bearer and lives a
// minute, so prose may name one; inside an example, a ticket shaped as one the API issued is a
// captured ticket.
func realFormatCredentials(s string, inExample bool) []string {
	var found []string
	for _, v := range solvrValue.FindAllString(s, -1) {
		if strings.Contains(v, examplePlaceholder) || strings.HasPrefix(v, auth.StreamTicketPrefix) {
			continue
		}
		shown, credential := len(auth.APIKeyPrefix), len(v) >= len(auth.APIKeyPrefix)+32
		for _, prefix := range []string{exampleRoomTokenPrefix, auth.UserAPIKeyPrefix, retiredRoomTokenPrefix} {
			if strings.HasPrefix(v, prefix) {
				shown, credential = len(prefix), len(v) >= len(prefix)+16
			}
		}
		if credential {
			found = append(found, masked(v, shown))
		}
	}
	if inExample {
		for _, v := range issuedTicket.FindAllString(s, -1) {
			if !strings.Contains(v, examplePlaceholder) {
				found = append(found, masked(v, len(auth.StreamTicketPrefix)))
			}
		}
	}
	return found
}

// masked shows a value's prefix and 4 more characters, so a failure never prints a credential.
func masked(v string, prefix int) string {
	return v[:min(len(v), prefix+4)] + "****"
}

// eachString visits every string under node with its JSON path and whether it sits in an example.
func eachString(node interface{}, at string, inExample bool, visit func(at, s string, inExample bool)) {
	switch v := node.(type) {
	case string:
		visit(at, v, inExample)
	case map[string]interface{}:
		for key, child := range v {
			eachString(child, at+"."+key, inExample || exampleKeys[key], visit)
		}
	case []interface{}:
		for i, child := range v {
			eachString(child, fmt.Sprintf("%s[%d]", at, i), inExample, visit)
		}
	}
}

// ticketForExample is the ticket POST /rooms/{slug}/stream-ticket issues to the example's agent
// presenting roomToken: what the example's placeholder must match in length, part by part.
func ticketForExample(roomToken string) string {
	ticket, _ := auth.IssueStreamTicket("secret", auth.StreamTicketClaims{
		RoomID: exampleRoomID, Kind: middleware.RoomCredentialRoomToken, Subject: exampleAgentID,
		TokenHash: token.HashToken(roomToken),
	}, time.Now())
	return ticket
}

func TestOpenAPIExamples_CarryNoRealFormatCredential(t *testing.T) {
	raw, err := os.ReadFile(clientContractPath)
	require.NoError(t, err)
	var fixture interface{}
	require.NoError(t, json.Unmarshal(raw, &fixture))
	roomToken, _, err := token.GenerateAgentRoomToken()
	require.NoError(t, err)
	wantBody, wantSig, _ := strings.Cut(strings.TrimPrefix(ticketForExample(roomToken), auth.StreamTicketPrefix), ".")

	for _, source := range []struct {
		name      string
		doc       interface{}
		inExample bool // the fixture is all examples
	}{
		{"GET /v1/openapi.json", servedSpec(t), false},
		{"contract/openapi-examples.json", fixture, true},
	} {
		var leaks []string
		roomTokens, tickets := 0, 0
		eachString(source.doc, "", source.inExample, func(at, s string, inExample bool) {
			for _, v := range realFormatCredentials(s, inExample) {
				leaks = append(leaks, at+": "+v)
			}
			if !inExample {
				return
			}
			for _, v := range solvrValue.FindAllString(s, -1) {
				if !strings.HasPrefix(v, exampleRoomTokenPrefix) || len(v) < len(exampleRoomTokenPrefix)+16 {
					continue
				}
				roomTokens++
				if strings.Contains(v, examplePlaceholder) {
					assert.Len(t, v, len(roomToken), "%s %s: the placeholder must be as long as a minted room token", source.name, at)
				}
			}
			for _, v := range issuedTicket.FindAllString(s, -1) {
				tickets++
				if strings.Contains(v, examplePlaceholder) {
					body, sig, _ := strings.Cut(strings.TrimPrefix(v, auth.StreamTicketPrefix), ".")
					assert.Equal(t, []int{len(wantBody), len(wantSig)}, []int{len(body), len(sig)},
						"%s %s: the placeholder must be as long as the ticket issued for the example, payload and signature", source.name, at)
				}
			}
		})
		require.Positive(t, roomTokens, "%s shows no room token: this check protects nothing", source.name)
		require.Positive(t, tickets, "%s shows no stream ticket: this check protects nothing", source.name)
		assert.Empty(t, leaks, "%s carries real-format credentials: replace each with a placeholder containing EXAMPLE", source.name)
	}
}

// The check fails on what it claims to catch, and passes placeholders, prose and invalid tickets.
func TestOpenAPIExamples_TheCredentialCheckCatchesWhatItClaims(t *testing.T) {
	roomToken, _, err := token.GenerateAgentRoomToken()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(roomToken, exampleRoomTokenPrefix) && token.IsAgentRoomToken(exampleRoomTokenPrefix+"x"),
		"the room-token prefix this check uses is the one token mints")
	agentKey := auth.GenerateAPIKey()
	key := strings.TrimPrefix(agentKey, auth.APIKeyPrefix)
	ticket := ticketForExample(roomToken)

	for name, leak := range map[string]string{
		"a minted room token": roomToken, "a minted agent API key": agentKey, "a Moltbook agent API key": auth.APIKeyPrefix + key[:32],
		"a user API key": auth.UserAPIKeyPrefix + key, "a retired shared room token": retiredRoomTokenPrefix + key,
		"a room token inside prose": "Authorization: Bearer " + roomToken,
	} {
		assert.Len(t, realFormatCredentials(leak, false), 1, "%s is not caught", name)
	}
	assert.Len(t, realFormatCredentials(ticket, true), 1, "an issued stream ticket inside an example is not caught")

	for name, fine := range map[string]struct {
		s         string
		inExample bool
	}{
		"an issued ticket outside the examples": {ticket, false},
		"the room token placeholder":            {exampleRoomTokenPrefix + examplePlaceholder + strings.Repeat("0", 36), true},
		"a ticket placeholder":                  {auth.StreamTicketPrefix + examplePlaceholder + "0000.EXAMPLE0000", true},
		"an invalid ticket (no signature)":      {"solvr_st_not-a-live-ticket", true},
		"prose naming the prefixes":             {"a solvr_rt_... room token, a solvr_st_... ticket, a solvr_sk_ key, prefix solvr_", true},
		"a short test value":                    {"solvr_rt_x", true},
	} {
		assert.Empty(t, realFormatCredentials(fine.s, fine.inExample), "%s is flagged", name)
	}
}
