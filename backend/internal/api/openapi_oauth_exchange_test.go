package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The OpenAPI document says what the code exchange answers (SPEC.md 5.2): the token, and the
// two facts about the sign-in that issued the code.
func TestOpenAPI_DocumentsTheOAuthCodeExchangeAnswer(t *testing.T) {
	spec := servedSpec(t)
	op := operation(t, spec, "post", "/auth/oauth/exchange")
	text, _ := op["description"].(string)
	require.NotEmpty(t, text, "POST /auth/oauth/exchange has no description")
	for _, want := range []string{
		"login_code", "access_token", "token_type", "expires_in",
		"is_new_user", "true only when", "created the account",
		"provider", "github", "google",
		"INVALID_LOGIN_CODE", "60 seconds",
	} {
		require.Contains(t, text, want, "the exchange description must say %q", want)
	}
}
