package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Lookups shared by a resource's own GET and the lists under it, so a child list answers
// exactly what its parent answers for an id that names nothing (idx 74 step 3) instead of
// an empty 200 list.

// publicUserFinder is the user lookup GET /v1/users/{id} and its lists share.
type publicUserFinder interface {
	FindByID(ctx context.Context, id string) (*models.User, error)
}

// findPublicUser returns the user userID names, or writes GET /v1/users/{id}'s refusal and
// returns nil: 400 BAD_REQUEST for a missing or malformed id, 404 NOT_FOUND for an absent
// or deleted user, 500 when the lookup itself fails.
func findPublicUser(ctx context.Context, w http.ResponseWriter, users publicUserFinder, userID string) *models.User {
	if userID == "" {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "user ID is required")
		return nil
	}
	// Validate UUID format to prevent DB errors (e.g. /v1/users/me matching {id})
	if _, err := uuid.Parse(userID); err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid user ID format")
		return nil
	}
	user, err := users.FindByID(ctx, userID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch user")
		return nil
	}
	if user == nil {
		writeUsersError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return nil
	}
	return user
}

// isAgentNotFound matches both agent-not-found sentinels (the handler's and the db
// repository's), as GET /v1/agents/{id} does.
func isAgentNotFound(err error) bool {
	return errors.Is(err, ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound)
}
