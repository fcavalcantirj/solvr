package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 7: participants and roles are collections, and each first-party integration adds and
// works with a third and subsequent agent without creating another room. The external
// consumer's members phase does it through the Go SDK alone (ListRoomMembers,
// AddRoomMember) against the real API: a closed room refuses an agent nobody admitted, the
// owner admits a third and a fourth agent to the same room, all four join with their own room
// tokens, the owner addresses its directive to the participants it listed, and each answers.

// consumerMembers is the consumer's members report.
type consumerMembers struct {
	Slug       string              `json:"slug"`
	IsPrivate  bool                `json:"is_private"`
	Refused    consumerError       `json:"refused"`
	Admitted   []consumerMember    `json:"admitted"`
	Members    []consumerMember    `json:"members"`
	NotOwner   consumerError       `json:"not_owner"`
	Handshakes []consumerHandshake `json:"handshakes"`
	Directive  consumerEntry       `json:"directive"`
	Addressed  []string            `json:"addressed"`
	Entries    []consumerEntry     `json:"entries"`
	Timeline   []consumerEntry     `json:"timeline"`
}

type consumerMember struct {
	RoomID  string `json:"room_id"`
	AgentID string `json:"agent_id"`
	Role    string `json:"role"`
	AddedBy string `json:"added_by"`
}

type consumerError struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
}

func TestExternalConsumer_AThirdAndFourthAgentAreAdmittedToTheSameClosedRoomThroughTheSDK(t *testing.T) {
	dbURL := newConsumerScratchSchema(t, len(consumerSchemaFiles(t)), nil)
	ts, pool := serveConsumerSchema(t, dbURL)
	bin := buildExternalConsumer(t)
	var ids, keys []string
	for _, name := range []string{"members_planner", "members_executor", "members_reviewer", "members_tester"} {
		id, key := registerConsumerAgent(t, ts.URL, name)
		ids, keys = append(ids, id), append(keys, key)
	}
	planner, later := ids[0], ids[1:]
	const run = "consumermembers"

	rep := runExternalConsumer(t, bin, "members", map[string]string{
		"SOLVR_API_URL":   ts.URL,
		"SOLVR_API_KEYS":  strings.Join(keys, ","),
		"SOLVR_AGENT_IDS": strings.Join(ids, ","),
		"SOLVR_RUN":       run,
	})
	m := rep.Members
	require.NotNil(t, m, "the members phase reported no room")
	ctx := context.Background()

	// One closed room, the only one of the run.
	var roomID string
	var private bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text, is_private FROM rooms WHERE slug = $1`, m.Slug).Scan(&roomID, &private))
	require.True(t, private, "the room is stored closed")
	require.True(t, m.IsPrivate, "the SDK surfaced the closed room")
	var rooms int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM rooms`).Scan(&rooms))
	require.Equal(t, 1, rooms, "admitting more agents created no other room")

	// Nobody admitted the agent yet: the closed room refused it.
	require.Equal(t, consumerError{Status: 403, Code: "FORBIDDEN"}, m.Refused)

	// The owner admitted each later agent, in turn, to that room.
	require.Len(t, m.Admitted, len(later))
	for i, id := range later {
		require.Equal(t, consumerMember{RoomID: roomID, AgentID: id, Role: "member", AddedBy: planner}, m.Admitted[i])
	}

	// The participants the SDK listed are the stored allowlist: the owner and every later agent.
	stored, err := db.NewRoomMemberRepository(pool).ListByRoom(ctx, uuid.MustParse(roomID))
	require.NoError(t, err)
	var want []consumerMember
	for _, s := range stored {
		want = append(want, consumerMember{RoomID: s.RoomID.String(), AgentID: s.AgentID, Role: s.Role, AddedBy: s.AddedBy})
	}
	require.Equal(t, want, m.Members)
	require.Len(t, m.Members, len(ids))
	require.Equal(t, []string{planner, "owner"}, []string{m.Members[0].AgentID, m.Members[0].Role}, "the creator owns the room")

	// Each agent joined with its own identity and its own room token.
	require.Len(t, m.Handshakes, len(ids))
	for i, hs := range m.Handshakes {
		require.Equal(t, consumerHandshake{AgentID: ids[i], RoomSlug: m.Slug, TokenPrefix: "solvr_rt_"}, hs)
	}
	var tokens int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(DISTINCT agent_id) FROM room_agent_tokens WHERE room_id = $1::uuid`, roomID).Scan(&tokens))
	require.Equal(t, len(ids), tokens, "one room token per agent")

	// The directive went to the participants the owner listed; each later agent answered it.
	require.Equal(t, planner, m.Directive.AuthorID)
	require.Equal(t, later, m.Addressed)
	var addressedRaw []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT addressed_member_ids FROM room_entries WHERE id = $1`, m.Directive.ID).Scan(&addressedRaw))
	var addressed []string
	require.NoError(t, json.Unmarshal(addressedRaw, &addressed))
	require.Equal(t, later, addressed, "the stored recipients are the listed participants")
	require.Len(t, m.Entries, len(later))
	for i, e := range m.Entries {
		require.Equal(t, later[i], e.AuthorID)
		require.NotNil(t, e.ReplyToEntryID)
		require.Equal(t, m.Directive.ID, *e.ReplyToEntryID)
		require.Contains(t, e.Body, run)
	}

	// The last agent admitted reads the whole exchange, and it is what the room stores.
	timeline := append([]consumerEntry{m.Directive}, m.Entries...)
	require.Equal(t, timeline, m.Timeline)
	requireRoomMessages(t, pool, m.Slug, timeline)

	// Only the owner reads the participants; the SDK surfaced the API's refusal.
	require.Equal(t, consumerError{Status: 403, Code: "FORBIDDEN"}, m.NotOwner)
}
