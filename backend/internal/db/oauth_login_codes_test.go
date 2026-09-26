package db_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
)

// idx 75 step 5: the OAuth callback must not put a bearer JWT in a URL (browser history, the
// frontend host's access log and Google Analytics all record the page URL). It redirects with
// a one-time login code instead; only a POST can turn the code into a token. The code is worth
// nothing once used or after its short life, so a copy of the URL that leaks is harmless.

func loginCodeFixture(t *testing.T) (context.Context, *db.Pool, *db.OAuthLoginCodeRepository, string) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	userID := uuid.NewString()
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, username, display_name, email, auth_provider, auth_provider_id, role, referral_code)
		 VALUES ($1, $2, 'Login Code User', $3, 'github', $4, 'user', $5)`,
		userID, "lc"+suffix, "lc"+suffix+"@test.com", "gh_lc_"+suffix, suffix[:8]); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return ctx, pool, db.NewOAuthLoginCodeRepository(pool), userID
}

func TestOAuthLoginCode_RedeemsOnceForTheUserItWasIssuedTo(t *testing.T) {
	ctx, _, repo, userID := loginCodeFixture(t)
	code, err := repo.Issue(ctx, userID, time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	user, err := repo.Redeem(ctx, code)
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if user.ID != userID || user.Role != "user" || !strings.HasSuffix(user.Email, "@test.com") {
		t.Fatalf("redeemed user = %+v, want id %s, role user and the account's email", user, userID)
	}
	if _, err := repo.Redeem(ctx, code); !errors.Is(err, db.ErrLoginCodeInvalid) {
		t.Fatalf("a code must work once: second Redeem err = %v, want ErrLoginCodeInvalid", err)
	}
}

func TestOAuthLoginCode_IsStoredAsAHashOnly(t *testing.T) {
	ctx, pool, repo, userID := loginCodeFixture(t)
	code, err := repo.Issue(ctx, userID, time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !strings.HasPrefix(code, "solvr_lc_") {
		t.Fatalf("code %q must carry the solvr_lc_ prefix so it is never mistaken for a bearer credential", code)
	}
	var leaked int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_login_codes WHERE code_hash = $1 OR code_hash LIKE '%' || $1 || '%'`, code).Scan(&leaked); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if leaked != 0 {
		t.Fatalf("the plaintext code is stored in oauth_login_codes")
	}
}

func TestOAuthLoginCode_ExpiredUnknownAndMalformedCodesAreInvalid(t *testing.T) {
	ctx, _, repo, userID := loginCodeFixture(t)
	expired, err := repo.Issue(ctx, userID, -time.Second)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	for name, code := range map[string]string{
		"expired":   expired,
		"unknown":   "solvr_lc_" + strings.Repeat("A", 43),
		"empty":     "",
		"a JWT":     "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.c2ln",
		"a prefix":  "solvr_lc_",
		"sql-ish":   "' OR 1=1 --",
		"very long": strings.Repeat("x", 10000),
	} {
		if _, err := repo.Redeem(ctx, code); !errors.Is(err, db.ErrLoginCodeInvalid) {
			t.Errorf("%s: Redeem err = %v, want ErrLoginCodeInvalid", name, err)
		}
	}
}

func TestOAuthLoginCode_ADeletedAccountCannotRedeem(t *testing.T) {
	ctx, pool, repo, userID := loginCodeFixture(t)
	code, err := repo.Issue(ctx, userID, time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, userID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := repo.Redeem(ctx, code); !errors.Is(err, db.ErrLoginCodeInvalid) {
		t.Fatalf("a code issued before the account was deleted must not mint a token: err = %v", err)
	}
}

func TestOAuthLoginCode_ManyRacingRedeemsHaveOneWinner(t *testing.T) {
	ctx, _, repo, userID := loginCodeFixture(t)
	code, err := repo.Issue(ctx, userID, time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.Redeem(ctx, code); err == nil {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("%d of 12 concurrent redeems succeeded, want exactly 1 (two API instances must not both mint a token)", winners.Load())
	}
}

func TestOAuthLoginCode_IssueSweepsLongExpiredCodes(t *testing.T) {
	ctx, pool, repo, userID := loginCodeFixture(t)
	if _, err := repo.Issue(ctx, userID, -48*time.Hour); err != nil {
		t.Fatalf("Issue (already long expired): %v", err)
	}
	if _, err := repo.Issue(ctx, userID, time.Minute); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	var stale int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_login_codes WHERE user_id = $1 AND expires_at < NOW() - INTERVAL '1 hour'`, userID).Scan(&stale); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if stale != 0 {
		t.Fatalf("%d long-expired codes survive the next Issue; the table would only ever grow", stale)
	}
}
