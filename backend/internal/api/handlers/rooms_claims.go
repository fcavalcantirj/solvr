package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// maxClaimTTLSeconds caps a claim lease at 24h to bound stale locks.
const maxClaimTTLSeconds = 86400

// RoomClaimsHandler serves the atomic claim/lease endpoints (mission #2) on the A2A
// namespace. All handlers run behind BearerGuard, so the room is in context.
type RoomClaimsHandler struct {
	claimRepo *db.RoomClaimRepository
}

// NewRoomClaimsHandler creates a new RoomClaimsHandler.
func NewRoomClaimsHandler(claimRepo *db.RoomClaimRepository) *RoomClaimsHandler {
	return &RoomClaimsHandler{claimRepo: claimRepo}
}

type acquireClaimRequest struct {
	Key        string `json:"key"`
	Agent      string `json:"agent"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"`
}

type claimActionRequest struct {
	Key        string `json:"key"`
	Agent      string `json:"agent"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"`
}

// Claim handles POST /r/{slug}/claim — compare-and-set acquire.
// Body: {key, agent, ttl_seconds?}. Returns {outcome: "won"|"held", claim}.
func (h *RoomClaimsHandler) Claim(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	var req acquireClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}
	if req.Key == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "key is required")
		return
	}
	if req.Agent == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent is required")
		return
	}
	if req.TTLSeconds > maxClaimTTLSeconds {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ttl_seconds exceeds maximum of 86400")
		return
	}

	claim, won, err := h.claimRepo.Acquire(r.Context(), models.AcquireClaimParams{
		RoomID:     room.ID,
		Key:        req.Key,
		Holder:     req.Agent,
		TTLSeconds: req.TTLSeconds,
	})
	if err != nil {
		slog.Error("failed to acquire claim", "error", err, "room_id", room.ID, "key", req.Key)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to acquire claim")
		return
	}

	outcome := models.ClaimOutcomeHeld
	if won {
		outcome = models.ClaimOutcomeWon
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"outcome": outcome,
			"claim":   claim,
		},
	})
}

// RenewClaim handles POST /r/{slug}/claim/renew — extend the caller's lease.
// Body: {key, agent, ttl_seconds?}. 409 if the caller is not the live holder.
func (h *RoomClaimsHandler) RenewClaim(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	req, ok := decodeClaimAction(w, r)
	if !ok {
		return
	}
	claim, err := h.claimRepo.Renew(r.Context(), room.ID, req.Key, req.Agent, req.TTLSeconds)
	if err != nil {
		if errors.Is(err, db.ErrClaimNotHeld) {
			roomWriteError(w, http.StatusConflict, "CLAIM_NOT_HELD", "you are not the current holder of this claim")
			return
		}
		slog.Error("failed to renew claim", "error", err, "room_id", room.ID, "key", req.Key)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to renew claim")
		return
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"claim": claim}})
}

// ReleaseClaim handles POST /r/{slug}/claim/release — free the caller's lock.
// Body: {key, agent}. 409 if the caller is not the holder.
func (h *RoomClaimsHandler) ReleaseClaim(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	req, ok := decodeClaimAction(w, r)
	if !ok {
		return
	}
	if err := h.claimRepo.Release(r.Context(), room.ID, req.Key, req.Agent); err != nil {
		if errors.Is(err, db.ErrClaimNotHeld) {
			roomWriteError(w, http.StatusConflict, "CLAIM_NOT_HELD", "you are not the current holder of this claim")
			return
		}
		slog.Error("failed to release claim", "error", err, "room_id", room.ID, "key", req.Key)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to release claim")
		return
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

// ListClaims handles GET /r/{slug}/claims — list live (non-expired) claims.
func (h *RoomClaimsHandler) ListClaims(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	claims, err := h.claimRepo.ListLive(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to list claims", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list claims")
		return
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": claims})
}

// decodeClaimAction decodes and validates a renew/release body, writing the error
// response and returning ok=false on failure.
func decodeClaimAction(w http.ResponseWriter, r *http.Request) (claimActionRequest, bool) {
	var req claimActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return req, false
	}
	if req.Key == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "key is required")
		return req, false
	}
	if req.Agent == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent is required")
		return req, false
	}
	if req.TTLSeconds > maxClaimTTLSeconds {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ttl_seconds exceeds maximum of 86400")
		return req, false
	}
	return req, true
}
