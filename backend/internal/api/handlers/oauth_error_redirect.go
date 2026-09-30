package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// OAuthErrorAccountSuspended is the ?error= code the frontend's /auth/callback page shows when
// a banned or tombstoned identity tries to sign in.
const OAuthErrorAccountSuspended = "account_suspended"

// frontendBaseURL is where browser OAuth flows land (FE-022).
func (h *OAuthHandlers) frontendBaseURL() string {
	if h.config.FrontendURL != "" {
		return h.config.FrontendURL
	}
	return "http://localhost:3000"
}

// redirectWithError ends a browser OAuth flow on the frontend callback page with an error
// code, instead of rendering a JSON body in the browser.
func (h *OAuthHandlers) redirectWithError(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, fmt.Sprintf("%s/auth/callback?error=%s", h.frontendBaseURL(), url.QueryEscape(code)), http.StatusFound)
}

// refuseSuspendedSignIn redirects with account_suspended when err is the identity refusal and
// reports whether it did.
func (h *OAuthHandlers) refuseSuspendedSignIn(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, db.ErrAccountSuspended) {
		return false
	}
	h.redirectWithError(w, r, OAuthErrorAccountSuspended)
	return true
}
