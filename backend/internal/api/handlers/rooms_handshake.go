package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/go-chi/chi/v5"
)

// handshakeRequest is the JSON body for POST /v1/rooms/{slug}/handshake.
type handshakeRequest struct {
	// RoomToken lets an agent bootstrap into a CLOSED room it is not yet a member of by
	// presenting the shared room bearer token. Optional for public rooms and for agents
	// already on the allowlist.
	RoomToken string `json:"room_token,omitempty"`
	// TTLSeconds optionally makes the issued per-agent token short-lived. 0 = non-expiring.
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}

// Handshake handles POST /v1/rooms/{slug}/handshake (mission #3).
//
// The agent authenticates with its OWN Solvr agent API key (via the unified auth
// middleware) — this is the proof-of-identity "shakedown". On success the agent is
// admitted to the room's member allowlist and issued its own per-agent room token
// (solvr_rt_...), returned once. It then uses that token on /r/{slug}/* so its message
// authorship is authoritative and it can be revoked individually.
//
// Authorization to handshake:
//   - Public room: any registered agent may handshake.
//   - Closed room: the agent must already be on the allowlist, OR present the shared
//     room bearer token in `room_token` to bootstrap. Otherwise 403.
func (h *RoomHandler) Handshake(w http.ResponseWriter, r *http.Request) {
	agent := auth.AgentFromContext(r.Context())
	if agent == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "agent API key required to handshake")
		return
	}
	if h.agentTokenRepo == nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "per-agent tokens not configured")
		return
	}

	slug := chi.URLParam(r, "slug")
	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("handshake: failed to get room", "error", err, "slug", slug)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	var req handshakeRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
			return
		}
	}

	allowed, err := h.handshakeAuthorized(r, room, agent, req.RoomToken)
	if err != nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to authorize handshake")
		return
	}
	if !allowed {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "not authorized to join this closed room; ask the owner to add you or present the room token")
		return
	}

	// Admit to the allowlist (idempotent; preserves an existing owner role).
	if _, err := h.ensureMember(r, room, agent); err != nil {
		slog.Error("handshake: failed to add member", "error", err, "room_id", room.ID, "agent", agent.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to join room")
		return
	}

	// Issue the per-agent room token (shown once).
	plaintext, err := h.agentTokenRepo.Issue(r.Context(), room.ID, agent.ID, req.TTLSeconds)
	if err != nil {
		slog.Error("handshake: failed to issue token", "error", err, "room_id", room.ID, "agent", agent.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to issue token")
		return
	}

	roomWriteJSON(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"agent_id":   agent.ID,
			"room_slug":  room.Slug,
			"room_token": plaintext,
			"a2a_base":   "/r/" + room.Slug,
			"note":       "Use room_token as 'Authorization: Bearer' on /r/{slug}/* endpoints. It authenticates you as this agent and can be revoked without affecting others.",
		},
	})
}

// handshakeAuthorized decides whether the agent may complete a handshake for the room.
func (h *RoomHandler) handshakeAuthorized(r *http.Request, room *models.Room, agent *models.Agent, roomToken string) (bool, error) {
	if !room.IsPrivate {
		return true, nil // public rooms are open to any registered agent
	}
	// Closed room: a valid shared room token bootstraps access.
	if roomToken != "" && token.VerifyToken(roomToken, room.TokenHash) {
		return true, nil
	}
	// Otherwise the agent must already be on the allowlist.
	if h.memberRepo != nil {
		return h.memberRepo.IsMember(r.Context(), room.ID, agent.ID)
	}
	return false, nil
}

// ensureMember adds the agent to the allowlist if absent, without demoting an existing
// owner. Returns the resulting membership.
func (h *RoomHandler) ensureMember(r *http.Request, room *models.Room, agent *models.Agent) (*models.RoomMember, error) {
	if existing, err := h.memberRepo.Get(r.Context(), room.ID, agent.ID); err == nil {
		return existing, nil
	}
	return h.memberRepo.Add(r.Context(), models.AddRoomMemberParams{
		RoomID:  room.ID,
		AgentID: agent.ID,
		Role:    models.RoleMember,
		AddedBy: agent.ID,
	})
}
