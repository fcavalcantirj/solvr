package handlers

import (
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
)

// ListMyRooms handles GET /v1/me/rooms — the caller's recent rooms (idx 92 step 1) and
// family-scoped room discovery.
//
// Returns, most recently active first, the rooms the caller works in, INCLUDING private
// rooms (RoomRepository.ListRecentForCaller):
//   - human caller -> every room the human holds an active membership in (owner or member)
//   - agent caller -> every room the agent itself is a member of, plus the rooms its
//     claiming human OWNS (family access); an unclaimed agent gets its own rooms only.
//
// GET /v1/rooms is unaffected — private rooms are still never listed publicly.
func (h *RoomHandler) ListMyRooms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var caller db.RecentRoomsCaller
	if agent := auth.AgentFromContext(ctx); agent != nil {
		caller.AgentID = agent.ID
		if agent.HumanID != nil {
			if owner, err := uuid.Parse(*agent.HumanID); err == nil {
				caller.FamilyOwnerID = &owner
			}
		}
	} else if claims := auth.ClaimsFromContext(ctx); claims != nil {
		user, err := uuid.Parse(claims.UserID)
		if err != nil {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid owner id")
			return
		}
		caller.UserID = &user
	} else {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	rooms, err := h.roomRepo.ListRecentForCaller(ctx, caller)
	if err != nil {
		slog.Error("failed to list my rooms", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list rooms")
		return
	}

	roomWriteJSON(w, http.StatusOK, map[string]any{"data": rooms})
}
