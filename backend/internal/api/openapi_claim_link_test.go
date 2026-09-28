package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// idx 75 step 5: the OpenAPI contract says where the claim token travels. A claim link
// carries the token after the #, the lookup and the claim take it in a request body, and no
// operation has the token as a path parameter.

func TestOpenAPI_TheClaimTokenTravelsInABodyAndTheLinkFragment(t *testing.T) {
	spec := servedSpec(t)
	paths := at(t, spec, "paths").(map[string]interface{})

	_, retired := paths["/claim/{token}"]
	require.False(t, retired, "the path-token claim route is retired")
	for path := range paths {
		require.False(t, strings.Contains(path, "{token}"), "%s carries a token in its path", path)
	}

	for _, path := range []string{"/agents/claim/lookup", "/agents/claim"} {
		op := operation(t, spec, "post", path)
		body := at(t, op, "requestBody", "content", "application/json", "schema")
		require.Equal(t, "#/components/schemas/ClaimTokenRequest", refName(body), "%s takes the token in a body", path)
		require.Equal(t, []string{"token"}, propertyNames(t, spec, "ClaimTokenRequest"))
		_, hasParams := op["parameters"]
		require.False(t, hasParams, "%s has no path or query parameter", path)
	}
	require.Equal(t, "#/components/schemas/ClaimInfoResponse",
		refName(at(t, operation(t, spec, "post", "/agents/claim/lookup"), "responses", "200", "content", "application/json", "schema")))
	require.ElementsMatch(t, []string{"token_valid", "agent", "expires_at", "error"}, propertyNames(t, spec, "ClaimInfoResponse"),
		"the lookup schema describes what the endpoint answers")

	link := at(t, spec, "x-solvr-conventions", "credential_transport", "claim_link").(map[string]interface{})
	require.Equal(t, "https://solvr.dev/claim#token=<claim token>", link["url"])
	require.Contains(t, link["note"], "fragment")
	require.Contains(t, link["note"], "POST /v1/agents/claim/lookup")
}
