package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// AccountChecker reports whether a human account may still authenticate: the user exists
// and is not soft-deleted (SPEC Part 20.2). A JWT outlives the account it was minted for,
// so every middleware that accepts one asks before trusting its subject.
type AccountChecker interface {
	IsActiveUser(ctx context.Context, userID string) (bool, error)
}

// errAccountUnavailable marks an account check that could not run (the store is down).
type errAccountUnavailable struct{ err error }

func (e errAccountUnavailable) Error() string { return "account check unavailable: " + e.err.Error() }

// checkJWTAccount returns nil when claims name a live account, an AuthError when the
// account is gone, and errAccountUnavailable when the check itself failed. A nil checker
// trusts the signature alone.
func checkJWTAccount(ctx context.Context, accounts AccountChecker, claims *Claims) error {
	if accounts == nil {
		return nil
	}
	active, err := accounts.IsActiveUser(ctx, claims.UserID)
	if err != nil {
		slog.Error("JWT account check failed", "user_id", claims.UserID, "error", err)
		return errAccountUnavailable{err}
	}
	if !active {
		return NewAuthError(ErrCodeUnauthorized, "account no longer exists")
	}
	return nil
}

// writeJWTAccountError answers a failed account check: 503 when the check could not run,
// so a transient outage does not read as a bad credential, 401 otherwise.
func writeJWTAccountError(w http.ResponseWriter, err error) {
	if _, ok := err.(errAccountUnavailable); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"error": map[string]interface{}{
				"code":    "SERVICE_UNAVAILABLE",
				"message": "could not verify the account, retry shortly",
			},
		})
		return
	}
	writeAuthError(w, err)
}
