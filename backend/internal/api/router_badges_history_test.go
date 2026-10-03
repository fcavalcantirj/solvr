package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 4 (feature:badges): the contribution cutover keeps every earned badge as
// history and awards none. Badges earned under the legacy milestone rules are served
// byte-for-byte unchanged by GET /v1/agents/{id}/badges and GET /v1/users/{id}/badges once the
// migrated replies exist, and still appear in GET /v1/me. An agent whose migrated
// contributions meet every legacy milestone (a succeeded approach, an accepted answer) gets no
// badge from them: none on its badge list, none in /v1/me, none in /v1/me/diff badges_earned.
// The legacy tables are archived (000138), so the contributions are seeded as the cutover
// left them: replies carrying the approach's and the answer's provenance.
func TestBadgeRoutes_ServeEarnedBadgesAsHistoryAcrossTheCutover(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	router := NewRouter(pool, nil, nil)
	send := func(path, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	get := func(path, bearer string) string {
		rec := send(path, bearer)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", path, rec.Body.String())
		return rec.Body.String()
	}

	type registration struct {
		APIKey string `json:"api_key"`
		Agent  struct {
			ID string `json:"id"`
		} `json:"agent"`
	}
	register := func(name string) registration {
		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register",
			strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"badge history probe"}`, name)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		var reg registration
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&reg))
		require.NotEmpty(t, reg.APIKey)
		id := reg.Agent.ID
		t.Cleanup(func() {
			c := context.Background()
			pool.Exec(c, `DELETE FROM badges WHERE owner_type = 'agent' AND owner_id = $1`, id) //nolint:errcheck
			pool.Exec(c, `DELETE FROM replies WHERE author_id = $1`, id)                        //nolint:errcheck
			pool.Exec(c, `DELETE FROM agents WHERE id = $1`, id)                                //nolint:errcheck
		})
		return reg
	}
	// The user's posts are deleted last (cleanups run in reverse): the agents' replies point
	// at them.
	userID, _ := createLiveTestUser(t, pool, "user")
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM badges WHERE owner_type = 'human' AND owner_id = $1`, userID) //nolint:errcheck
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id = $1`, userID)                       //nolint:errcheck
	})
	name := fmt.Sprintf("bdgh_%d", time.Now().UnixNano()%1000000000)
	holder := register(name + "_h")
	newcomer := register(name + "_n")

	// History: badges earned under the legacy milestone rules, long before the cutover.
	earned := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	award := func(ownerType, ownerID, badgeType, badgeName, metadata string) {
		_, err := pool.Exec(ctx, `
			INSERT INTO badges (owner_type, owner_id, badge_type, badge_name, description, awarded_at, metadata)
			VALUES ($1, $2, $3, $4, 'earned before the cutover', $5, $6::jsonb)`,
			ownerType, ownerID, badgeType, badgeName, earned, metadata)
		require.NoError(t, err)
	}
	award("agent", holder.Agent.ID, "first_solve", "First Solve", `{"problem_id":"legacy-problem"}`)
	award("agent", holder.Agent.ID, "first_answer_accepted", "First Accepted Answer", `{}`)
	award("human", userID, "first_solve", "First Solve", `{}`)

	// The newcomer meets every legacy milestone that has a rule: a succeeded approach and an
	// accepted answer, each on a post of the user's.
	post := func(label string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id)
			VALUES ('post', $1, 'Legacy badge history fixture body', ARRAY['bdgh'], 'open', 'human', $2)
			RETURNING id::text`, "bdgh "+label+" "+name, userID).Scan(&id))
		return id
	}
	problem := post("problem")
	question := post("question")

	holderPath := "/v1/agents/" + holder.Agent.ID + "/badges"
	userPath := "/v1/users/" + userID + "/badges"
	holderBefore, userBefore := get(holderPath, ""), get(userPath, "")
	require.Contains(t, holderBefore, `"first_answer_accepted"`)
	require.Contains(t, userBefore, `"first_solve"`)
	since := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	// The newcomer's qualifying contributions as the cutover migrated them.
	tag, err := pool.Exec(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
		VALUES ($1, 'agent', $3, 'the solution', 'approach', gen_random_uuid(),
		        '{"legacy_table":"approaches","angle":"badge angle","method":"badge method","status":"succeeded","outcome":"worked","solution":"the solution"}'),
		       ($2, 'agent', $3, 'the accepted answer', 'answer', gen_random_uuid(),
		        '{"legacy_table":"answers","is_accepted":true}')`, problem, question, newcomer.Agent.ID)
	require.NoError(t, err)
	require.EqualValues(t, 2, tag.RowsAffected())

	require.JSONEq(t, holderBefore, get(holderPath, ""), "earned agent badges are history: unchanged by the cutover")
	require.JSONEq(t, userBefore, get(userPath, ""), "earned human badges are history: unchanged by the cutover")
	require.JSONEq(t, `{"data":{"badges":[]}}`, get("/v1/agents/"+newcomer.Agent.ID+"/badges", ""),
		"migrated contributions award no badge")

	type meBadges struct {
		Data struct {
			Badges []struct {
				BadgeType string    `json:"badge_type"`
				AwardedAt time.Time `json:"awarded_at"`
			} `json:"badges"`
			BadgesEarned []json.RawMessage `json:"badges_earned"`
		} `json:"data"`
	}
	decode := func(raw string) meBadges {
		var m meBadges
		require.NoError(t, json.Unmarshal([]byte(raw), &m), raw)
		return m
	}
	me := decode(get("/v1/me", holder.APIKey))
	require.Len(t, me.Data.Badges, 2)
	for _, b := range me.Data.Badges {
		require.True(t, earned.Equal(b.AwardedAt), "%s keeps its award time", b.BadgeType)
	}
	require.Empty(t, decode(get("/v1/me", newcomer.APIKey)).Data.Badges)

	diffPath := "/v1/me/diff?since=" + url.QueryEscape(since.Format(time.RFC3339))
	for _, key := range []string{holder.APIKey, newcomer.APIKey} {
		diff := decode(get(diffPath, key))
		require.NotNil(t, diff.Data.BadgesEarned)
		require.Empty(t, diff.Data.BadgesEarned, "the backfill earns no badge")
	}

	var total int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM badges WHERE owner_id IN ($1, $2, $3)`,
		holder.Agent.ID, newcomer.Agent.ID, userID).Scan(&total))
	require.Equal(t, 3, total)
}
