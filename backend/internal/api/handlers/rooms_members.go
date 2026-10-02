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
// allowlist (mission #3). Idempotent; an explicit role promotes or demotes.
func (h *RoomHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	room := h.resolveRoomForManage(w, r)
	if room == nil {
		return
	}
	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.AgentID == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_id is required")
		return
	}
	// An omitted role adds a new agent as a member and leaves an existing member's role
	// as it is (a plain re-add or its retry never demotes an owner).
	role := req.Role
	if role != "" && role != models.RoleMember && role != models.RoleOwner {
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
	if errors.Is(err, db.ErrLastRoomOwner) {
		writeLastOwnerError(w)
		return
	}
	if err != nil {
		// A missing agent violates the FK — report as a clear 400.
		roomWriteError(w, http.StatusBadRequest, "INVALID_AGENT", "agent_id does not reference an existing agent")
		return
	}
	if member.Admitted {
		h.notifyMember(r, room, member.AgentID, models.NotificationRoomMemberAdded, member.Role)
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
		if errors.Is(err, db.ErrLastRoomOwner) {
			writeLastOwnerError(w)
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
	h.notifyMember(r, room, agentID, models.NotificationRoomMemberRemoved, "")
	w.WriteHeader(http.StatusNoContent)
}

// RevokeMemberToken handles DELETE /v1/rooms/{slug}/members/{agent_id}/token — owner
// revokes ONE participant's per-agent room token without touching its membership or any
// other participant. The token is dead at once on every surface (REST, /r adapter, open
// streams on every instance via the room access notice); the agent, still a member, may
// prove its identity again with its own API key to get a new token. Idempotent while the
// membership is active.
func (h *RoomHandler) RevokeMemberToken(w http.ResponseWriter, r *http.Request) {
	room := h.resolveRoomForManage(w, r)
	if room == nil {
		return
	}
	agentID := chi.URLParam(r, "agent_id")
	isMember, err := h.memberRepo.IsMember(r.Context(), room.ID, agentID)
	if err != nil {
		slog.Error("failed to check room member for token revoke", "error", err, "room_id", room.ID, "agent", agentID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to revoke token")
		return
	}
	if !isMember {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "agent is not a member of this room")
		return
	}
	if err := h.agentTokenRepo.Revoke(r.Context(), room.ID, agentID); err != nil {
		slog.Error("failed to revoke per-agent room token", "error", err, "room_id", room.ID, "agent", agentID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to revoke token")
		return
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

// writeLastOwnerError reports the final-owner guard: a live room always keeps an owner.
func writeLastOwnerError(w http.ResponseWriter) {
	roomWriteError(w, http.StatusConflict, "LAST_OWNER",
		"a room must keep at least one owner; add another owner first or delete the room")
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
