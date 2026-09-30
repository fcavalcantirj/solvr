package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// IdentityRefuser reports whether an identity is banned or belongs to a tombstoned account
// (db.BannedIdentityRepository.IsRefused). Anti-abuse W4: every sign-in and registration
// entry point asks before admitting anyone.
type IdentityRefuser interface {
	IsRefused(ctx context.Context, q db.IdentityQuery) (bool, error)
}

// refuseIdentity answers 403 ACCOUNT_SUSPENDED when q is refused, 500 when the check could not
// run, and reports whether it answered. A nil gate admits everyone.
func refuseIdentity(w http.ResponseWriter, r *http.Request, gate IdentityRefuser, q db.IdentityQuery) bool {
	if gate == nil {
		return false
	}
	refused, err := gate.IsRefused(r.Context(), q)
	if err != nil {
		slog.Error("identity check failed", "error", err, "path", r.URL.Path)
		writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check account status")
		return true
	}
	if refused {
		writeErrorResponse(w, http.StatusForbidden, "ACCOUNT_SUSPENDED", "this account is suspended")
		return true
	}
	return false
}

// WithIdentityGate makes Register and Login refuse banned and tombstoned emails.
func (h *AuthHandlers) WithIdentityGate(gate IdentityRefuser) *AuthHandlers {
	h.identityGate = gate
	return h
}

// WithIdentityGate makes ExchangeLoginCode refuse an account whose email is banned.
func (h *OAuthHandlers) WithIdentityGate(gate IdentityRefuser) *OAuthHandlers {
	h.identityGate = gate
	return h
}

// WithIdentityGate makes agent registration and both claim flows refuse banned identities.
func (h *AgentsHandler) WithIdentityGate(gate IdentityRefuser) *AgentsHandler {
	h.identityGate = gate
	return h
}

// WithIdentityGate makes the room handshake refuse a banned agent.
func (h *RoomHandler) WithIdentityGate(gate IdentityRefuser) *RoomHandler {
	h.identityGate = gate
	return h
}
