package services

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Anti-abuse W0, on the real repositories: a soft-deleted (tombstoned) identity signing in
// again through OAuth is refused promptly and explicitly, and leaves nothing behind.

// countingUserRepo is the real user repository, counting provider lookups so a loop in
// FindOrCreateUser shows up as a number.
type countingUserRepo struct {
	*db.UserRepository
	providerLookups atomic.Int64
}

func (r *countingUserRepo) FindByAuthProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	r.providerLookups.Add(1)
	return r.UserRepository.FindByAuthProvider(ctx, provider, providerID)
}

type oauthOutcome struct {
	err     error
	elapsed time.Duration
}

// findOrCreateWithDeadline calls FindOrCreateUser under a 3s deadline and fails the test if
// the call has not returned 10s later, so a hang fails instead of wedging the run.
func findOrCreateWithDeadline(t *testing.T, svc *OAuthUserService, info *OAuthUserInfo) oauthOutcome {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan oauthOutcome, 1)
	start := time.Now()
	go func() {
		_, _, err := svc.FindOrCreateUser(ctx, info)
		done <- oauthOutcome{err: err, elapsed: time.Since(start)}
	}()
	select {
	case out := <-done:
		return out
	case <-time.After(10 * time.Second):
		t.Fatal("FindOrCreateUser did not return within 10s")
		return oauthOutcome{}
	}
}

// seedTombstonedOAuthUser creates a GitHub user through the repositories every signup uses,
// then soft-deletes it the way an account tombstone does (auth_methods are kept).
func seedTombstonedOAuthUser(t *testing.T, pool *db.Pool) (providerID, email string) {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	return seedTombstonedUser(t, pool, models.AuthProviderGitHub, "tomb_"+suffix, suffix)
}

// seedTombstonedUser is seedTombstonedOAuthUser for a given provider identity; suffix makes
// the username and email unique.
func seedTombstonedUser(t *testing.T, pool *db.Pool, provider, providerID, suffix string) (string, string) {
	t.Helper()
	ctx := context.Background()
	email := "tomb_" + suffix + "@test.solvr.dev"
	users := db.NewUserRepository(pool)
	user, err := users.Create(ctx, &models.User{
		Username: "tomb_" + suffix, DisplayName: "Tombstoned " + suffix, Email: email,
		AuthProvider: provider, AuthProviderID: providerID, Role: models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := db.NewAuthMethodRepository(pool).Create(ctx, &models.AuthMethod{
		UserID: user.ID, AuthProvider: provider, AuthProviderID: providerID, LastUsedAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed auth method: %v", err)
	}
	if err := users.Delete(ctx, user.ID); err != nil {
		t.Fatalf("soft-delete user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM auth_methods WHERE auth_provider_id = $1", providerID)         //nolint:errcheck
		pool.Exec(context.Background(), "DELETE FROM users WHERE email LIKE $1", "tomb_"+suffix+"%@test.solvr.dev") //nolint:errcheck
	})
	return providerID, email
}

func oauthTestPool(t *testing.T) *db.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func assertPromptSuspension(t *testing.T, out oauthOutcome, lookups int64) {
	t.Helper()
	if out.elapsed > time.Second {
		t.Errorf("returned after %s (%d provider lookups); want an answer within 1s", out.elapsed.Round(time.Millisecond), lookups)
	}
	if !errors.Is(out.err, ErrAccountSuspended) || !strings.Contains(out.err.Error(), "account suspended") {
		t.Errorf("error = %v (%d provider lookups); want an explicit 'account suspended' refusal", out.err, lookups)
	}
}

// Same provider identity and same email as the tombstoned account: today's code finds nothing
// live, fails Create on users_email_key, and calls itself again with no limit.
func TestFindOrCreateUser_TombstonedIdentityIsRefusedPromptly(t *testing.T) {
	pool := oauthTestPool(t)
	providerID, email := seedTombstonedOAuthUser(t, pool)
	repo := &countingUserRepo{UserRepository: db.NewUserRepository(pool)}
	svc := NewOAuthUserService(repo, db.NewAuthMethodRepository(pool))
	svc.WithIdentityGate(db.NewBannedIdentityRepository(pool))

	out := findOrCreateWithDeadline(t, svc, &OAuthUserInfo{
		Provider: models.AuthProviderGitHub, ProviderID: providerID, Email: email, DisplayName: "Returning spammer",
	})
	assertPromptSuspension(t, out, repo.providerLookups.Load())
}

// Same provider identity with a new email: today's code creates a users row, then fails the
// auth_methods insert on auth_methods_unique_oauth_id, leaving that row orphaned.
func TestFindOrCreateUser_TombstonedProviderWithNewEmailLeavesNoOrphan(t *testing.T) {
	pool := oauthTestPool(t)
	providerID, _ := seedTombstonedOAuthUser(t, pool)
	newEmail := "tomb_new_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12] + "@test.solvr.dev"
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM users WHERE email = $1", newEmail) //nolint:errcheck
	})
	repo := &countingUserRepo{UserRepository: db.NewUserRepository(pool)}
	svc := NewOAuthUserService(repo, db.NewAuthMethodRepository(pool))
	svc.WithIdentityGate(db.NewBannedIdentityRepository(pool))

	out := findOrCreateWithDeadline(t, svc, &OAuthUserInfo{
		Provider: models.AuthProviderGitHub, ProviderID: providerID, Email: newEmail, DisplayName: "Returning spammer",
	})
	assertPromptSuspension(t, out, repo.providerLookups.Load())

	var orphans int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE email = $1", newEmail).Scan(&orphans); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if orphans != 0 {
		t.Errorf("%d users row(s) left behind for %s; want none", orphans, newEmail)
	}
}
