package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// The human-only auth routes turn an agent API key away with 403 in the public error
// envelope — code, message and the request id of the response — like every other error.
func TestStatusContract_AgentKeyOnHumanAuthRoutesUsesTheErrorEnvelope(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, agentKey := statusContractAgent(t, ts, pool)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	routes := []struct{ method, path, body string }{
		{"POST", "/v1/auth/register", `{}`},
		{"POST", "/v1/auth/login", `{}`},
		{"GET", "/v1/auth/github", ""},
		{"GET", "/v1/auth/github/callback", ""},
		{"GET", "/v1/auth/google", ""},
		{"GET", "/v1/auth/google/callback", ""},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			got, err := callStatusContract(client, rt.method, ts.URL+rt.path, agentKey, rt.body)
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, got.status, got.body)
			require.Equal(t, "FORBIDDEN", got.code, got.body)
			require.Contains(t, got.message, "POST /v1/agents/register", got.body)
			require.NotEmpty(t, got.headerID)
			require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
		})
	}
}
