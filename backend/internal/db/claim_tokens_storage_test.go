package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Storage rules for claim tokens (idx 75, step 5): a live claim token is a credential, so
// the row holds its SHA-256 and a sealed copy, never the token itself.

func newClaimStorageAgent(t *testing.T, pool *Pool) string {
	t.Helper()
	ctx := context.Background()
	agentID := "claim_store_" + strings.ReplaceAll(t.Name(), "/", "_")
	if len(agentID) > 50 {
		agentID = agentID[:50]
	}
	_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name) VALUES ($1, $1)`, agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	})
	return agentID
}

func TestClaimTokenRepository_StoresAHashAndASealedCopyNeverTheToken(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewClaimTokenRepository(pool).WithSealSecret("server-secret")
	agentID := newClaimStorageAgent(t, pool)

	token := &models.ClaimToken{
		Token:     "storage_token_" + time.Now().Format("150405.000000"),
		AgentID:   agentID,
		ExpiresAt: time.Now().Add(4 * time.Hour),
	}
	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var hash string
	var sealed []byte
	var rowJSON string
	err := pool.QueryRow(ctx, `SELECT token_hash, token_sealed, row_to_json(c)::text FROM claim_tokens c WHERE id = $1`, token.ID).
		Scan(&hash, &sealed, &rowJSON)
	if err != nil {
		t.Fatalf("read the stored row: %v", err)
	}
	if hash != hashClaimToken(token.Token) {
		t.Errorf("token_hash = %s, want sha256(token) = %s", hash, hashClaimToken(token.Token))
	}
	if len(sealed) == 0 {
		t.Error("token_sealed is empty although the repository has a seal secret")
	}
	if strings.Contains(rowJSON, token.Token) {
		t.Errorf("the clear token is stored in the row: %s", rowJSON)
	}
	if strings.Contains(string(sealed), token.Token) {
		t.Error("the sealed copy contains the clear token")
	}
}

func TestClaimTokenRepository_TheClearTokenColumnIsGone(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	var n int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'claim_tokens' AND column_name = 'token'`).Scan(&n)
	if err != nil {
		t.Fatalf("read the schema: %v", err)
	}
	if n != 0 {
		t.Fatal("claim_tokens still has a clear-text `token` column")
	}
}

func TestClaimTokenRepository_ReissuesTheLiveTokenOnlyToTheHolderOfTheKey(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	agentID := newClaimStorageAgent(t, pool)

	value := "reissue_token_" + time.Now().Format("150405.000000")
	sealing := NewClaimTokenRepository(pool).WithSealSecret("server-secret")
	if err := sealing.Create(ctx, &models.ClaimToken{Token: value, AgentID: agentID, ExpiresAt: time.Now().Add(4 * time.Hour)}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, tc := range []struct {
		name string
		repo *ClaimTokenRepository
		want string
	}{
		{"same server secret", NewClaimTokenRepository(pool).WithSealSecret("server-secret"), value},
		{"another server secret", NewClaimTokenRepository(pool).WithSealSecret("rotated-secret"), ""},
		{"no server secret", NewClaimTokenRepository(pool), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.repo.FindActiveByAgentID(ctx, agentID)
			if err != nil || got == nil {
				t.Fatalf("FindActiveByAgentID = %v, %v; the live row must still be found", got, err)
			}
			if got.Token != tc.want {
				t.Errorf("Token = %q, want %q", got.Token, tc.want)
			}
			if !got.IsActive() {
				t.Error("the live row is not reported active")
			}
		})
	}
}

func TestClaimTokenRepository_WithoutASecretOnlyTheHashIsKept(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	agentID := newClaimStorageAgent(t, pool)

	token := &models.ClaimToken{Token: "unsealed_token_" + time.Now().Format("150405.000000"), AgentID: agentID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := NewClaimTokenRepository(pool).Create(ctx, token); err != nil {
		t.Fatalf("Create: %v", err)
	}
	var sealed []byte
	var rowJSON string
	if err := pool.QueryRow(ctx, `SELECT token_sealed, row_to_json(c)::text FROM claim_tokens c WHERE id = $1`, token.ID).Scan(&sealed, &rowJSON); err != nil {
		t.Fatalf("read the stored row: %v", err)
	}
	if sealed != nil {
		t.Error("a repository with no secret stored a sealed copy")
	}
	if strings.Contains(rowJSON, token.Token) {
		t.Errorf("the clear token is stored: %s", rowJSON)
	}
	found, err := NewClaimTokenRepository(pool).FindByToken(ctx, token.Token)
	if err != nil || found == nil || found.ID != token.ID {
		t.Fatalf("FindByToken = %v, %v; the hash must still find the row", found, err)
	}
}

func TestClaimTokenRepository_FindByTokenNeedsTheExactToken(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewClaimTokenRepository(pool).WithSealSecret("server-secret")
	agentID := newClaimStorageAgent(t, pool)

	value := "exact_token_" + time.Now().Format("150405.000000")
	if err := repo.Create(ctx, &models.ClaimToken{Token: value, AgentID: agentID, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByToken(ctx, value)
	if err != nil || found == nil {
		t.Fatalf("FindByToken(value) = %v, %v", found, err)
	}
	if found.Token != value || found.AgentID != agentID {
		t.Errorf("found %+v; the caller's token and the agent must come back", found)
	}
	for _, wrong := range []string{value + "x", value[:len(value)-1], strings.ToUpper(value), "", " " + value} {
		if got, err := repo.FindByToken(ctx, wrong); err != nil || got != nil {
			t.Errorf("FindByToken(%q) = %v, %v; only the exact token finds the row", wrong, got, err)
		}
	}
}

func TestClaimTokenRepository_DeleteUnusedByAgentID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewClaimTokenRepository(pool).WithSealSecret("server-secret")
	agentID := newClaimStorageAgent(t, pool)

	live := &models.ClaimToken{Token: "unused_live_" + time.Now().Format("150405.000000"), AgentID: agentID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := repo.Create(ctx, live); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// A used token is history (who claimed the agent) and must survive.
	if _, err := pool.Exec(ctx, `
		INSERT INTO claim_tokens (token_hash, agent_id, expires_at, used_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 hour', NOW())`, hashClaimToken("used_history"), agentID); err != nil {
		t.Fatalf("insert used token: %v", err)
	}

	deleted, err := repo.DeleteUnusedByAgentID(ctx, agentID)
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteUnusedByAgentID = %d, %v; want exactly the one live unused token", deleted, err)
	}
	if got, _ := repo.FindByToken(ctx, live.Token); got != nil {
		t.Error("the unused token is still there")
	}
	var kept int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM claim_tokens WHERE agent_id = $1 AND used_at IS NOT NULL`, agentID).Scan(&kept); err != nil || kept != 1 {
		t.Errorf("used tokens kept = %d, %v; want 1", kept, err)
	}
	// With the old one gone, a new live token can be created (the partial unique index).
	again := &models.ClaimToken{Token: "unused_again_" + time.Now().Format("150405.000000"), AgentID: agentID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := repo.Create(ctx, again); err != nil {
		t.Errorf("a fresh token after DeleteUnusedByAgentID: %v", err)
	}
}
