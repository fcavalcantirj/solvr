package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accountRemovalFixture is two agents and two users with reputation, a tagged post each and
// an upvote on it, so every public account projection lists them; keepAgent/keepUser stay.
type accountRemovalFixture struct {
	tag                           string
	delAgent, banAgent, keepAgent string
	delUser, banUser, keepUser    string
}

func newAccountRemovalFixture(t *testing.T, pool *Pool) accountRemovalFixture {
	t.Helper()
	ctx := context.Background()
	sfx := time.Now().Format("150405.000000")
	n := time.Now().UnixNano() % 1000000000
	f := accountRemovalFixture{
		tag:      fmt.Sprintf("acctrm%d", n),
		delAgent: "agent_acdel_" + sfx, banAgent: "agent_acban_" + sfx, keepAgent: "agent_ackeep_" + sfx,
	}
	users := NewUserRepository(pool)
	newUser := func(label string) string {
		u, err := users.Create(ctx, &models.User{
			Username: fmt.Sprintf("ac%s%d", label, n), DisplayName: "Account removal " + label,
			Email: fmt.Sprintf("ac%s%d@example.com", label, n), AuthProvider: models.AuthProviderGitHub,
			AuthProviderID: fmt.Sprintf("gh_ac%s%d", label, n), Role: models.UserRoleUser,
		})
		require.NoError(t, err)
		return u.ID
	}
	f.delUser, f.banUser, f.keepUser = newUser("del"), newUser("ban"), newUser("keep")

	for _, id := range []string{f.delAgent, f.banAgent, f.keepAgent} {
		insertRemapAgent(t, pool, ctx, id)
		_, err := pool.Exec(ctx, `INSERT INTO agent_reputation_grants (agent_id, grant_key, points)
			VALUES ($1, 'model_declared', 10)`, id)
		require.NoError(t, err)
	}
	owners := []struct{ kind, id string }{
		{"agent", f.delAgent}, {"agent", f.banAgent}, {"agent", f.keepAgent},
		{"human", f.delUser}, {"human", f.banUser}, {"human", f.keepUser},
	}
	for _, o := range owners {
		var postID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id, status, publication_state, moderation_state)
			VALUES ('question', $1, 'A post that earns its author a place on the leaderboard.', $2, $3, $4, 'open', 'published', 'approved')
			RETURNING id`, "Account removal post "+o.id, []string{f.tag}, o.kind, o.id).Scan(&postID))
		_, err := pool.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ('post', $1, 'agent', $2, 'up', true)`, postID, "agent_acvoter_"+sfx)
		require.NoError(t, err)
	}
	return f
}

// removedAccountsListed reports which of the fixture's accounts each public account
// projection names: the leaderboard (all, agents, users, the tag board), the sitemap URLs
// (agents, users, the all-types listing). The agent count is returned beside it.
func removedAccountsListed(t *testing.T, pool *Pool, f accountRemovalFixture) (map[string][]string, int) {
	t.Helper()
	ctx := context.Background()
	ids := []string{f.delAgent, f.banAgent, f.keepAgent, f.delUser, f.banUser, f.keepUser}
	named := func(found map[string]bool) []string {
		out := []string{}
		for _, id := range ids {
			if found[id] {
				out = append(out, id)
			}
		}
		return out
	}
	board := NewCanonicalLeaderboardRepository(pool)
	main := func(opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
		return board.GetLeaderboard(ctx, opts)
	}
	listed := map[string][]string{}
	for _, view := range []string{"all", "agents", "users"} {
		entries := leaderboardEntriesFor(t, main, models.LeaderboardOptions{Type: view, Timeframe: "all_time"}, ids...)
		found := map[string]bool{}
		for id := range entries {
			found[id] = true
		}
		listed["leaderboard "+view] = named(found)
	}
	byTag := func(opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
		return board.GetLeaderboardByTag(ctx, f.tag, opts)
	}
	found := map[string]bool{}
	for id := range leaderboardEntriesFor(t, byTag, models.LeaderboardOptions{Type: "all", Timeframe: "all_time"}, ids...) {
		found[id] = true
	}
	listed["leaderboard tag"] = named(found)

	sitemap := NewSitemapRepository(pool)
	all, err := sitemap.GetSitemapURLs(ctx)
	require.NoError(t, err)
	found = map[string]bool{}
	for _, a := range all.Agents {
		found[a.ID] = true
	}
	listed["sitemap all"] = named(found)
	for _, kind := range []string{"agents", "users"} {
		page, err := sitemap.GetPaginatedSitemapURLs(ctx, models.SitemapURLsOptions{Type: kind, Page: 1, PerPage: 5000})
		require.NoError(t, err)
		found = map[string]bool{}
		for _, a := range page.Agents {
			found[a.ID] = true
		}
		for _, u := range page.Users {
			found[u.ID] = true
		}
		listed["sitemap "+kind] = named(found)
	}
	counts, err := sitemap.GetSitemapCounts(ctx)
	require.NoError(t, err)
	return listed, counts.Agents
}

// Task idx 77 step 5 ("verify visibility/deletion changes remove public projections
// promptly"), for accounts. Every way an account leaves Solvr sets deleted_at: self-deletion
// (AgentRepository.Delete, UserRepository.Delete) and an admin ban (AccountBanRepository).
// From the next read on, the public account projections no longer name it: the leaderboard
// (all, agents, users and the per-tag board), the sitemap URLs (agents, users, the all-types
// listing) and the sitemap's agent count. The accounts that stay keep appearing everywhere.
func TestPublicAccountProjections_DropARemovedAccountOnTheNextRead(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	f := newAccountRemovalFixture(t, pool)

	agents := []string{f.delAgent, f.banAgent, f.keepAgent}
	users := []string{f.delUser, f.banUser, f.keepUser}
	everyone := append(append([]string{}, agents...), users...)
	before, agentCount := removedAccountsListed(t, pool, f)
	assert.Equal(t, map[string][]string{
		"leaderboard all": everyone, "leaderboard agents": agents, "leaderboard users": users,
		"leaderboard tag": everyone, "sitemap all": agents, "sitemap agents": agents, "sitemap users": users,
	}, before, "before removal every public account projection names all six accounts")
	assert.Equal(t, 3, agentCount, "before removal the sitemap counts three agents")

	require.NoError(t, NewAgentRepository(pool).Delete(ctx, f.delAgent))
	require.NoError(t, NewUserRepository(pool).Delete(ctx, f.delUser))
	bans := NewAccountBanRepository(pool)
	for _, ban := range []BanRequest{
		{AccountType: "agent", AccountID: f.banAgent, Reason: "idx 77 account removal"},
		{AccountType: "human", AccountID: f.banUser, Reason: "idx 77 account removal"},
	} {
		_, err := bans.BanAccount(ctx, ban)
		require.NoError(t, err)
	}

	kept := []string{f.keepAgent, f.keepUser}
	after, agentCount := removedAccountsListed(t, pool, f)
	assert.Equal(t, map[string][]string{
		"leaderboard all": kept, "leaderboard agents": {f.keepAgent}, "leaderboard users": {f.keepUser},
		"leaderboard tag": kept, "sitemap all": {f.keepAgent}, "sitemap agents": {f.keepAgent}, "sitemap users": {f.keepUser},
	}, after, "a deleted or banned account leaves every public account projection on the next read")
	assert.Equal(t, 1, agentCount, "the sitemap no longer counts the removed agents")
}
