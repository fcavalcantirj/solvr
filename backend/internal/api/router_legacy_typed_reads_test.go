package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task idx 73 step 3: the legacy typed reads are retired. A single-post read served the post GET
// /v1/posts/{id} serves; the contribution reads served the approaches, progress notes, approach
// relationships, answers and responses the cutover turned into replies (MigrateContributions,
// RemapLegacyRelations). Following each route's instructions on GET /v1/posts/{id} and GET
// /v1/posts/{id}/replies must find what the route served, with the fields the instructions name.
// The legacy tables are archived (000138): the fixture seeds the replies as the cutover left them.

type typedReplyRow struct {
	migratedReplyRow
	Upvotes   int       `json:"upvotes"`
	Downvotes int       `json:"downvotes"`
	Score     int       `json:"score"`
	UpdatedAt time.Time `json:"updated_at"`
}

type typedReplyPage struct {
	Data []typedReplyRow `json:"data"`
	Meta struct {
		Total      int    `json:"total"`
		HasMore    bool   `json:"has_more"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
}

// legacyContribution is one legacy row as the retired route served it, with the id of the
// reply the cutover made from it.
type legacyContribution struct {
	id, body, author, replyID string
	// votesUp and votesDown are the confirmed votes cast on its reply.
	votesUp, votesDown int
	createdAt          time.Time
	provenance         map[string]any
}

func TestRetiredTypedReads_CanonicalReadsServeWhatTheRouteServed(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	ctx := context.Background()
	client := &http.Client{}

	agentID, _ := statusContractAgent(t, ts, pool)
	userID, _ := createLiveTestUser(t, pool, models.UserRoleUser)
	var agentName string
	require.NoError(t, pool.QueryRow(ctx, "SELECT display_name FROM agents WHERE id = $1", agentID).Scan(&agentName))

	marker := uuid.NewString()
	post := func(label string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility)
			 VALUES ('post', $1, $2, 'agent', $3, 'open', 'public') RETURNING id::text`,
			"retired typed reads "+label+" "+marker, "retired typed reads fixture "+marker, agentID).Scan(&id))
		return id
	}
	problem, question, idea := post("problem"), post("question"), post("idea")
	posts := []string{problem, question, idea}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, "DELETE FROM votes WHERE voter_type = 'human' AND voter_id = $1", userID)     //nolint:errcheck
		pool.Exec(c, "DELETE FROM replies WHERE post_id = ANY($1::uuid[])", posts)                 //nolint:errcheck
		pool.Exec(c, "DELETE FROM reputation_history WHERE owner_id IN ($1, $2)", agentID, userID) //nolint:errcheck
		pool.Exec(c, "DELETE FROM posts WHERE id = ANY($1::uuid[])", posts)                        //nolint:errcheck
	})

	base := time.Now().UTC().Add(-5 * time.Hour).Truncate(time.Microsecond)
	at := func(minutes int) time.Time { return base.Add(time.Duration(minutes) * time.Minute) }

	// migrated inserts the reply the cutover made from a legacy row whose id is legacyID, under
	// parent when it is set (nil for a top-level reply), soft-deleted at created when deleted.
	migrated := func(postID string, parent any, author, authorID, body, legacyType, legacyID string,
		provenance map[string]any, created time.Time, deleted bool) string {
		var deletedAt *time.Time
		if deleted {
			deletedAt = &created
		}
		raw, err := json.Marshal(provenance)
		require.NoError(t, err)
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body,
				legacy_type, legacy_id, provenance, created_at, updated_at, deleted_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::uuid, $8::jsonb, $9, $9, $10) RETURNING id::text`,
			postID, parent, author, authorID, body, legacyType, legacyID, string(raw), created, deletedAt).Scan(&id))
		return id
	}

	// The problem: an older failed approach, a newer one that updates it with a progress note, and
	// a deleted one the list never showed. The newer approach's relationship to the older one is
	// in its reply's provenance, as RemapLegacyRelations records it.
	approach := func(angle, method, status, outcome string, assumptions []string, differsFrom []string,
		created time.Time, deleted bool, relationships ...map[string]any) legacyContribution {
		if differsFrom == nil {
			differsFrom = []string{}
		}
		if assumptions == nil {
			assumptions = []string{}
		}
		var outcomeValue any // NULL when the approach had no outcome
		if outcome != "" {
			outcomeValue = outcome
		}
		prov := map[string]any{"legacy_table": "approaches", "angle": angle, "method": method,
			"assumptions": assumptions, "differs_from": differsFrom, "status": status,
			"outcome": outcomeValue, "solution": nil, "is_latest": true, "archived_cid": nil}
		if len(relationships) > 0 {
			prov["approach_relationships"] = relationships
		}
		row := legacyContribution{id: uuid.NewString(), author: "agent", createdAt: created}
		row.replyID = migrated(problem, nil, "agent", agentID,
			"**Approach:** "+angle+"\n\n**Method:** "+method+"\n\n**Status:** "+status,
			"approach", row.id, prov, created, deleted)
		row.provenance = map[string]any{"angle": angle, "method": method, "status": status}
		return row
	}
	older := approach("older angle "+marker, "older method", "failed", "it did not hold", []string{"cache is cold"}, nil, at(0), false)
	relationshipID := uuid.NewString()
	newer := approach("newer angle "+marker, "newer method", "working", "", nil, []string{older.id}, at(10), false,
		map[string]any{"legacy_id": relationshipID, "relation_type": "updates", "to_reply_id": older.replyID,
			"to_approach_id": older.id, "created_at": at(11).Format(time.RFC3339Nano)})
	approach("deleted angle "+marker, "deleted method", "starting", "", nil, nil, at(20), true)
	noteID, noteContent := uuid.NewString(), "progress note "+marker
	migrated(problem, newer.replyID, "agent", agentID, noteContent, "progress_note", noteID,
		map[string]any{"legacy_table": "progress_notes", "approach_id": newer.id}, at(12), false)

	// The question: an accepted answer by the human, a later one by the agent with votes, and a
	// deleted one.
	answer := func(author, authorID string, accepted bool, created time.Time, deleted bool) legacyContribution {
		row := legacyContribution{id: uuid.NewString(), author: author, createdAt: created,
			body: author + " answer " + uuid.NewString(), provenance: map[string]any{"is_accepted": accepted}}
		row.replyID = migrated(question, nil, author, authorID, row.body, "answer", row.id,
			map[string]any{"legacy_table": "answers", "is_accepted": accepted}, created, deleted)
		return row
	}
	accepted := answer("human", userID, true, at(30), false)
	voted := answer("agent", agentID, false, at(31), false)
	answer("agent", agentID, false, at(32), true)

	// The idea: two responses of different types (responses had no soft delete).
	response := func(author, authorID, responseType string, created time.Time) legacyContribution {
		row := legacyContribution{id: uuid.NewString(), author: author, createdAt: created,
			body: author + " response " + uuid.NewString(), provenance: map[string]any{"response_type": responseType}}
		row.replyID = migrated(idea, nil, author, authorID, row.body, "response", row.id,
			map[string]any{"legacy_table": "responses", "response_type": responseType}, created, false)
		return row
	}
	build := response("agent", agentID, "build", at(40))
	critique := response("human", userID, "critique", at(41))

	// The confirmed votes on the replies, where the cutover moved the legacy rows' votes: the
	// human up-votes the voted answer and down-votes the build response, and casts an unconfirmed
	// vote on the critique, which counts for nothing.
	vote := func(replyID, direction string, confirmed bool) {
		_, err := pool.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ('reply', $1::uuid, 'human', $2, $3, $4)`, replyID, userID, direction, confirmed)
		require.NoError(t, err)
	}
	vote(voted.replyID, "up", true)
	voted.votesUp = 1
	vote(build.replyID, "down", true)
	build.votesDown = 1
	vote(critique.replyID, "up", false)

	retired := func(t *testing.T, path string) (replacement, instructions string) {
		t.Helper()
		got, err := callStatusContract(client, "GET", ts.URL+path, "", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusGone, got.status, got.body)
		require.Equal(t, ErrCodeEndpointRetired, got.code, got.body)
		require.NotContains(t, got.body, marker, "the migration error reads nothing")
		var envelope struct {
			Error struct {
				Details struct {
					Replacement  string `json:"replacement"`
					Instructions string `json:"instructions"`
				} `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(got.body), &envelope), got.body)
		return envelope.Error.Details.Replacement, envelope.Error.Details.Instructions
	}
	readPost := func(t *testing.T, id string) models.PostWithAuthor {
		t.Helper()
		got, err := callStatusContract(client, "GET", ts.URL+"/v1/posts/"+id, "", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, got.status, got.body)
		var resp struct {
			Data models.PostWithAuthor `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(got.body), &resp), got.body)
		return resp.Data
	}
	readReplies := func(t *testing.T, postID, query string) typedReplyPage {
		t.Helper()
		got, err := callStatusContract(client, "GET", ts.URL+"/v1/posts/"+postID+"/replies"+query, "", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, got.status, got.body)
		var page typedReplyPage
		require.NoError(t, json.Unmarshal([]byte(got.body), &page), got.body)
		require.False(t, page.Meta.HasMore, "one page holds the fixture post's replies")
		require.Equal(t, len(page.Data), page.Meta.Total, "meta.total counts every reply of the post")
		return page
	}
	// ofType follows "the replies whose legacy_type is <type>", oldest first.
	ofType := func(page typedReplyPage, legacyType string) []typedReplyRow {
		var found []typedReplyRow
		for _, reply := range page.Data {
			if reply.LegacyType != nil && *reply.LegacyType == legacyType {
				found = append(found, reply)
			}
		}
		return found
	}
	requireMigrated := func(t *testing.T, want legacyContribution, got typedReplyRow, postID string) {
		t.Helper()
		require.Equal(t, want.id, *got.LegacyID, "the legacy id is legacy_id, oldest first")
		require.NotEqual(t, want.id, got.ID, "the reply has its own id")
		require.Equal(t, postID, got.PostID, "the legacy parent id is post_id")
		require.Equal(t, want.author, got.AuthorType)
		require.Equal(t, want.author, got.Author.Type)
		require.Equal(t, got.AuthorID, got.Author.ID)
		require.True(t, want.createdAt.Equal(got.CreatedAt), "created_at is unchanged: %v vs %v", want.createdAt, got.CreatedAt)
		for key, value := range want.provenance {
			require.Equal(t, value, got.Provenance[key], "provenance.%s", key)
		}
		if want.body != "" {
			require.Equal(t, want.body, got.Body, "content is body")
			require.Equal(t, want.votesUp, got.Upvotes, "upvotes count the confirmed votes moved to the reply")
			require.Equal(t, want.votesDown, got.Downvotes, "downvotes count the confirmed votes moved to the reply")
			require.Equal(t, want.votesUp-want.votesDown, got.Score, "vote_score is score")
		}
	}

	t.Run("single posts read the same post", func(t *testing.T) {
		for path, want := range map[string]struct{ id, typ string }{
			"/v1/problems/" + problem:   {problem, "problem"},
			"/v1/questions/" + question: {question, "question"},
			"/v1/ideas/" + idea:         {idea, "idea"},
		} {
			replacement, instructions := retired(t, path)
			require.Equal(t, "GET /v1/posts/{id}", replacement, path)
			require.Contains(t, instructions, "every post is type post since the legacy types were retired", path)
			p := readPost(t, want.id)
			require.Equal(t, want.id, p.ID, path)
			require.Equal(t, models.PostTypePost, p.Type, "%s: data.type is post, the only type (000138)", path)
			require.Equal(t, "retired typed reads "+want.typ+" "+marker, p.Title, path)
			require.Equal(t, agentID, p.Author.ID, path)
		}
		_, instructions := retired(t, "/v1/questions/"+question)
		require.Contains(t, instructions, "accepted_answer_id was retired: the accepted answer is the reply whose provenance.is_accepted is true")
		acceptedReply := ofType(readReplies(t, question, "?limit=100"), "answer")[0]
		require.Equal(t, accepted.id, *acceptedReply.LegacyID)
		require.Equal(t, true, acceptedReply.Provenance["is_accepted"],
			"the accepted answer's reply carries is_accepted in its provenance")
	})

	t.Run("answers", func(t *testing.T) {
		for _, path := range []string{"/v1/questions/" + question + "/answers", "/v1/questions/" + question} {
			replacement, instructions := retired(t, path)
			require.Contains(t, []string{"GET /v1/posts/{id}/replies", "GET /v1/posts/{id}"}, replacement)
			require.Contains(t, instructions, `Each answer is a reply whose legacy_type is "answer"`, path)
		}
		found := ofType(readReplies(t, question, "?limit=100"), "answer")
		require.Len(t, found, 2, "the live answers, without the deleted one")
		requireMigrated(t, accepted, found[0], question)
		requireMigrated(t, voted, found[1], question)
		require.Equal(t, "Live Test User", found[0].Author.DisplayName)
		require.Equal(t, agentName, found[1].Author.DisplayName)
	})

	t.Run("responses", func(t *testing.T) {
		for _, path := range []string{"/v1/ideas/" + idea + "/responses", "/v1/ideas/" + idea} {
			_, instructions := retired(t, path)
			require.Contains(t, instructions, `Each response is a reply whose legacy_type is "response"`, path)
		}
		found := ofType(readReplies(t, idea, "?limit=100"), "response")
		require.Len(t, found, 2)
		requireMigrated(t, build, found[0], idea)
		requireMigrated(t, critique, found[1], idea)
	})

	t.Run("approaches and their progress notes", func(t *testing.T) {
		replacement, instructions := retired(t, "/v1/problems/"+problem+"/approaches")
		require.Equal(t, "GET /v1/posts/{id}/replies", replacement)
		require.Contains(t, instructions, `Each approach is a reply whose legacy_type is "approach"`)
		require.Contains(t, instructions, `legacy_type is "progress_note"`)
		page := readReplies(t, problem, "?limit=100")
		found := ofType(page, "approach")
		require.Len(t, found, 2, "the live approaches, without the deleted one")
		requireMigrated(t, older, found[0], problem)
		requireMigrated(t, newer, found[1], problem)
		require.Equal(t, []any{"cache is cold"}, found[0].Provenance["assumptions"])
		require.Equal(t, "it did not hold", found[0].Provenance["outcome"])
		require.Equal(t, []any{older.id}, found[1].Provenance["differs_from"])
		require.Contains(t, found[0].Body, "older angle "+marker, "body renders the angle")
		require.Contains(t, found[0].Body, "failed", "body renders the status")
		require.True(t, older.createdAt.Equal(found[0].UpdatedAt), "updated_at is unchanged")

		notes := ofType(page, "progress_note")
		require.Len(t, notes, 1)
		require.NotNil(t, notes[0].ParentReplyID)
		require.Equal(t, found[1].ID, *notes[0].ParentReplyID, "a progress note is a child of its approach's reply")
		require.Equal(t, noteID, *notes[0].LegacyID)
		require.Equal(t, noteContent, notes[0].Body)
		require.Equal(t, newer.id, notes[0].Provenance["approach_id"])
		require.True(t, at(12).Equal(notes[0].CreatedAt))
	})

	t.Run("approach history", func(t *testing.T) {
		replacement, instructions := retired(t, "/v1/problems/"+problem+"/approaches/"+newer.id+"/history")
		require.Equal(t, "GET /v1/posts/{id}/replies", replacement)
		require.Contains(t, instructions, "provenance.approach_relationships")
		found := ofType(readReplies(t, problem, "?limit=100"), "approach")
		var current, previous typedReplyRow
		for _, reply := range found {
			switch *reply.LegacyID {
			case newer.id:
				current = reply
			case older.id:
				previous = reply
			}
		}
		require.NotEmpty(t, current.ID, "current is the reply migrated from the approach")
		rels, ok := current.Provenance["approach_relationships"].([]any)
		require.True(t, ok, "relationships are in the from-approach reply's provenance: %v", current.Provenance)
		require.Len(t, rels, 1)
		rel := rels[0].(map[string]any)
		require.Equal(t, relationshipID, rel["legacy_id"])
		require.Equal(t, "updates", rel["relation_type"])
		require.Equal(t, older.id, rel["to_approach_id"])
		require.Equal(t, previous.ID, rel["to_reply_id"], "history follows to_reply_id")
		created, err := time.Parse(time.RFC3339Nano, rel["created_at"].(string))
		require.NoError(t, err)
		require.True(t, at(11).Equal(created))
		_, more := previous.Provenance["approach_relationships"]
		require.False(t, more, "the walk ends at a reply with no relationship")
	})

	t.Run("export", func(t *testing.T) {
		replacement, instructions := retired(t, "/v1/problems/"+problem+"/export")
		require.Equal(t, "GET /v1/posts/{id}", replacement)
		require.True(t, strings.HasPrefix(instructions, "There is no canonical export"))
		require.Equal(t, problem, readPost(t, problem).ID)
		page := readReplies(t, problem, "?limit=100")
		require.Len(t, ofType(page, "approach"), 2)
		require.Len(t, ofType(page, "progress_note"), 1)
	})

	t.Run("page and per_page became limit and cursor", func(t *testing.T) {
		got, err := callStatusContract(client, "GET", ts.URL+"/v1/posts/"+problem+"/replies?limit=1", "", "")
		require.NoError(t, err)
		var first typedReplyPage
		require.NoError(t, json.Unmarshal([]byte(got.body), &first), got.body)
		require.Len(t, first.Data, 1)
		require.True(t, first.Meta.HasMore)
		require.NotEmpty(t, first.Meta.NextCursor)
		got, err = callStatusContract(client, "GET", ts.URL+"/v1/posts/"+problem+"/replies?limit=1&cursor="+first.Meta.NextCursor, "", "")
		require.NoError(t, err)
		var second typedReplyPage
		require.NoError(t, json.Unmarshal([]byte(got.body), &second), got.body)
		require.Len(t, second.Data, 1)
		require.NotEqual(t, first.Data[0].ID, second.Data[0].ID, "the cursor continues where the page ended")
	})
}
