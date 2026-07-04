package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// addMemberRequest is the JSON body for POST /v1/rooms/{slug}/members.
type addMemberRequest struct {
	AgentID string `json:"agent_id"`
	Role    string `json:"role,omitempty"`
}

// resolveRoomForManage loads the room and verifies the caller may manage it, writing
// the appropriate error and returning nil on failure.
func (h *RoomHandler) resolveRoomForManage(w http.ResponseWriter, r *http.Request) *models.Room {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return nil
	}
	claims := auth.ClaimsFromContext(r.Context())
	agent := auth.AgentFromContext(r.Context())
	if claims == nil && agent == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return nil
	}
	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return nil
		}
		slog.Error("failed to get room for member management", "error", err, "slug", slug)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return nil
	}
	if !h.canManage(r.Context(), claims, agent, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner or admin can manage members")
		return nil
	}
	return room
}

// AddMember handles POST /v1/rooms/{slug}/members — owner adds an agent to the
// allowlist (mission #3). Idempotent; may also promote/demote via role.
func (h *RoomHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	room := h.resolveRoomForManage(w, r)
	if room == nil {
		return
	}
	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}
	if req.AgentID == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_id is required")
		return
	}
	role := req.Role
	if role == "" {
		role = models.RoleMember
	}
	if role != models.RoleMember && role != models.RoleOwner {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "role must be 'member' or 'owner'")
		return
	}

	addedBy := managerIdentity(r)
	member, err := h.memberRepo.Add(r.Context(), models.AddRoomMemberParams{
		RoomID:  room.ID,
		AgentID: req.AgentID,
		Role:    role,
		AddedBy: addedBy,
	})
	if err != nil {
		// A missing agent violates the FK — report as a clear 400.
		roomWriteError(w, http.StatusBadRequest, "INVALID_AGENT", "agent_id does not reference an existing agent")
		return
	}
	roomWriteJSON(w, http.StatusCreated, map[string]any{"data": member})
}

// RemoveMember handles DELETE /v1/rooms/{slug}/members/{agent_id} — owner revokes an
// agent's membership (mission #3). Also revokes that agent's per-agent room token, so
// a single agent is removed without rotating the shared token for everyone else.
func (h *RoomHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	room := h.resolveRoomForManage(w, r)
	if room == nil {
		return
	}
	agentID := chi.URLParam(r, "agent_id")
	if agentID == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_id is required")
		return
	}

	if err := h.memberRepo.Remove(r.Context(), room.ID, agentID); err != nil {
		if errors.Is(err, db.ErrRoomMemberNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "agent is not a member of this room")
			return
		}
		slog.Error("failed to remove room member", "error", err, "room_id", room.ID, "agent", agentID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove member")
		return
	}
	// Revoke the agent's per-agent room token (best-effort; membership removal already
	// blocks closed-room reads via the ACL).
	if h.agentTokenRepo != nil {
		if err := h.agentTokenRepo.Revoke(r.Context(), room.ID, agentID); err != nil {
			slog.Error("failed to revoke per-agent room token", "error", err, "room_id", room.ID, "agent", agentID)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers handles GET /v1/rooms/{slug}/members — owner views the allowlist.
func (h *RoomHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	room := h.resolveRoomForManage(w, r)
	if room == nil {
		return
	}
	members, err := h.memberRepo.ListByRoom(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to list room members", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list members")
		return
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": members})
}

// managerIdentity returns a short identifier for who performed a management action,
// used for the room_members.added_by audit column.
func managerIdentity(r *http.Request) string {
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		return agent.ID
	}
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		return claims.UserID
	}
	return "unknown"
}
