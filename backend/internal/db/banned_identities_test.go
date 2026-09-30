package db

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W4: the ban list, read and written on a freshly migrated scratch database.

func banTestSuffix() string { return strings.ReplaceAll(uuid.NewString(), "-", "")[:12] }

func TestBannedIdentities_IsRefused(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	repo := NewBannedIdentityRepository(pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}

	exec(`INSERT INTO banned_identities (kind, provider, value, reason) VALUES
		('email', '', 'banned@test.solvr.dev', 't'),
		('oauth', 'github', '4242', 't'),
		('agent_id', '', 'agent_banned', 't')`)
	// A tombstoned user (email and a Google identity) and a tombstoned agent, no ban rows.
	var tombUser string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code)
		VALUES ('tomb_user', 'T', 'Tomb@Test.Solvr.dev', 'google', 'g-77', 'TOMB0001') RETURNING id::text`).Scan(&tombUser))
	exec(`INSERT INTO auth_methods (user_id, auth_provider, auth_provider_id) VALUES ($1, 'github', '9090')`, tombUser)
	exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, tombUser)
	exec(`INSERT INTO agents (id, display_name, status, deleted_at) VALUES ('agent_tomb', 'Tomb Agent', 'active', NOW())`)
	exec(`INSERT INTO agents (id, display_name, status) VALUES ('agent_live', 'Live Agent', 'active')`)

	cases := []struct {
		name string
		q    IdentityQuery
		want bool
	}{
		{"banned email, other case", IdentityQuery{Email: "BANNED@test.solvr.dev"}, true},
		{"banned oauth", IdentityQuery{Provider: "github", ProviderID: "4242"}, true},
		{"same id, other provider", IdentityQuery{Provider: "google", ProviderID: "4242"}, false},
		{"banned agent", IdentityQuery{AgentID: "agent_banned"}, true},
		{"tombstoned email, no ban row", IdentityQuery{Email: "tomb@test.solvr.dev"}, true},
		{"tombstoned legacy provider column", IdentityQuery{Provider: "google", ProviderID: "g-77"}, true},
		{"tombstoned auth method", IdentityQuery{Provider: "github", ProviderID: "9090"}, true},
		{"tombstoned agent", IdentityQuery{AgentID: "agent_tomb"}, true},
		{"live agent", IdentityQuery{AgentID: "agent_live"}, false},
		{"unknown email", IdentityQuery{Email: "fresh@test.solvr.dev"}, false},
		{"empty query", IdentityQuery{}, false},
	}
	for _, tc := range cases {
		got, err := repo.IsRefused(ctx, tc.q)
		require.NoError(t, err, tc.name)
		require.Equal(t, tc.want, got, tc.name)
	}

	// The trigger (000114) refuses a banned email too, case-insensitively.
	_, err := pool.Exec(ctx, `INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code)
		VALUES ('ban_again', 'B', 'Banned@Test.Solvr.dev', 'github', 'x-1', 'BANA0001')`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account suspended")
}

func TestAccountBans_BanHumanAndAgentWithOwner(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	repo := NewAccountBanRepository(pool)
	s := banTestSuffix()

	var ownerID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code)
		VALUES ($1, 'Owner', $2, 'github', $3, 'OWNR0001') RETURNING id::text`, "own_"+s, "Owner_"+s+"@Test.dev", "gh-"+s).Scan(&ownerID))
	_, err := pool.Exec(ctx, `INSERT INTO auth_methods (user_id, auth_provider, auth_provider_id) VALUES ($1, 'google', $2)`, ownerID, "go-"+s)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, NOW() + interval '1 day')`, ownerID, "rt-"+s)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO agents (id, display_name, status, human_id, api_key_hash, key_sha256)
		VALUES ('agent_ban_'||$1, 'Ban Agent', 'active', $2::uuid, 'bcrypt', 'sha-'||$1)`, s, ownerID)
	require.NoError(t, err)
	agentID := "agent_ban_" + s

	// Dry run: the full report, no write.
	dry, err := repo.BanAccount(ctx, BanRequest{AccountType: "agent", AccountID: agentID, Reason: "spam", IncludeOwner: true, DryRun: true})
	require.NoError(t, err)
	require.NotNil(t, dry.Owner)
	require.Len(t, dry.Owner.Identities, 3, "email + github (users column) + google (auth method)")
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM banned_identities`))
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM agents WHERE id = $1 AND deleted_at IS NOT NULL`, agentID))

	res, err := repo.BanAccount(ctx, BanRequest{AccountType: "agent", AccountID: agentID, Reason: "spam", IncludeOwner: true})
	require.NoError(t, err)
	require.False(t, res.AlreadyDeleted)
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM agents
		WHERE id = $1 AND deleted_at IS NOT NULL AND api_key_hash IS NULL AND key_sha256 IS NULL`, agentID))
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM users WHERE id = $1::uuid AND deleted_at IS NOT NULL`, ownerID))
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM refresh_tokens WHERE user_id = $1::uuid`, ownerID))
	require.Equal(t, 4, countRows(t, pool, ctx, `SELECT count(*) FROM banned_identities`), "agent id, owner email, two OAuth ids")
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM banned_identities WHERE kind = 'email' AND value = $1`, "owner_"+s+"@test.dev"))

	// Banning again keeps the first deleted_at and adds no rows.
	again, err := repo.BanAccount(ctx, BanRequest{AccountType: "human", AccountID: ownerID, Reason: "again"})
	require.NoError(t, err)
	require.True(t, again.AlreadyDeleted)
	require.Equal(t, 4, countRows(t, pool, ctx, `SELECT count(*) FROM banned_identities`))

	_, err = repo.BanAccount(ctx, BanRequest{AccountType: "agent", AccountID: "agent_missing", Reason: "x"})
	require.True(t, errors.Is(err, ErrNotFound), "%v", err)
	_, err = repo.BanAccount(ctx, BanRequest{AccountType: "robot", AccountID: agentID, Reason: "x"})
	require.True(t, errors.Is(err, ErrInvalidBanRequest), "%v", err)
}

// A failure after the tombstone rolls the whole ban back: nothing is half-banned.
func TestAccountBans_FailureRollsBackTheTombstone(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, status, api_key_hash) VALUES ('agent_rb', 'RB', 'active', 'bcrypt')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		CREATE FUNCTION fail_ban() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced failure'; END $$;
		CREATE TRIGGER fail_ban BEFORE INSERT ON banned_identities FOR EACH ROW EXECUTE FUNCTION fail_ban();`)
	require.NoError(t, err)

	_, err = NewAccountBanRepository(pool).BanAccount(ctx, BanRequest{AccountType: "agent", AccountID: "agent_rb", Reason: "spam"})
	require.Error(t, err)
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM agents
		WHERE id = 'agent_rb' AND deleted_at IS NULL AND api_key_hash = 'bcrypt'`))
}
