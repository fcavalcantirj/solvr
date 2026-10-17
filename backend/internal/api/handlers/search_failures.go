package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/requestid"
)

// statusClientClosedRequest is the status a search is recorded with when its client closed the
// request before the search finished (SPEC.md 5.4). No client ever reads it: it exists so the
// logs and api_request_events count the request as the client's (class 4), not as a server
// failure. The 5xx on /v1/search measured from 2026-10-03 had this signature (C1 spike).
const statusClientClosedRequest = 499

// searchFailureRecordTimeout bounds the write of one failed search.
const searchFailureRecordTimeout = 2 * time.Second

// invalidSearchTextParam returns the first search parameter whose value is not UTF-8 text or
// holds a NUL character, or "". Postgres refuses such text, so it is the client's error.
func invalidSearchTextParam(params url.Values) string {
	for _, name := range SearchParamNames() {
		for _, v := range params[name] {
			if !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
				return name
			}
		}
	}
	return ""
}

// searcherOf is who searched: the signed-in human, the agent, or nobody known.
func searcherOf(r *http.Request) (searcherType string, searcherID *string) {
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		return "human", &claims.UserID
	}
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		return "agent", &agent.ID
	}
	return "anonymous", nil
}

// sqlStateOf is the Postgres SQLSTATE behind err, or "".
func sqlStateOf(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// answerSearchFailure answers a search the repository could not complete. When the client has
// already closed the request, the cancelled search is the client's doing: 499, logged at info.
// Otherwise the server failed: 500, logged once with the request id, the code and the SQLSTATE.
func (h *SearchHandler) answerSearchFailure(w http.ResponseWriter, r *http.Request, start time.Time, query string, err error) {
	id := requestid.From(r.Context())
	if r.Context().Err() != nil {
		writeSearchError(w, statusClientClosedRequest, "CLIENT_CLOSED_REQUEST",
			"the client closed the request before the search finished")
		slog.Info("search abandoned by the client", "request_id", id, "error_code", "CLIENT_CLOSED_REQUEST",
			"duration_ms", time.Since(start).Milliseconds())
		h.recordFailure(r, start, query, statusClientClosedRequest, "CLIENT_CLOSED_REQUEST", "")
		return
	}
	state := sqlStateOf(err)
	writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "search failed")
	slog.Error("search failed", "request_id", id, "error_code", "INTERNAL_ERROR", "sqlstate", state, "error", err.Error())
	h.recordFailure(r, start, query, http.StatusInternalServerError, "INTERNAL_ERROR", state)
}

// recordFailure stores a search that did not answer 2xx, after the response, never delaying it
// (SPEC.md 22.8). A failure that cannot be stored is logged and dropped.
func (h *SearchHandler) recordFailure(r *http.Request, start time.Time, query string, status int, code, sqlstate string) {
	if h.failures == nil {
		return
	}
	searcherType, _ := searcherOf(r)
	f := &models.SearchFailure{
		RequestID:    requestid.From(r.Context()),
		Query:        query,
		Status:       status,
		ErrorCode:    code,
		SQLState:     sqlstate,
		SearcherType: searcherType,
		DurationMs:   int(time.Since(start).Milliseconds()),
		OccurredAt:   start,
	}
	// The write outlives the response, so it keeps the request's values (its id) but not its
	// cancellation.
	detached := context.WithoutCancel(r.Context())
	go func() {
		defer func() { recover() }() //nolint:errcheck // a statistic must never take the server down
		ctx, cancel := context.WithTimeout(detached, searchFailureRecordTimeout)
		defer cancel()
		if err := h.failures.RecordFailure(ctx, f); err != nil {
			slog.Warn("search failure record failed", "request_id", f.RequestID, "error", err)
		}
	}()
}
