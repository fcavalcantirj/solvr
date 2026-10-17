package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/traffic"
)

// POST /v1/analytics/pageview: the site's web server reports a page it served (SPEC.md 27.6).
//
// The report carries the page's path and what the request for it said about itself: its
// User-Agent, Sec-Fetch-Mode, Sec-Fetch-Dest and Sec-CH-UA, whether it named a language, and
// whether it was a client-side navigation or a prefetch. The API decides everything: a prefetch
// is not a view and is dropped; the path is reduced to the kind of page it is
// (traffic.ContentGroupForPath) and the view is classified (traffic.Classify). Only the kind,
// the class and the reason are stored: no path, address, user agent or cookie, and no consent
// is needed (SPEC.md 27.7). A row that could not be written is a statistic briefly short,
// never an error the web server must handle.

// pageViewPathMax bounds path, in bytes: a page path is short, and a longer one is not a page.
const pageViewPathMax = 512

// pageViewHeaderMax bounds a reported header value in characters, as the skill fetch does
// (funnelUserAgentMax): the web server cuts User-Agent and Sec-CH-UA to 200.
const pageViewHeaderMax = 200

// pageViewTokenMax bounds Sec-Fetch-Mode and Sec-Fetch-Dest, one short word each.
const pageViewTokenMax = 20

// PageViewRecorder stores one classified page view (db.PageViewRepository).
type PageViewRecorder interface {
	Record(ctx context.Context, contentGroup string, label db.TrafficLabel) error
}

// PageViewHandler serves POST /v1/analytics/pageview.
type PageViewHandler struct {
	repo PageViewRecorder
}

// NewPageViewHandler wires the handler to the page view store.
func NewPageViewHandler(repo PageViewRecorder) *PageViewHandler {
	return &PageViewHandler{repo: repo}
}

// pageViewReport is the web server's report of one page it served.
type pageViewReport struct {
	Path           string `json:"path"`
	UserAgent      string `json:"user_agent"`
	RequestMode    string `json:"request_mode"`
	FetchDest      string `json:"fetch_dest"`
	ClientHints    string `json:"sec_ch_ua"`
	AcceptLanguage bool   `json:"accept_language"`
	RSC            bool   `json:"rsc"`
	Prefetch       bool   `json:"prefetch"`
}

// Record handles POST /v1/analytics/pageview. Public: the web server reports with no credential.
func (h *PageViewHandler) Record(w http.ResponseWriter, r *http.Request) {
	var report pageViewReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if msg := report.problem(); msg != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", msg)
		return
	}
	if report.Prefetch {
		writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": false})
		return
	}

	label := db.LabelFor(traffic.Signals{
		Surface:      traffic.SurfacePage,
		UserAgent:    report.UserAgent,
		ClientHints:  report.ClientHints,
		FetchMode:    report.RequestMode,
		HeadersKnown: true,
	})
	if err := h.repo.Record(r.Context(), traffic.ContentGroupForPath(report.Path), label); err != nil {
		slog.Warn("failed to record a page view", "error", err)
		writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": false})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": true})
}

// problem names what is wrong with a report, or is empty.
func (p *pageViewReport) problem() string {
	switch {
	case !strings.HasPrefix(p.Path, "/") || len(p.Path) > pageViewPathMax:
		return "path must be the page's path: it starts with / and has at most 512 bytes"
	case utf8.RuneCountInString(p.UserAgent) > pageViewHeaderMax:
		return "user_agent is too long: send at most the first 200 characters of the User-Agent header"
	case utf8.RuneCountInString(p.ClientHints) > pageViewHeaderMax:
		return "sec_ch_ua is too long: send at most the first 200 characters of the Sec-CH-UA header"
	case len(p.RequestMode) > pageViewTokenMax:
		return "request_mode is too long"
	case len(p.FetchDest) > pageViewTokenMax:
		return "fetch_dest is too long"
	}
	return ""
}
