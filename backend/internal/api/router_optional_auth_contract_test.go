package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Public read routes permit an omitted Authorization header, but a credential that
// is actually presented must authenticate. Representative routes cover each router
// group that mounts OptionalAuthMiddleware, including the room access policy.
func TestStatusContract_OptionalAuthRejectsInvalidCredentials(t *testing.T) {
	ts, _, _ := newStatusContractServer(t)
	client := &http.Client{}

	paths := []string{
		"/v1/search?q=optional-auth-contract",
		"/v1/posts?per_page=1",
		"/v1/blog?per_page=1",
		// The legacy typed GETs' OptionalAuth group (BART-151); its list, GET /v1/problems, is
		// retired and answers 410 outside every auth middleware (idx 73), so the group is probed
		// through a single-post read.
		"/v1/problems/00000000-0000-0000-0000-000000000000",
		"/v1/rooms/optional-auth-contract-absent-room",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			got, err := callStatusContract(client, http.MethodGet, ts.URL+path, "not-a-valid-token", "")
			require.NoError(t, err)
			require.Equal(t, http.StatusUnauthorized, got.status, got.body)
			require.Equal(t, "INVALID_TOKEN", got.code, got.body)
			require.NotEmpty(t, got.message, got.body)
			require.NotEmpty(t, got.headerID)
			require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
		})
	}
}
