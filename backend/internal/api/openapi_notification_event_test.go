package api

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4: the published Notification schema documents the event contract — its schema
// version, the canonical post/reply subject, and the event types of version 1.
func TestOpenAPI_NotificationDocumentsTheEventContract(t *testing.T) {
	spec := servedSpec(t)
	require.ElementsMatch(t, []string{"id", "user_id", "agent_id", "type", "title", "body", "link", "read_at", "created_at",
		"schema_version", "subject"}, propertyNames(t, spec, "Notification"))

	version := at(t, spec, "components", "schemas", "Notification", "properties", "schema_version").(map[string]interface{})
	require.Equal(t, "integer", version["type"])
	require.Equal(t, []interface{}{float64(0), float64(1), float64(2), float64(3)}, version["enum"])
	desc := version["description"].(string)
	require.True(t, strings.Contains(desc, "0") && strings.Contains(desc, "1") && strings.Contains(desc, "2") && strings.Contains(desc, "3"), desc)

	subject := at(t, spec, "components", "schemas", "Notification", "properties", "subject").(map[string]interface{})
	require.Equal(t, "object", subject["type"])
	props := subject["properties"].(map[string]interface{})
	require.ElementsMatch(t, jsonFields(reflect.TypeOf(models.NotificationSubject{})), mapKeysOf(props),
		"the subject documents exactly the fields the API answers")
	require.ElementsMatch(t, []string{"post_id", "reply_id", "room_id", "entry_id"}, mapKeysOf(props))

	eventType := at(t, spec, "components", "schemas", "Notification", "properties", "type").(map[string]interface{})
	for _, name := range []string{"post.approved", "post.rejected", "reply.removed", "reply.flagged", "blog_post_rejected"} {
		require.Contains(t, eventType["description"], name, "schema 1 event types are documented")
	}
	for _, name := range []string{"room.reply", "room.review_requested", "subject.entry_id"} {
		require.Contains(t, eventType["description"], name, "schema 3 event types are documented (idx 92)")
	}
}

// Schema version 2 adds the room events and subject.room_id (SPEC.md Part 5.6): the type,
// version and subject documentation name them.
func TestOpenAPI_NotificationDocumentsTheRoomEventsOfVersionTwo(t *testing.T) {
	spec := servedSpec(t)
	props := func(path ...string) map[string]interface{} {
		return at(t, spec, append([]string{"components", "schemas", "Notification", "properties"}, path...)...).(map[string]interface{})
	}
	typeDesc := props("type")["description"].(string)
	for _, name := range []string{models.NotificationRoomMemberAdded, models.NotificationRoomMemberRemoved, "schema_version 2", "subject.room_id"} {
		require.Contains(t, typeDesc, name)
	}
	require.Contains(t, props("schema_version")["description"], "2: ")
	require.Contains(t, props("schema_version")["description"], "room")
	room := props("subject", "properties", "room_id")
	require.Equal(t, "string", room["type"])
	require.Equal(t, "uuid", room["format"])
	require.Contains(t, props("subject")["description"], "room")
}
