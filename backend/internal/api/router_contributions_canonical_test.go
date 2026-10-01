package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 73 step 3, "adapt then retire": GET /v1/users/{id}/contributions and GET
// /v1/me/contributions were adapted onto the replies migrated from a user's answers, approaches
// and responses (idx 76). They are retired now: through the real router both answer every caller
// 410 ENDPOINT_RETIRED, and the canonical GET /v1/replies?author_type=&author_id= serves what
// they listed — the migrated reply under its own id, never a legacy row that was not migrated —
// plus the author's replies written since the cutover, newest first, leaving out a reply on a
// post the caller may not read (the routes listed it with an empty parent_title).
func TestRetiredContributionListings_RepliesByAuthorServeWhatTheRoutesServed(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userID, jwt := createLiveTestUser(t, pool, "user")
	newPost := func(postType, title, visibility string) string {
		t.Helper()
		var id string
		var owner any // the owning family of a family post; NULL for a public one
		if visibility == "family" {
			owner = userID
		}
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
				publication_state, moderation_state, visibility, owner_human_id)
			VALUES ($1, $2, 'A post the probe user contributed to', ARRAY['probe'], 'open', 'human', $3,
				'published', 'approved', $4, $5::uuid)
			RETURNING id::text`, postType, title, userID, visibility, owner).Scan(&id))
		t.Cleanup(func() {
			bg := context.Background()
			_, _ = pool.Exec(bg, "DELETE FROM answers WHERE question_id = $1", id)
			_, _ = pool.Exec(bg, "DELETE FROM replies WHERE post_id = $1", id)
			_, _ = pool.Exec(bg, "DELETE FROM posts WHERE id = $1", id)
		})
		return id
	}
	questionID := newPost("question", "Contributions probe question", "public")
	problemID := newPost("problem", "Contributions probe problem", "public")
	familyID := newPost("question", "Contributions probe family question", "family")

	t0 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	reply := func(postID, body, legacyType, provenance string, at time.Time) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance,
				created_at, updated_at)
			VALUES ($1, 'human', $2, $3, NULLIF($4, ''), CASE WHEN $4 <> '' THEN gen_random_uuid() END,
				NULLIF($5, '')::jsonb, $6, $6)
			RETURNING id::text`, postID, userID, body, legacyType, provenance, at).Scan(&id))
		return id
	}
	answer := reply(questionID, "the answer the probe migrated", "answer", `{"legacy_table":"answers"}`, t0)
	approach := reply(problemID, "**Angle:** profile the heap", "approach",
		`{"legacy_table":"approaches","angle":"profile the heap","status":"failed"}`, t0.Add(time.Minute))
	onFamily := reply(familyID, "an answer on a family question", "answer", `{"legacy_table":"answers"}`, t0.Add(2*time.Minute))
	native := reply(questionID, "a reply written since the cutover", "", "", t0.Add(3*time.Minute))
	_, err = pool.Exec(ctx, `
		INSERT INTO answers (question_id, author_type, author_id, content)
		VALUES ($1, 'human', $2, 'a legacy answer that was never migrated')`, questionID, userID)
	require.NoError(t, err)

	router := NewRouter(pool, nil, nil, nil)
	serve := func(path, bearer string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("the contribution listings answer the migration error to every caller", func(t *testing.T) {
		for _, c := range []struct{ route, path, bearer string }{
			{"GET /v1/users/{id}/contributions", "/v1/users/" + userID + "/contributions", ""},
			{"GET /v1/users/{id}/contributions", "/v1/users/" + userID + "/contributions?type=answers&page=2", jwt},
			{"GET /v1/me/contributions", "/v1/me/contributions", jwt},
			{"GET /v1/me/contributions", "/v1/me/contributions", ""},
		} {
			rec := serve(c.path, c.bearer)
			requireRetiredRecorder(t, rec, c.route)
			assert.NotContains(t, rec.Body.String(), "the probe migrated", c.path)
		}
	})

	type page struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	list := func(query, bearer string) page {
		t.Helper()
		rec := serve("/v1/replies?author_type=human&author_id="+url.QueryEscape(userID)+query, bearer)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "never migrated", "a legacy row is not a reply")
		var p page
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
		return p
	}
	ids := func(p page) []string {
		out := []string{}
		for _, item := range p.Data {
			out = append(out, item["id"].(string))
		}
		return out
	}

	t.Run("GET /v1/replies lists the author's replies newest first, migrated ones under the reply id", func(t *testing.T) {
		p := list("", "")
		require.Equal(t, []string{native, approach, answer}, ids(p), "the family question's reply is hidden from anonymous")
		assert.Equal(t, float64(3), p.Meta["total"])
		assert.Equal(t, false, p.Meta["has_more"])

		byID := map[string]map[string]any{}
		for _, item := range p.Data {
			byID[item["id"].(string)] = item
		}
		a := byID[answer]
		assert.Equal(t, "answer", a["legacy_type"], "type was the legacy_type")
		assert.Equal(t, questionID, a["post_id"], "parent_id is post_id")
		assert.Equal(t, map[string]any{"id": questionID, "type": "question", "title": "Contributions probe question"},
			a["post"], "parent_type and parent_title are post.type and post.title")
		assert.Equal(t, "the answer the probe migrated", a["body"], "content_preview was the start of body")
		created, err := time.Parse(time.RFC3339Nano, a["created_at"].(string))
		require.NoError(t, err)
		assert.True(t, created.Equal(t0), "created_at is unchanged: %v", a["created_at"])
		assert.Equal(t, userID, a["author"].(map[string]any)["id"])

		ap := byID[approach]
		assert.Equal(t, "approach", ap["legacy_type"])
		prov := ap["provenance"].(map[string]any)
		assert.Equal(t, "failed", prov["status"], "status is provenance.status")
		assert.Equal(t, "profile the heap", prov["angle"], "an approach's preview was its angle")

		n := byID[native]
		_, typed := n["legacy_type"]
		assert.False(t, typed, "a reply written since the cutover carries no legacy_type: %v", n)
		assert.Equal(t, "Contributions probe question", n["post"].(map[string]any)["title"])
	})

	t.Run("the caller's family also reads the reply on its family post", func(t *testing.T) {
		p := list("", jwt)
		require.Equal(t, []string{native, onFamily, approach, answer}, ids(p))
		assert.Equal(t, float64(4), p.Meta["total"])
		assert.Equal(t, "Contributions probe family question", p.Data[1]["post"].(map[string]any)["title"])
	})

	t.Run("limit and cursor page through every reply once", func(t *testing.T) {
		var seen []string
		query := "&limit=1"
		for range 6 {
			p := list(query, "")
			seen = append(seen, ids(p)...)
			if p.Meta["has_more"] != true {
				break
			}
			cursor, _ := p.Meta["next_cursor"].(string)
			require.NotEmpty(t, cursor, "has_more without next_cursor: %v", p.Meta)
			query = "&limit=1&cursor=" + url.QueryEscape(cursor)
		}
		assert.Equal(t, []string{native, approach, answer}, seen)
	})

	t.Run("the retirements name this query", func(t *testing.T) {
		for _, ret := range LegacyReadRetirements {
			if strings.HasSuffix(ret.Route, "/contributions") {
				assert.Equal(t, "GET /v1/replies", ret.Replacement, ret.Route)
				assert.Contains(t, ret.Instructions, "GET /v1/replies?author_type=", ret.Route)
			}
		}
	})
}
