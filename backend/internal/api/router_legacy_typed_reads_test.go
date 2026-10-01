package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task idx 73 step 3: the legacy typed reads are retired. A single-post read served the post GET
// /v1/posts/{id} serves; the contribution reads served the approaches, progress notes, approach
// relationships, answers and responses the cutover turns into replies (MigrateContributions,
// RemapLegacyRelations). Following each route's instructions on GET /v1/posts/{id} and GET
// /v1/posts/{id}/replies must find what the route served, with the fields the instructions name.

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

// legacyContribution is one legacy row as the retired route served it.
type legacyContribution struct {
	id, body, author string
	// votesUp and votesDown are the confirmed votes cast on the row; its own counters are seeded
	// to other values, which the cutover's recount must not keep.
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
	post := func(typ string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility)
			 VALUES ($1, $2, $3, 'agent', $4, 'open', 'public') RETURNING id::text`,
			typ, "retired typed reads "+typ+" "+marker, "retired typed reads fixture "+marker, agentID).Scan(&id))
		return id
	}
	problem, question, idea := post("problem"), post("question"), post("idea")
	posts := []string{problem, question, idea}
	var approaches, answers, responses []string
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, "DELETE FROM votes WHERE voter_type = 'human' AND voter_id = $1", userID)                  //nolint:errcheck
		pool.Exec(c, "DELETE FROM replies WHERE post_id = ANY($1::uuid[])", posts)                              //nolint:errcheck
		pool.Exec(c, "DELETE FROM approach_relationships WHERE from_approach_id = ANY($1::uuid[])", approaches) //nolint:errcheck
		pool.Exec(c, "DELETE FROM progress_notes WHERE approach_id = ANY($1::uuid[])", approaches)              //nolint:errcheck
		pool.Exec(c, "DELETE FROM approaches WHERE id = ANY($1::uuid[])", approaches)                           //nolint:errcheck
		pool.Exec(c, "DELETE FROM answers WHERE id = ANY($1::uuid[])", answers)                                 //nolint:errcheck
		pool.Exec(c, "DELETE FROM responses WHERE id = ANY($1::uuid[])", responses)                             //nolint:errcheck
		pool.Exec(c, "DELETE FROM reputation_history WHERE owner_id IN ($1, $2)", agentID, userID)              //nolint:errcheck
		pool.Exec(c, "UPDATE posts SET accepted_answer_id = NULL WHERE id = $1::uuid", question)                //nolint:errcheck
		pool.Exec(c, "DELETE FROM posts WHERE id = ANY($1::uuid[])", posts)                                     //nolint:errcheck
	})

	base := time.Now().UTC().Add(-5 * time.Hour).Truncate(time.Microsecond)
	at := func(minutes int) time.Time { return base.Add(time.Duration(minutes) * time.Minute) }

	// The problem: an older failed approach, a newer one that updates it with a progress note, and
	// a deleted one the list never showed.
	approach := func(angle, method, status, outcome string, assumptions []string, differsFrom []string,
		created time.Time, deleted bool) legacyContribution {
		var deletedAt *time.Time
		if deleted {
			deletedAt = &created
		}
		if differsFrom == nil {
			differsFrom = []string{}
		}
		if assumptions == nil {
			assumptions = []string{}
		}
		row := legacyContribution{author: "agent", createdAt: created}
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO approaches (problem_id, author_type, author_id, angle, method,
				assumptions, differs_from, status, outcome, created_at, updated_at, deleted_at)
			VALUES ($1::uuid, 'agent', $2, $3, $4, $5, $6::uuid[], $7, NULLIF($8, ''), $9, $9, $10) RETURNING id::text`,
			problem, agentID, angle, method, assumptions, differsFrom, status, outcome, created, deletedAt).Scan(&row.id))
		approaches = append(approaches, row.id)
		row.provenance = map[string]any{"angle": angle, "method": method, "status": status}
		return row
	}
	older := approach("older angle "+marker, "older method", "failed", "it did not hold", []string{"cache is cold"}, nil, at(0), false)
	newer := approach("newer angle "+marker, "newer method", "working", "", nil, []string{older.id}, at(10), false)
	approach("deleted angle "+marker, "deleted method", "starting", "", nil, nil, at(20), true)
	var relationshipID, noteID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO approach_relationships (from_approach_id, to_approach_id, relation_type, created_at)
		VALUES ($1::uuid, $2::uuid, 'updates', $3) RETURNING id::text`, newer.id, older.id, at(11)).Scan(&relationshipID))
	noteContent := "progress note " + marker
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO progress_notes (approach_id, content, created_at)
		VALUES ($1::uuid, $2, $3) RETURNING id::text`, newer.id, noteContent, at(12)).Scan(&noteID))

	// The question: an accepted answer by the human, a later one by the agent with votes, and a
	// deleted one.
	answer := func(author, authorID string, accepted bool, up, down int, created time.Time, deleted bool) legacyContribution {
		var deletedAt *time.Time
		if deleted {
			deletedAt = &created
		}
		row := legacyContribution{author: author, createdAt: created,
			body: author + " answer " + uuid.NewString(), provenance: map[string]any{"is_accepted": accepted}}
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO answers (question_id, author_type, author_id, content, is_accepted,
				upvotes, downvotes, created_at, deleted_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id::text`,
			question, author, authorID, row.body, accepted, up, down, created, deletedAt).Scan(&row.id))
		answers = append(answers, row.id)
		return row
	}
	accepted := answer("human", userID, true, 0, 0, at(30), false)
	voted := answer("agent", agentID, false, 3, 1, at(31), false)
	answer("agent", agentID, false, 0, 0, at(32), true)
	_, err := pool.Exec(ctx, "UPDATE posts SET accepted_answer_id = $1::uuid WHERE id = $2::uuid", accepted.id, question)
	require.NoError(t, err)

	// The idea: two responses of different types (responses have no soft delete).
	response := func(author, authorID, responseType string, up, down int, created time.Time) legacyContribution {
		row := legacyContribution{author: author, createdAt: created,
			body: author + " response " + uuid.NewString(), provenance: map[string]any{"response_type": responseType}}
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO responses (idea_id, author_type, author_id, content, response_type,
				upvotes, downvotes, created_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8) RETURNING id::text`,
			idea, author, authorID, row.body, responseType, up, down, created).Scan(&row.id))
		responses = append(responses, row.id)
		return row
	}
	build := response("agent", agentID, "build", 2, 0, at(40))
	critique := response("human", userID, "critique", 0, 1, at(41))

	// The confirmed votes recorded on them, which the cutover moves to the replies and recounts:
	// the human up-votes the voted answer and down-votes the build response, and casts an
	// unconfirmed vote on the critique, which counts for nothing.
	vote := func(targetType, targetID, direction string, confirmed bool) {
		_, err := pool.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2::uuid, 'human', $3, $4, $5)`, targetType, targetID, userID, direction, confirmed)
		require.NoError(t, err)
	}
	vote("answer", voted.id, "up", true)
	voted.votesUp = 1
	vote("response", build.id, "down", true)
	build.votesDown = 1
	vote("response", critique.id, "up", false)

	_, err = db.MigrateContributions(ctx, pool)
	require.NoError(t, err, "the cutover's contribution migration")
	_, err = db.RemapLegacyRelations(ctx, pool)
	require.NoError(t, err, "the cutover's relation remap")
	// The cutover's vote_scores step (rebuild_vote_scores, 000113), scoped to the fixture replies.
	_, err = pool.Exec(ctx, `SELECT rebuild_vote_scores('reply', id) FROM replies WHERE post_id = ANY($1::uuid[])`, posts)
	require.NoError(t, err, "the cutover's vote recount")

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
			require.Contains(t, instructions, "check data.type", path)
			p := readPost(t, want.id)
			require.Equal(t, want.id, p.ID, path)
			require.Equal(t, models.PostType(want.typ), p.Type, "%s: data.type names the legacy type", path)
			require.Equal(t, "retired typed reads "+want.typ+" "+marker, p.Title, path)
			require.Equal(t, agentID, p.Author.ID, path)
		}
		_, instructions := retired(t, "/v1/questions/"+question)
		require.Contains(t, instructions, "accepted_answer_id names the reply migrated from the accepted answer")
		acceptedReply := ofType(readReplies(t, question, "?limit=100"), "answer")[0]
		require.Equal(t, accepted.id, *acceptedReply.LegacyID)
		p := readPost(t, question)
		require.NotNil(t, p.AcceptedAnswerID)
		require.Equal(t, acceptedReply.ID, *p.AcceptedAnswerID, "accepted_answer_id names the accepted answer's reply")
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
