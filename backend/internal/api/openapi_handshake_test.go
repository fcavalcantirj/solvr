package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// idx 75 step 1: the served contract defines what a handshake does when an agent already
// holds a token (it adds a session), how rotation is asked for and what the other sessions
// then see, so a client library can implement recovery without reading prose elsewhere.

func TestOpenAPIOperations_HandshakePublishesSessionsAndRotation(t *testing.T) {
	spec := servedSpec(t)
	op := operation(t, spec, "post", "/rooms/{slug}/handshake")

	assert.Equal(t, "handshakeRoom", op["operationId"])
	assert.NotEmpty(t, op["summary"])
	desc, _ := op["description"].(string)
	for _, want := range []string{"rotate", "CREDENTIAL_ROTATED", "TOKEN_LIMIT_REACHED", "session"} {
		assert.Contains(t, desc, want, "the operation explains %q", want)
	}
	require.NotEmpty(t, op["security"], "a handshake proves identity with the agent API key")
	param(t, spec, op, "slug")

	body := deref(t, spec, at(t, op, "requestBody", "content", "application/json", "schema")).(map[string]interface{})
	props := at(t, body, "properties").(map[string]interface{})
	assert.Equal(t, "boolean", at(t, props, "rotate", "type"), "rotate is a boolean")
	assert.Equal(t, "integer", at(t, props, "ttl_seconds", "type"))
	assert.Contains(t, at(t, props, "rotate", "description"), "CREDENTIAL_ROTATED")

	created := deref(t, spec, at(t, op, "responses", "201")).(map[string]interface{})
	schema := deref(t, spec, at(t, created, "content", "application/json", "schema")).(map[string]interface{})
	data := deref(t, spec, at(t, schema, "properties", "data")).(map[string]interface{})
	for _, field := range []string{"agent_id", "room_slug", "room_token", "rotated", "a2a_base"} {
		assert.Contains(t, at(t, data, "properties").(map[string]interface{}), field, "the handshake answer documents %s", field)
	}

	responses := at(t, op, "responses").(map[string]interface{})
	for _, code := range []string{"400", "401", "403", "404", "409"} {
		assert.NotEmpty(t, responses[code], "handshake names its %s row", code)
	}
	assert.Equal(t, "#/components/responses/Conflict", refName(responses["409"]))

	unauthorized := at(t, spec, "components", "responses", "Unauthorized", "description").(string)
	assert.Contains(t, unauthorized, "CREDENTIAL_ROTATED", "the shared 401 row names the recoverable code")
	conflict := at(t, spec, "components", "responses", "Conflict", "description").(string)
	assert.Contains(t, conflict, "TOKEN_LIMIT_REACHED", "the shared 409 row names the limit code")
}

func TestOpenAPIConventions_RoomTokenSessionsAndTheRotationStreamEvent(t *testing.T) {
	spec := servedSpec(t)
	sessions := at(t, spec, "x-solvr-conventions", "room_token_sessions").(map[string]interface{})

	assert.EqualValues(t, db.MaxLiveRoomAgentTokens, sessions["max_live_tokens_per_agent_per_room"])
	assert.Equal(t, "rotate", sessions["rotate_field"])
	assert.EqualValues(t, http.StatusUnauthorized, at(t, sessions, "replaced_token", "status"))
	assert.Equal(t, "CREDENTIAL_ROTATED", at(t, sessions, "replaced_token", "code"))
	assert.EqualValues(t, http.StatusConflict, at(t, sessions, "limit_reached", "status"))
	assert.Equal(t, "TOKEN_LIMIT_REACHED", at(t, sessions, "limit_reached", "code"))
	assert.Contains(t, at(t, sessions, "note"), "handshake again")

	streams := at(t, spec, "x-solvr-conventions", "streams").(map[string]interface{})
	assert.Equal(t, "credential_rotated", streams["credential_rotated_event"])
	assert.Contains(t, streams["credential_rotated_note"], "handshake")
	assert.Equal(t, "access_revoked", streams["access_revoked_event"], "the access_revoked contract is unchanged")
}
