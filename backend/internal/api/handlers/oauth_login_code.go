package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// maxLoginCodeBodyBytes bounds POST /v1/auth/oauth/exchange: the body is one short code.
const maxLoginCodeBodyBytes = 4 << 10

// OAuthLoginCodeStore holds the one-time codes the OAuth callbacks hand to the browser in place
// of a JWT (idx 75 step 5). db.OAuthLoginCodeRepository is the implementation: shared by every
// API instance, hash-only, single-use.
type OAuthLoginCodeStore interface {
	// Issue stores a code for userID that redeems for ttl and returns its plaintext once. origin
	// is what the callback knew: the provider, and whether this sign-in created the account.
	Issue(ctx context.Context, userID string, ttl time.Duration, origin db.LoginCodeOrigin) (string, error)
	// Redeem consumes a code and returns its account and origin, or db.ErrLoginCodeInvalid.
	Redeem(ctx context.Context, code string) (*db.OAuthLoginUser, error)
}

// WithLoginCodes sets the store the callbacks issue codes into and ExchangeLoginCode redeems
// from. Without one the callbacks refuse to log anyone in rather than fall back to a token in
// the redirect URL.
func (h *OAuthHandlers) WithLoginCodes(store OAuthLoginCodeStore) *OAuthHandlers {
	h.loginCodes = store
	return h
}

// redirectWithLoginCode sends the browser to the frontend callback page carrying a one-time
// login code for userID. The code is not a credential: it opens nothing until POSTed to
// ExchangeLoginCode, and only once, within auth.LoginCodeTTL. A JWT never rides in a URL, which
// browser history, the frontend host's access log and analytics page views all record. Nor does
// the origin: it is stored with the code and answered by the exchange alone.
func (h *OAuthHandlers) redirectWithLoginCode(w http.ResponseWriter, r *http.Request, userID string, origin db.LoginCodeOrigin) {
	if h.loginCodes == nil {
		slog.Error("OAuth login attempted without a login code store")
		h.redirectWithError(w, r, OAuthErrorLoginUnavailable)
		return
	}
	code, err := h.loginCodes.Issue(r.Context(), userID, auth.LoginCodeTTL, origin)
	if err != nil {
		slog.Error("Login code issue failed", "error", err)
		h.redirectWithError(w, r, OAuthErrorLoginFailed)
		return
	}

	// Per FE-022: Browser OAuth flow redirects to frontend callback page
	callbackURL := fmt.Sprintf("%s/auth/callback?code=%s", h.frontendBaseURL(), url.QueryEscape(code))
	http.Redirect(w, r, callbackURL, http.StatusFound)
}

// LoginCodeExchangeRequest is the request body for POST /v1/auth/oauth/exchange.
type LoginCodeExchangeRequest struct {
	LoginCode string `json:"login_code"`
}

// ExchangeLoginCode handles POST /v1/auth/oauth/exchange.
// It redeems the one-time code the OAuth callback put in the redirect and answers with the
// access token in the response body, where no URL, history entry or page view can record it.
// The answer also says what the callback stored with the code (SPEC.md 5.2): is_new_user, true
// only when that sign-in created the account, and its provider.
func (h *OAuthHandlers) ExchangeLoginCode(w http.ResponseWriter, r *http.Request) {
	var req LoginCodeExchangeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginCodeBodyBytes)).Decode(&req); err != nil {
		writeValidationError(w, "invalid JSON body")
		return
	}
	if req.LoginCode == "" {
		writeValidationError(w, "login_code is required")
		return
	}
	if h.loginCodes == nil {
		slog.Error("Login code exchange attempted without a login code store")
		writeInternalError(w, "Login is temporarily unavailable")
		return
	}

	user, err := h.loginCodes.Redeem(r.Context(), req.LoginCode)
	if errors.Is(err, db.ErrLoginCodeInvalid) {
		writeUnauthorized(w, "INVALID_LOGIN_CODE", "This login link is invalid, expired or already used. Sign in again.")
		return
	}
	if err != nil {
		slog.Error("Login code redeem failed", "error", err)
		writeInternalError(w, "Failed to complete the login")
		return
	}
	if refuseIdentity(w, r, h.identityGate, db.IdentityQuery{Email: user.Email}) {
		return
	}

	jwtExpiry, err := time.ParseDuration(h.config.JWTExpiry)
	if err != nil {
		jwtExpiry = 15 * time.Minute // Default
	}
	accessToken, err := auth.GenerateJWT(h.config.JWTSecret, user.ID, user.Email, user.Role, jwtExpiry)
	if err != nil {
		slog.Error("JWT generation failed", "error", err)
		writeInternalError(w, "Failed to generate access token")
		return
	}

	// A response that carries a token is never cached by the browser or an intermediary.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(jwtExpiry.Seconds()),
		"is_new_user":  user.IsNewUser,
		"provider":     user.Provider,
	})
}
