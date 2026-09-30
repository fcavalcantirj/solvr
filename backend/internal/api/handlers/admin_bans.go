package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// AccountBanner tombstones an account and bans its identities (db.AccountBanRepository).
type AccountBanner interface {
	BanAccount(ctx context.Context, req db.BanRequest) (*db.BanResult, error)
}

// AdminBansHandler serves POST /admin/bans.
type AdminBansHandler struct {
	bans AccountBanner
}

// NewAdminBansHandler creates an AdminBansHandler.
func NewAdminBansHandler(bans AccountBanner) *AdminBansHandler {
	return &AdminBansHandler{bans: bans}
}

// BanRequestBody is the body of POST /admin/bans.
type BanRequestBody struct {
	AccountType  string `json:"account_type"` // human | agent
	AccountID    string `json:"account_id"`
	Reason       string `json:"reason"`
	IncludeOwner bool   `json:"include_owner"` // also ban the human who claimed the agent
	DryRun       bool   `json:"dry_run"`
}

// Ban handles POST /admin/bans: in one transaction it soft-deletes the account, removes its
// credentials and puts its email, OAuth identities or agent id on the ban list. A dry run
// reports the same result and writes nothing. There is no unban endpoint.
func (h *AdminBansHandler) Ban(w http.ResponseWriter, r *http.Request) {
	if !(&AdminHandler{}).checkAdminAuth(w, r) {
		return
	}
	var body BanRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	res, err := h.bans.BanAccount(r.Context(), db.BanRequest{
		AccountType: body.AccountType, AccountID: body.AccountID, Reason: body.Reason,
		IncludeOwner: body.IncludeOwner, DryRun: body.DryRun,
	})
	switch {
	case errors.Is(err, db.ErrInvalidBanRequest):
		writeAdminError(w, http.StatusBadRequest, "VALIDATION_ERROR", "account_type (human|agent), account_id and reason are required")
		return
	case errors.Is(err, db.ErrNotFound):
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "account not found")
		return
	case err != nil:
		slog.Error("ban failed", "error", err, "account_type", body.AccountType, "account_id", body.AccountID)
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ban failed; nothing was written")
		return
	}
	if !body.DryRun {
		slog.Warn("account banned", "account_type", res.AccountType, "account_id", res.AccountID, "identities", len(res.Identities))
	}
	writeAdminJSON(w, http.StatusOK, map[string]interface{}{"data": res, "dry_run": body.DryRun})
}
