package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task idx 73 step 3: the legacy comment lists are retired. Their comments are the replies the
// cutover made (MigrateContributions), so following each route's instructions on GET
// /v1/posts/{id}/replies must find exactly the comments the route listed, with the fields the
// instructions name. The legacy tables are archived (000138): the fixture seeds the replies as
// the migration shaped them.

// legacyCommentRow is one comment as the retired list served it.
type legacyCommentRow struct {
	id, targetType, targetID, authorType, authorID, displayName, content string
	createdAt                                                            time.Time
}

type migratedReplyRow struct {
	ID            string         `json:"id"`
	PostID        string         `json:"post_id"`
	ParentReplyID *string        `json:"parent_reply_id"`
	AuthorType    string         `json:"author_type"`
	AuthorID      string         `json:"author_id"`
	Body          string         `json:"body"`
	LegacyType    *string        `json:"legacy_type"`
	LegacyID      *string        `json:"legacy_id"`
	Provenance    map[string]any `json:"provenance"`
	CreatedAt     time.Time      `json:"created_at"`
	Author        struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		DisplayName string `json:"display_name"`
	} `json:"author"`
}

type replyListPage struct {
	Data []migratedReplyRow `json:"data"`
	Meta struct {
		Total      int    `json:"total"`
		HasMore    bool   `json:"has_more"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
}

func TestRetiredCommentLists_RepliesListWhatTheRouteListed(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	ctx := context.Background()
	client := &http.Client{}

	agentID, _ := statusContractAgent(t, ts, pool)
	userID, _ := createLiveTestUser(t, pool, models.UserRoleUser)
	var agentName string
	require.NoError(t, pool.QueryRow(ctx, "SELECT display_name FROM agents WHERE id = $1", agentID).Scan(&agentName))
	names := map[string]string{"agent": agentName, "human": "Live Test User"}
	authors := map[string]string{"agent": agentID, "human": userID}

	marker := uuid.NewString()
	post := func(label string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility)
			 VALUES ('post', $1, $2, 'agent', $3, 'open', 'public') RETURNING id::text`,
			"retired comment lists "+label+" "+marker, "retired comment lists fixture "+marker, agentID).Scan(&id))
		return id
	}
	problem, question, idea := post("problem"), post("question"), post("idea")
	t.Cleanup(func() { // the replies go with their posts (ON DELETE CASCADE)
		pool.Exec(context.Background(), "DELETE FROM posts WHERE id = ANY($1::uuid[])", []string{problem, question, idea}) //nolint:errcheck
	})
	// The replies the migration made from an approach, an answer and a response; each route's
	// {id} is the contribution's legacy id.
	contribution := func(postID, legacyType, body, provenance string) (replyID, legacyID string) {
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
			VALUES ($1::uuid, 'agent', $2, $3, $4, gen_random_uuid(), $5::jsonb) RETURNING id::text, legacy_id::text`,
			postID, agentID, body, legacyType, provenance).Scan(&replyID, &legacyID))
		return replyID, legacyID
	}
	approachReply, approach := contribution(problem, "approach", "**Angle:** fixture angle",
		`{"legacy_table":"approaches","angle":"fixture angle","method":"fixture method","status":"starting"}`)
	answerReply, answer := contribution(question, "answer", "fixture answer", `{"legacy_table":"answers","is_accepted":false}`)
	responseReply, response := contribution(idea, "response", "fixture response", `{"legacy_table":"responses","response_type":"support"}`)

	// Two live comments on each target (the human's written first) and a deleted one, which the
	// retired list never showed, each as the migration made it: a reply with the comment's id as
	// legacy_id, top-level on a post and a child of the target contribution's reply otherwise.
	base := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Microsecond)
	listed := map[string][]legacyCommentRow{}
	comment := func(targetType, targetID, postID string, parent any, author string, at time.Time, deleted bool) legacyCommentRow {
		row := legacyCommentRow{id: uuid.NewString(), targetType: targetType, targetID: targetID, authorType: author,
			authorID: authors[author], displayName: names[author], createdAt: at,
			content: author + " comment on " + targetType + " " + uuid.NewString()}
		var deletedAt *time.Time
		if deleted {
			deletedAt = &at
		}
		_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body,
				legacy_type, legacy_id, provenance, created_at, updated_at, deleted_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, 'comment', $6::uuid,
				jsonb_build_object('legacy_table', 'comments', 'target_type', $7::text, 'target_id', $8::text), $9, $9, $10)`,
			postID, parent, author, row.authorID, row.content, row.id, targetType, targetID, at, deletedAt)
		require.NoError(t, err)
		return row
	}
	routes := []struct {
		path, targetType, targetID, postID string
		parent                             any // the reply migrated from the target contribution; nil for a post
	}{
		{"/v1/posts/" + question + "/comments", "post", question, question, nil},
		{"/v1/approaches/" + approach + "/comments", "approach", approach, problem, approachReply},
		{"/v1/answers/" + answer + "/comments", "answer", answer, question, answerReply},
		{"/v1/responses/" + response + "/comments", "response", response, idea, responseReply},
	}
	for i, r := range routes {
		at := base.Add(time.Duration(i) * time.Minute)
		listed[r.targetID] = []legacyCommentRow{
			comment(r.targetType, r.targetID, r.postID, r.parent, "human", at, false),
			comment(r.targetType, r.targetID, r.postID, r.parent, "agent", at.Add(10*time.Second), false),
		}
		comment(r.targetType, r.targetID, r.postID, r.parent, "agent", at.Add(20*time.Second), true)
	}

	replies := func(postID, query string) replyListPage {
		t.Helper()
		got, err := callStatusContract(client, "GET", ts.URL+"/v1/posts/"+postID+"/replies"+query, "", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, got.status, got.body)
		var page replyListPage
		require.NoError(t, json.Unmarshal([]byte(got.body), &page), got.body)
		return page
	}

	for _, r := range routes {
		t.Run(r.targetType, func(t *testing.T) {
			got, err := callStatusContract(client, "GET", ts.URL+r.path, "", "")
			require.NoError(t, err)
			require.Equal(t, http.StatusGone, got.status, got.body)
			require.Equal(t, ErrCodeEndpointRetired, got.code, got.body)
			var envelope struct {
				Error struct {
					Details struct {
						Replacement  string `json:"replacement"`
						Instructions string `json:"instructions"`
					} `json:"details"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal([]byte(got.body), &envelope), got.body)
			require.Equal(t, "GET /v1/posts/{id}/replies", envelope.Error.Details.Replacement)
			instructions := envelope.Error.Details.Instructions

			page := replies(r.postID, "?limit=100")
			require.False(t, page.Meta.HasMore, "one page holds the fixture post's replies")
			require.Equal(t, len(page.Data), page.Meta.Total, "meta.total counts every reply of the post")

			// Follow the instructions: a post's comments are its top-level comment replies; a
			// contribution's are the children of the reply migrated from it.
			var parent *string
			if r.targetType != "post" {
				require.Contains(t, instructions, `legacy_type is "`+r.targetType+`" and legacy_id is {id}`)
				for i := range page.Data {
					reply := page.Data[i]
					if reply.LegacyType != nil && *reply.LegacyType == r.targetType && *reply.LegacyID == r.targetID {
						parent = &page.Data[i].ID
					}
				}
				require.NotNil(t, parent, "the reply migrated from the %s", r.targetType)
			}
			var found []migratedReplyRow
			for _, reply := range page.Data {
				sameParent := (parent == nil && reply.ParentReplyID == nil) ||
					(parent != nil && reply.ParentReplyID != nil && *reply.ParentReplyID == *parent)
				if sameParent && reply.LegacyType != nil && *reply.LegacyType == "comment" {
					found = append(found, reply)
				}
			}
			want := listed[r.targetID]
			require.Len(t, found, len(want), "exactly the live comments the route listed: %s", instructions)
			require.Greater(t, page.Meta.Total, len(found), "the post holds more replies than these comments")
			for i, c := range want {
				reply := found[i]
				require.Equal(t, c.id, *reply.LegacyID, "the comment's id is legacy_id, oldest first")
				require.NotEqual(t, c.id, reply.ID, "the reply has its own id")
				require.Equal(t, c.content, reply.Body, "content is body")
				require.Equal(t, r.postID, reply.PostID)
				require.Equal(t, c.authorType, reply.AuthorType)
				require.Equal(t, c.authorID, reply.AuthorID)
				require.Equal(t, c.authorID, reply.Author.ID)
				require.Equal(t, c.authorType, reply.Author.Type)
				require.Equal(t, c.displayName, reply.Author.DisplayName)
				require.True(t, c.createdAt.Equal(reply.CreatedAt), "created_at is unchanged: %v vs %v", c.createdAt, reply.CreatedAt)
				require.Equal(t, c.targetType, reply.Provenance["target_type"], "provenance keeps target_type")
				require.Equal(t, c.targetID, reply.Provenance["target_id"], "provenance keeps target_id")
			}

			// page and per_page became limit and cursor.
			first := replies(r.postID, "?limit=1")
			require.Len(t, first.Data, 1)
			require.True(t, first.Meta.HasMore)
			require.NotEmpty(t, first.Meta.NextCursor)
			second := replies(r.postID, "?limit=1&cursor="+first.Meta.NextCursor)
			require.Len(t, second.Data, 1)
			require.Equal(t, page.Data[1].ID, second.Data[0].ID, "the cursor continues where the page ended")
		})
	}
}
