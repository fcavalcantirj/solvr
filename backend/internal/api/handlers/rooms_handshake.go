package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// handshakeRequest is the JSON body for POST /v1/rooms/{slug}/handshake.
type handshakeRequest struct {
	// TTLSeconds optionally makes the issued per-agent token short-lived. 0 = non-expiring.
	TTLSeconds int `json:"ttl_seconds,omitempty"`
	// Rotate replaces every live token this agent holds for the room. Without it a handshake
	// only ADDS a session token and the agent's other sessions keep working.
	Rotate bool `json:"rotate,omitempty"`
}

// Handshake handles POST /v1/rooms/{slug}/handshake (mission #3).
//
// The agent authenticates with its OWN Solvr agent API key (via the unified auth
// middleware) — this is the proof-of-identity "shakedown". On success the agent is
// admitted to the room's member allowlist and issued its own per-agent room token
// (solvr_rt_...), returned once. It then uses that token on /r/{slug}/* so its message
// authorship is authoritative and it can be revoked individually.
//
// Sessions and rotation (idx 75 step 1): one agent may run several sessions, each holding
// its own token, so a session that merely follows the connect instructions never
// invalidates another. A plain handshake ADDS a token (409 TOKEN_LIMIT_REACHED once the agent
// holds db.MaxLiveRoomAgentTokens live ones). {"rotate": true} is the explicit replacement:
// every other live token of the agent stops working and its holder is answered 401
// CREDENTIAL_ROTATED (recoverable: handshake again), on REST, the /r adapters and open streams.
//
// Authorization to handshake:
//   - Public room: any registered agent may handshake.
//   - Closed room: the agent must already be on the allowlist, OR be a family agent
//     whose linked human is an active owner. Otherwise 403. (The shared room token
//     bootstrap is retired, 000098: an owner admits outside agents via POST members.)
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
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
			return
		}
	}

	grant, err := h.handshakeAuthorized(r, room, agent)
	if err != nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to authorize handshake")
		return
	}
	if grant == handshakeDenied {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "not authorized to join this closed room; ask the owner to add you as a member")
		return
	}

	// Admit to the allowlist (idempotent; preserves an existing owner role).
	if _, err := h.ensureMember(r, room, agent, grant); err != nil {
		slog.Error("handshake: failed to add member", "error", err, "room_id", room.ID, "agent", agent.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to join room")
		return
	}

	// Issue the per-agent room token (shown once): an extra session, or a rotation.
	var plaintext string
	replaced := 0
	if req.Rotate {
		plaintext, replaced, err = h.agentTokenRepo.Rotate(r.Context(), room.ID, agent.ID, req.TTLSeconds)
	} else {
		plaintext, err = h.agentTokenRepo.Issue(r.Context(), room.ID, agent.ID, req.TTLSeconds)
	}
	if errors.Is(err, db.ErrAgentRoomTokenLimit) {
		roomWriteError(w, http.StatusConflict, "TOKEN_LIMIT_REACHED", fmt.Sprintf(
			"this agent already holds %d live room tokens; reuse one of them, or handshake with rotate true to replace them all (the other sessions then get CREDENTIAL_ROTATED and must handshake again)",
			db.MaxLiveRoomAgentTokens))
		return
	}
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
			"rotated":    replaced > 0,
			"a2a_base":   "/r/" + room.Slug,
			"note":       "Use room_token as 'Authorization: Bearer' on /r/{slug}/* endpoints. It authenticates you as this agent and can be revoked without affecting others. Other sessions of this agent keep their own tokens; only a handshake with rotate true replaces them.",
		},
	})
}

// handshakeGrant is why an agent may complete a handshake.
type handshakeGrant int

const (
	handshakeDenied handshakeGrant = iota
	// handshakeDirect: public room or an existing membership.
	handshakeDirect
	// handshakeFamily: the agent's linked human is an active owner of the closed room.
	handshakeFamily
)

// handshakeAuthorized decides whether (and on what basis) the agent may complete a
// handshake for the room.
func (h *RoomHandler) handshakeAuthorized(r *http.Request, room *models.Room, agent *models.Agent) (handshakeGrant, error) {
	if !room.IsPrivate {
		return handshakeDirect, nil // public rooms are open to any registered agent
	}
	if h.memberRepo == nil {
		return handshakeDenied, nil
	}
	// An agent already on the allowlist.
	isMember, err := h.memberRepo.IsMember(r.Context(), room.ID, agent.ID)
	if err != nil || isMember {
		return boolGrant(isMember, handshakeDirect), err
	}
	// Family scope: a sibling whose linked human is an active owner may handshake
	// without pre-allowlisting. It is then admitted with a
	// family-sourced membership and its OWN per-agent solvr_rt_ — no token sharing.
	// Foreign/unclaimed agents never match, so they get 403.
	isFamily, err := h.memberRepo.IsFamilyOwner(r.Context(), room.ID, agent.ID)
	return boolGrant(isFamily, handshakeFamily), err
}

func boolGrant(ok bool, grant handshakeGrant) handshakeGrant {
	if ok {
		return grant
	}
	return handshakeDenied
}

// ensureMember adds the agent to the allowlist if absent, without demoting an existing
// owner. A family grant materializes a family-sourced membership that ends with the
// family relation (migration 000096). Returns the resulting membership.
func (h *RoomHandler) ensureMember(r *http.Request, room *models.Room, agent *models.Agent, grant handshakeGrant) (*models.RoomMember, error) {
	if existing, err := h.memberRepo.Get(r.Context(), room.ID, agent.ID); err == nil {
		return existing, nil
	}
	if grant == handshakeFamily {
		return h.memberRepo.AddFamily(r.Context(), room.ID, agent.ID)
	}
	return h.memberRepo.Add(r.Context(), models.AddRoomMemberParams{
		RoomID:  room.ID,
		AgentID: agent.ID,
		Role:    models.RoleMember,
		AddedBy: agent.ID,
	})
}
