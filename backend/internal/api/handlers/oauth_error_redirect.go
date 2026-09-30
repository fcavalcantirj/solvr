package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Codes a browser OAuth flow ends with on the frontend's /auth/callback?error=… page. An error
// the provider reports (access_denied, bad_verification_code, invalid_grant, …) is passed on
// under the provider's own code.
const (
	OAuthErrorAccountSuspended    = "account_suspended"    // a banned or tombstoned identity
	OAuthErrorMissingCode         = "missing_code"         // the callback carried no authorization code
	OAuthErrorProviderUnavailable = "provider_unavailable" // GitHub or Google could not be reached or answered badly
	OAuthErrorLoginFailed         = "login_failed"         // the account or the session could not be set up
	OAuthErrorLoginUnavailable    = "login_unavailable"    // login codes are not configured on this server
	oauthErrorFromProvider        = "oauth_error"          // the provider refused without a code
)

// frontendBaseURL is where browser OAuth flows land (FE-022).
func (h *OAuthHandlers) frontendBaseURL() string {
	if h.config.FrontendURL != "" {
		return h.config.FrontendURL
	}
	return "http://localhost:3000"
}

// redirectWithError ends a browser OAuth flow on the frontend callback page with an error
// code, instead of rendering a JSON body in the browser. Every GitHub and Google callback
// error ends here (Felipe, 2026-09-30).
func (h *OAuthHandlers) redirectWithError(w http.ResponseWriter, r *http.Request, code string) {
	if code == "" {
		code = oauthErrorFromProvider
	}
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
