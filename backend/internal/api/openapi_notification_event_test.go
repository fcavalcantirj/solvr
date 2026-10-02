package api

import (
	"strings"
	"testing"

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
	require.Equal(t, []interface{}{float64(0), float64(1)}, version["enum"])
	desc := version["description"].(string)
	require.True(t, strings.Contains(desc, "0") && strings.Contains(desc, "1"), desc)

	subject := at(t, spec, "components", "schemas", "Notification", "properties", "subject").(map[string]interface{})
	require.Equal(t, "object", subject["type"])
	props := subject["properties"].(map[string]interface{})
	require.Len(t, props, 2)
	require.Contains(t, props, "post_id")
	require.Contains(t, props, "reply_id")

	eventType := at(t, spec, "components", "schemas", "Notification", "properties", "type").(map[string]interface{})
	for _, name := range []string{"post.approved", "post.rejected", "reply.removed", "reply.flagged", "blog_post_rejected"} {
		require.Contains(t, eventType["description"], name, "schema 1 event types are documented")
	}
}
