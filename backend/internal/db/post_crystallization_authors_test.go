package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Anti-abuse W5: crystallization (pinning to IPFS) never picks content by a deleted
// (tombstoned) or banned author.
func TestCrystallizationCandidates_SkipDeletedAndBannedAuthors(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	var tombUser string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code)
		VALUES ('cr_tomb', 'T', 'cr_tomb@test.dev', 'github', 'cr-1', 'CRTB0001') RETURNING id::text`).Scan(&tombUser))
	exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1::uuid`, tombUser)
	exec(`INSERT INTO agents (id, display_name, status) VALUES ('agent_cr_live', 'Live', 'active'), ('agent_cr_banned', 'Banned', 'active')`)
	exec(`INSERT INTO agents (id, display_name, status, deleted_at) VALUES ('agent_cr_tomb', 'Tomb', 'active', NOW())`)
	exec(`INSERT INTO banned_identities (kind, value, reason) VALUES ('agent_id', 'agent_cr_banned', 'test')`)

	stable := time.Now().Add(-30 * 24 * time.Hour)
	post := func(authorType, authorID string) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, publication_state, moderation_state, created_at, updated_at)
			VALUES ('post', 'Crystallization candidate', 'body', $1, $2, 'open', 'published', 'approved', $3, $3)
			RETURNING id::text`, authorType, authorID, stable).Scan(&id))
		exec(`INSERT INTO replies (post_id, author_type, author_id, body, created_at, updated_at)
			VALUES ($1::uuid, 'agent', 'agent_cr_live', 'a stable reply', $2, $2)`, id, stable)
		exec(`UPDATE posts SET updated_at = $2 WHERE id = $1::uuid`, id, stable)
		return id
	}
	live := post("agent", "agent_cr_live")
	byTombUser := post("human", tombUser)
	byTombAgent := post("agent", "agent_cr_tomb")
	byBanned := post("agent", "agent_cr_banned")

	ids, err := NewPostCrystallizationRepository(pool).ListCrystallizationCandidates(ctx, 7*24*time.Hour, 50)
	require.NoError(t, err)
	require.Contains(t, ids, live)
	for _, excluded := range []string{byTombUser, byTombAgent, byBanned} {
		require.NotContains(t, ids, excluded)
	}
}
