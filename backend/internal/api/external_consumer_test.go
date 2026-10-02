package api

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 6: test an external consumer against a clean new schema plus an upgraded schema, not
// only mocked SDK responses. The client contract tests hold each client to recorded
// examples served by a stub; here the consumer program (contract/consumer: its own module,
// the Go SDK only, HTTP only) works against the real API on two real schemas:
//   - clean: every migration applied to an empty database, and the same with the legacy
//     contribution tables dropped the way schema cleanup will drop them;
//   - upgraded: the production schema (productionSchemaVersion) holding rows the old
//     release wrote, migrated to head and converted by the knowledge cutover.

func consumerEnv(baseURL, run, plannerKey, executorKey string) map[string]string {
	return map[string]string{
		"SOLVR_API_URL":  baseURL,
		"SOLVR_API_KEYS": plannerKey + "," + executorKey,
		"SOLVR_RUN":      run,
	}
}

func TestExternalConsumer_CleanSchema(t *testing.T) {
	for _, tc := range []struct {
		name, run  string
		dropLegacy bool
	}{
		{"every migration", "consumerclean", false},
		{"legacy tables dropped", "consumernolegacy", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbURL := newConsumerScratchSchema(t, len(consumerSchemaFiles(t)), nil)
			ts, pool := serveConsumerSchema(t, dbURL)
			if tc.dropLegacy {
				_, err := pool.Exec(context.Background(), "DROP TABLE "+strings.Join(db.LegacyTables, ", ")+" CASCADE")
				require.NoError(t, err, "drop the legacy tables")
			}
			bin := buildExternalConsumer(t)
			plannerID, plannerKey := registerConsumerAgent(t, ts.URL, "consumer_planner")
			executorID, executorKey := registerConsumerAgent(t, ts.URL, "consumer_executor")
			env := consumerEnv(ts.URL, tc.run, plannerKey, executorKey)

			rep := runExternalConsumer(t, bin, "collaborate", env)
			requireConsumerCollaborated(t, pool, rep, plannerID, executorID, tc.run)
			require.Nil(t, rep.Existing, "no existing content was named")

			// A new post waits for moderation; once approved, search finds it and its replies.
			approveConsumerPost(t, pool, rep.Post.ID)
			env["SOLVR_FIND"] = tc.run
			found := runExternalConsumer(t, bin, "find", env)
			require.Len(t, found.Searches, 1)
			requireConsumerFound(t, found, 0, tc.run, rep.Post.ID, rep.Replies[0].ID, rep.Replies[1].ID)
		})
	}
}

// productionSeed is what the old release wrote, in its own schema.
type productionSeed struct {
	agentID                 string
	question, questionTitle string
	answer, answerBody      string
	problem, approach       string
	roomSlug, roomMessage   string
}

// seedProductionSchema writes rows the way the release on productionSchemaVersion wrote
// them: typed posts, an answer, an approach with a progress note, a room with a message, and
// an agent whose key only has its bcrypt hash (key_sha256 is backfilled on first use). The
// rows are weeks old, as production's are: the create limits count the last hour and halve
// for accounts younger than NewAccountThreshold.
func seedProductionSchema(t *testing.T, ctx context.Context, conn *pgx.Conn, key string) productionSeed {
	t.Helper()
	hash, err := auth.HashAPIKey(key)
	require.NoError(t, err)
	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, conn.QueryRow(ctx, sql, args...).Scan(&v), sql)
		return v
	}
	const old = "now() - interval '40 days'"
	s := productionSeed{
		agentID:       "legacy-planner",
		questionTitle: "How do I drain a pgbouncer pool before a failover?",
		answerBody:    "Run PAUSE, wait until no client is active, then fail over.",
		roomSlug:      "legacy-handoff",
		roomMessage:   "handoff notes written before the upgrade",
	}
	id(`INSERT INTO agents (id, display_name, api_key_hash, created_at, updated_at)
		VALUES ($1, 'Legacy Planner', $2, `+old+`, `+old+`) RETURNING id`, s.agentID, hash)
	s.question = id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, created_at, updated_at)
		VALUES ('question', $1, 'Connections drop while the primary fails over.', 'agent', $2, 'open', `+old+`, `+old+`)
		RETURNING id::text`, s.questionTitle, s.agentID)
	s.answer = id(`INSERT INTO answers (question_id, author_type, author_id, content, created_at)
		VALUES ($1, 'agent', $2, $3, `+old+`) RETURNING id::text`, s.question, s.agentID, s.answerBody)
	s.problem = id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, created_at, updated_at)
		VALUES ('problem', 'Kafka consumer lag spikes after every rebalance', 'Lag jumps to millions after each rebalance.',
		'agent', $1, 'solved', `+old+`, `+old+`) RETURNING id::text`, s.agentID)
	s.approach = id(`INSERT INTO approaches (problem_id, author_type, author_id, angle, method, status, outcome, created_at, updated_at)
		VALUES ($1, 'agent', $2, 'cooperative sticky assignor', 'set partition.assignment.strategy', 'succeeded',
		'lag stays flat', `+old+`, `+old+`) RETURNING id::text`, s.problem, s.agentID)
	id(`INSERT INTO progress_notes (approach_id, content, created_at) VALUES ($1, 'rolled out to half the fleet', `+old+`)
		RETURNING id::text`, s.approach)
	room := id(`INSERT INTO rooms (slug, display_name, token_hash, message_count, created_at, updated_at, last_active_at)
		VALUES ($1, 'Legacy handoff', 'legacy-room-token-hash', 1, `+old+`, `+old+`, `+old+`) RETURNING id::text`, s.roomSlug)
	id(`INSERT INTO messages (room_id, author_type, author_id, agent_name, content, sequence_num, created_at)
		VALUES ($1, 'agent', $2, 'Legacy Planner', $3, 1, `+old+`) RETURNING id::text`, room, s.agentID, s.roomMessage)
	return s
}

func TestExternalConsumer_UpgradedSchema(t *testing.T) {
	ctx := context.Background()
	files := consumerSchemaFiles(t)
	require.Equal(t, "000084_add_group_reply_fields_to_messages.up.sql", filepath.Base(files[productionSchemaVersion-1]),
		"the upgrade starts from the migration production last applied")

	legacyKey := auth.GenerateAPIKey()
	var seed productionSeed
	dbURL := newConsumerScratchSchema(t, productionSchemaVersion, func(ctx context.Context, conn *pgx.Conn) {
		seed = seedProductionSchema(t, ctx, conn, legacyKey)
	})
	ts, pool := serveConsumerSchema(t, dbURL)
	cut, err := db.RunKnowledgeCutover(ctx, pool, db.KnowledgeCutoverOptions{})
	require.NoError(t, err, "the knowledge cutover")
	require.Zero(t, cut.PostExceptions)
	require.EqualValues(t, 2, cut.RepliesCreated, "the answer and the approach become replies")
	require.EqualValues(t, 1, cut.ProgressNotes)
	var approachReply string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM replies WHERE legacy_type = 'approach' AND legacy_id = $1`,
		seed.approach).Scan(&approachReply))

	bin := buildExternalConsumer(t)
	executorID, executorKey := registerConsumerAgent(t, ts.URL, "consumer_executor")
	const run = "consumerupgraded"
	// The planner is the agent the old release registered, with the key it was given then.
	env := consumerEnv(ts.URL, run, legacyKey, executorKey)
	env["SOLVR_EXISTING_POST"] = seed.question
	env["SOLVR_EXISTING_ROOM"] = seed.roomSlug

	rep := runExternalConsumer(t, bin, "collaborate", env)
	requireConsumerCollaborated(t, pool, rep, seed.agentID, executorID, run)
	var backfilled bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT key_sha256 IS NOT NULL FROM agents WHERE id = $1`, seed.agentID).Scan(&backfilled))
	require.True(t, backfilled, "the pre-upgrade key authenticated and got its SHA-256 lookup hash")

	// What the old release wrote is the consumer's to continue.
	ex := rep.Existing
	require.NotNil(t, ex, "the consumer reports the existing post and room")
	require.Equal(t, seed.question, ex.Post.ID)
	require.Equal(t, seed.questionTitle, ex.Post.Title)
	require.Equal(t, "question", ex.Post.Type, "a migrated post keeps its legacy label")
	require.Len(t, ex.Replies, 1, "the legacy answer is the question's one reply")
	migrated := ex.Replies[0]
	require.NotNil(t, migrated.LegacyType)
	require.Equal(t, "answer", *migrated.LegacyType)
	require.NotNil(t, migrated.LegacyID)
	require.Equal(t, seed.answer, *migrated.LegacyID)
	require.Equal(t, seed.agentID, migrated.AuthorID)
	require.Equal(t, seed.answerBody, migrated.Body)
	require.Equal(t, executorID, ex.FollowUp.AuthorID)
	require.NotNil(t, ex.FollowUp.ParentReplyID, "the follow-up is threaded under the migrated answer")
	require.Equal(t, migrated.ID, *ex.FollowUp.ParentReplyID)
	require.Nil(t, ex.FollowUp.LegacyType)
	var parent string
	var legacyType *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT parent_reply_id::text, legacy_type FROM replies WHERE id = $1 AND post_id = $2`,
		ex.FollowUp.ID, seed.question).Scan(&parent, &legacyType))
	require.Equal(t, migrated.ID, parent)
	require.Nil(t, legacyType, "a reply written after the upgrade is native")

	require.NotNil(t, ex.Room, "the consumer reports the existing room")
	require.Equal(t, executorID, ex.Room.AgentID, "the executor joined the pre-upgrade room")
	require.Len(t, ex.Room.Timeline, 1)
	require.Equal(t, seed.agentID, ex.Room.Timeline[0].AuthorID)
	require.Equal(t, seed.roomMessage, ex.Room.Timeline[0].Body)
	require.Equal(t, executorID, ex.Room.Sent.AuthorID)
	requireRoomMessages(t, pool, seed.roomSlug, []consumerEntry{ex.Room.Timeline[0], ex.Room.Sent})

	approveConsumerPost(t, pool, rep.Post.ID)
	env["SOLVR_FIND"] = run + "|pgbouncer failover|cooperative sticky assignor"
	found := runExternalConsumer(t, bin, "find", env)
	require.Len(t, found.Searches, 3)
	requireConsumerFound(t, found, 0, run, rep.Post.ID, rep.Replies[0].ID, rep.Replies[1].ID)
	requireConsumerFound(t, found, 1, "pgbouncer failover", seed.question)
	requireConsumerFound(t, found, 2, "cooperative sticky assignor", seed.problem, approachReply)

	// A cutover run after the consumer wrote finds nothing left to convert and duplicates nothing.
	var repliesBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM replies`).Scan(&repliesBefore))
	again, err := db.RunKnowledgeCutover(ctx, pool, db.KnowledgeCutoverOptions{})
	require.NoError(t, err)
	require.Zero(t, again.PendingContributions)
	require.Zero(t, again.RepliesCreated)
	require.Zero(t, again.ProgressNotes)
	require.Zero(t, again.PostStatesRemapped)
	var repliesAfter int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM replies`).Scan(&repliesAfter))
	require.Equal(t, repliesBefore, repliesAfter)
}

// requireConsumerCollaborated holds the collaborate report to what the API stored.
func requireConsumerCollaborated(t *testing.T, pool *db.Pool, rep consumerReport, plannerID, executorID, run string) {
	t.Helper()
	ctx := context.Background()

	// Room: create, join, send, read, watch.
	room := rep.Room
	require.NotEmpty(t, room.Slug)
	require.Len(t, room.Handshakes, 2)
	for i, agentID := range []string{plannerID, executorID} {
		require.Equal(t, agentID, room.Handshakes[i].AgentID)
		require.Equal(t, room.Slug, room.Handshakes[i].RoomSlug)
		require.Equal(t, "solvr_rt_", room.Handshakes[i].TokenPrefix, "each agent works in the room with its own room token")
	}
	require.Len(t, room.Entries, 2)
	require.Equal(t, plannerID, room.Entries[0].AuthorID)
	require.Contains(t, room.Entries[0].Body, run)
	require.Equal(t, executorID, room.Entries[1].AuthorID)
	require.NotNil(t, room.Entries[1].ReplyToEntryID, "the executor answers the planner's entry")
	require.Equal(t, room.Entries[0].ID, *room.Entries[1].ReplyToEntryID)
	require.Equal(t, room.Entries, room.Timeline, "the timeline, one entry per page, is what was sent")
	require.GreaterOrEqual(t, room.Pages, 2)
	require.Equal(t, executorID, room.Live.AuthorID)
	require.Equal(t, []consumerEntry{room.Entries[1], room.Live}, room.Stream,
		"the stream replays what came after the first entry, then delivers the live one")
	requireRoomMessages(t, pool, room.Slug, []consumerEntry{room.Entries[0], room.Entries[1], room.Live})

	// Post and replies.
	post := rep.Post
	require.NotEmpty(t, post.ID)
	require.Equal(t, "post", post.Type, "a post the consumer creates has no legacy type")
	require.Equal(t, plannerID, post.AuthorID)
	require.Equal(t, "pending", post.ModerationState, "a new post waits for moderation")
	var postType, postedBy string
	require.NoError(t, pool.QueryRow(ctx, `SELECT type, posted_by_id FROM posts WHERE id = $1 AND deleted_at IS NULL`,
		post.ID).Scan(&postType, &postedBy))
	require.Equal(t, "post", postType)
	require.Equal(t, plannerID, postedBy)

	require.Len(t, rep.Replies, 2)
	first, threaded := rep.Replies[0], rep.Replies[1]
	require.Equal(t, executorID, first.AuthorID)
	require.Nil(t, first.ParentReplyID)
	require.Equal(t, plannerID, threaded.AuthorID)
	require.NotNil(t, threaded.ParentReplyID)
	require.Equal(t, first.ID, *threaded.ParentReplyID)
	rows, err := pool.Query(ctx, `SELECT id::text, author_id, coalesce(parent_reply_id::text, ''), legacy_type IS NULL
		FROM replies WHERE post_id = $1 ORDER BY created_at, id`, post.ID)
	require.NoError(t, err)
	var stored []string
	for rows.Next() {
		var id, author, parent string
		var native bool
		require.NoError(t, rows.Scan(&id, &author, &parent, &native))
		require.True(t, native, "reply %s is native", id)
		stored = append(stored, id+" "+author+" "+parent)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{first.ID + " " + executorID + " ", threaded.ID + " " + plannerID + " " + first.ID}, stored)

	require.Len(t, rep.ReplyPages, 2, "two replies read one per page")
	require.Len(t, rep.ReplyPages[0].IDs, 1)
	require.True(t, rep.ReplyPages[0].HasMore)
	require.True(t, rep.ReplyPages[0].NextCursor)
	require.Equal(t, 2, rep.ReplyPages[0].Total)
	require.Len(t, rep.ReplyPages[1].IDs, 1)
	require.False(t, rep.ReplyPages[1].HasMore)
	require.ElementsMatch(t, []string{first.ID, threaded.ID}, append(rep.ReplyPages[0].IDs, rep.ReplyPages[1].IDs...))

	require.True(t, rep.Vote.Voted)
	require.Equal(t, "up", rep.Vote.Direction)
	var direction string
	require.NoError(t, pool.QueryRow(ctx, `SELECT direction FROM votes WHERE target_type = 'reply' AND target_id = $1
		AND voter_id = $2`, threaded.ID, executorID).Scan(&direction))
	require.Equal(t, "up", direction)

	require.Equal(t, 404, rep.MissingPost.Status)
	require.Equal(t, "NOT_FOUND", rep.MissingPost.Code)
}

// requireRoomMessages holds a room's stored message timeline to the entries the consumer saw.
func requireRoomMessages(t *testing.T, pool *db.Pool, slug string, want []consumerEntry) {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT e.id, coalesce(e.author_id, ''), coalesce(e.body, ''), e.reply_to_entry_id
		FROM room_entries e JOIN rooms r ON r.id = e.room_id
		WHERE r.slug = $1 AND e.kind = 'message' AND e.deleted_at IS NULL ORDER BY e.sequence`, slug)
	require.NoError(t, err)
	var got []consumerEntry
	for rows.Next() {
		var e consumerEntry
		require.NoError(t, rows.Scan(&e.ID, &e.AuthorID, &e.Body, &e.ReplyToEntryID))
		got = append(got, e)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, want, got, "room %s stores the messages the consumer saw", slug)
}

// requireConsumerFound holds search i of a find report: it ran the query and found the
// post, with these replies among its matched replies.
func requireConsumerFound(t *testing.T, rep consumerReport, i int, query, postID string, replyIDs ...string) {
	t.Helper()
	s := rep.Searches[i]
	require.Equal(t, query, s.Query)
	for _, r := range s.Results {
		if r.ID == postID {
			require.Subset(t, r.MatchedReplies, replyIDs, "search %q matched the replies of %s", query, postID)
			return
		}
	}
	t.Fatalf("search %q (total %d) did not find post %s: %+v", query, s.Total, postID, s.Results)
}
