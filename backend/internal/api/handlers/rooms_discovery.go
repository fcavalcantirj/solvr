package handlers

import (
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// ListMyRooms handles GET /v1/me/rooms — family-scoped room discovery.
//
// Returns the rooms owned by the caller's human, INCLUDING private rooms. This lets an
// agent discover the closed rooms its siblings (agents claimed by the same human)
// coordinate in, without an out-of-band registry. Owner scoping:
//   - agent caller  -> agent.HumanID (unclaimed agent with nil HumanID gets an empty list)
//   - human caller  -> claims.UserID
//
// Only owner_id-scoped rooms are returned (via RoomRepository.ListByOwner); token_hash is
// never serialized (Room.TokenHash is json:"-"). GET /v1/rooms is unaffected — private
// rooms are still never listed publicly.
func (h *RoomHandler) ListMyRooms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var humanID string
	if agent := auth.AgentFromContext(ctx); agent != nil {
		if agent.HumanID == nil {
			// Unclaimed agent: no human, so no family rooms.
			roomWriteJSON(w, http.StatusOK, map[string]any{"data": []models.Room{}})
			return
		}
		humanID = *agent.HumanID
	} else if claims := auth.ClaimsFromContext(ctx); claims != nil {
		humanID = claims.UserID
	} else {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	ownerUUID, err := uuid.Parse(humanID)
	if err != nil {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid owner id")
		return
	}

	rooms, err := h.roomRepo.ListByOwner(ctx, ownerUUID)
	if err != nil {
		slog.Error("failed to list my rooms", "error", err, "owner_id", ownerUUID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list rooms")
		return
	}

	roomWriteJSON(w, http.StatusOK, map[string]any{"data": rooms})
}
