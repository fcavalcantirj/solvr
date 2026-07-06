package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSearch_ViewerScoped_HumanCaller is the BART-153 guarantee: GET /v1/search returns
// what the CALLER is authorized to see = own + family (private) + public. Prior tests
// (BART-151/152) only exercised claimed-AGENT keys; this locks the contract for a HUMAN
// caller (JWT) and asserts meta.total is itself viewer-scoped (not a public-only count).
//
// setupRoomTestServer wires NO embedding service, so /v1/search runs the full-text path.
// The server's default JWT secret matches the one createRoomTestUser signs with, so do NOT
// run this with JWT_SECRET set in the environment.
func TestSearch_ViewerScoped_HumanCaller(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()

	marker := fmt.Sprintf("bart153_%d", time.Now().UnixNano()%1000000000)
	kw := "zqxviewerkw" // rare token in BOTH the private and public posts; drives the tsquery.

	// Two humans. createRoomTestUser returns (userID, JWT signed with the server's secret).
	userA, jwtA := createRoomTestUser(t, pool)
	_, jwtB := createRoomTestUser(t, pool)

	// A FAMILY (private) question owned by userA + a PUBLIC question, both matching kw,
	// each carrying a per-visibility needle so we can assert exactly what a caller sees.
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,owner_human_id)
		 VALUES ('question',$1,$2,'human',$3,'open','family',$4::uuid) RETURNING id::text`,
		"PRIV VIEWER "+marker, "private "+kw+" PRIVNEEDLE "+marker, userA, userA).Scan(new(string)))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility)
		 VALUES ('question',$1,$2,'human',$3,'open','public') RETURNING id::text`,
		"PUB VIEWER "+marker, "public "+kw+" PUBNEEDLE "+marker, userA).Scan(new(string)))
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM posts WHERE title LIKE '%"+marker+"%'") }) //nolint:errcheck

	// search does GET /v1/search (optional bearer) and returns (body, meta.total).
	search := func(bearer string) (string, int) {
		req, _ := http.NewRequest("GET", ts.URL+"/v1/search?q="+kw+"&per_page=50", nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		var parsed struct {
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		_ = json.Unmarshal(b, &parsed)
		return string(b), parsed.Meta.Total
	}

	bodyA, totalA := search(jwtA)
	bodyB, totalB := search(jwtB)
	bodyAnon, totalAnon := search("")

	// Owner (userA) sees BOTH own-private AND public in ONE response — the "own + family + public" contract.
	require.Contains(t, bodyA, "PRIVNEEDLE "+marker, "owner (human JWT) must see own family-private post")
	require.Contains(t, bodyA, "PUBNEEDLE "+marker, "owner must also see the public post")

	// Foreign human sees public only.
	require.NotContains(t, bodyB, "PRIVNEEDLE "+marker, "foreign human must NOT see another family's private post")
	require.Contains(t, bodyB, "PUBNEEDLE "+marker, "foreign human sees the public post")

	// Anonymous sees public only.
	require.NotContains(t, bodyAnon, "PRIVNEEDLE "+marker, "anon must NOT see any private post")
	require.Contains(t, bodyAnon, "PUBNEEDLE "+marker, "anon sees the public post")

	// meta.total is itself viewer-scoped: owner counts own-private + public; foreign/anon count public only.
	require.Greater(t, totalA, totalAnon, "meta.total for owner must exceed anon (count is viewer-scoped, not just rows)")
	require.Equal(t, totalAnon, totalB, "foreign human total must equal anon total (both public-only)")
}
