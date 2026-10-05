package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The connection-funnel API.
//
//	POST /v1/analytics/funnel           the page reports a browser step, or the web
//	                                    server reports that it served the skill
//	GET  /v1/analytics/funnel/contract  the one documented event contract
//
// Only BROWSER steps and the web server's skill_fetched (funnel_skill.go) may be
// reported here: a client can announce that it opened a panel or copied a prompt,
// but it can never claim a room was created, a participant joined, or a two-way
// exchange happened — those are recorded from confirmed server actions elsewhere,
// so no client can fake them or suppress the measurement by blocking browser
// analytics. The endpoint stores no credential, no message body, no task text and
// no room title; the actor is classified from the request's own credential and
// reduced to a pseudonymous reference.

// funnelBoundedField caps every optional client string at the column bound so a
// malformed report is a clean 400, not a database constraint error.
const funnelBoundedField = 60

// FunnelHandler serves the connection-funnel ingest and contract endpoints.
type FunnelHandler struct {
	repo *db.FunnelEventRepository
	// sourceRooms / sourcePosts resolve a step's public source (SetSourceResolvers).
	// Optional: without them a reported source is dropped, the step still recorded.
	sourceRooms funnelRoomSource
	sourcePosts connectPostLookup
}

// NewFunnelHandler wires the handler to the funnel event store.
func NewFunnelHandler(repo *db.FunnelEventRepository) *FunnelHandler {
	return &FunnelHandler{repo: repo}
}

// ingestFunnelRequest is the browser's report of one funnel step.
type ingestFunnelRequest struct {
	FlowID             string `json:"flow_id"`
	Event              string `json:"event"`
	Preset             string `json:"preset"`
	Role               string `json:"role"`
	EntrySurface       string `json:"entry_surface"`
	InstructionVersion string `json:"instruction_version"`
	// Source names the public room (by slug) or post (by id) this step is attributed to.
	Source *funnelSourceRef `json:"source,omitempty"`
	// RequestMode and UserAgent belong to skill_fetched alone: the Sec-Fetch-Mode and the
	// User-Agent header of the request for the skill. The API reads them to set
	// entry_surface (funnel_skill.go) and never stores them. Any other step ignores both.
	RequestMode string `json:"request_mode"`
	UserAgent   string `json:"user_agent"`
}

// IngestBrowserEvent handles POST /v1/analytics/funnel. Public, optional auth:
// a logged-out visitor's steps are recorded as anonymous, a signed-in person's
// or an agent's as a pseudonymous reference.
func (h *FunnelHandler) IngestBrowserEvent(w http.ResponseWriter, r *http.Request) {
	var req ingestFunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	// A client may only report a browser step or the web server's. Naming a
	// server-only step — or a step that does not exist — is refused rather than
	// trusted. The refusal names the accepted steps from the contract itself.
	if !models.IsClientReportedFunnelEvent(req.Event) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"event must be one of "+strings.Join(models.ClientReportedFunnelEvents(), ", "))
		return
	}

	// skill_fetched has its own rules: the API decides everything about the step.
	if req.Event == models.FunnelSkillFetched {
		h.ingestSkillFetched(w, r, req)
		return
	}

	if len(req.FlowID) > 64 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "flow_id is too long")
		return
	}
	for _, field := range []string{req.Preset, req.Role, req.EntrySurface, req.InstructionVersion} {
		if len(field) > funnelBoundedField {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "a funnel attribute is too long")
			return
		}
	}

	if req.Source != nil && (!models.ValidFunnelSourceKind(req.Source.Kind) || len(req.Source.Ref) > funnelSourceRefMax) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"source must be {kind: room|post, ref: a room slug or post id}")
		return
	}

	actorType, actorRef := funnelActor(r)

	err := h.repo.RecordBrowserEvent(r.Context(), db.BrowserFunnelEvent{
		FlowID:             req.FlowID,
		EventName:          req.Event,
		ActorType:          actorType,
		ActorRef:           actorRef,
		Preset:             req.Preset,
		Role:               req.Role,
		EntrySurface:       req.EntrySurface,
		InstructionVersion: req.InstructionVersion,
		Source:             h.resolveSource(r.Context(), req.Source),
	})
	if err != nil {
		// A funnel row that could not be written is a statistic that is briefly
		// short, never a failed user action. Report a soft failure, not a 500 the
		// page must handle.
		slog.Warn("failed to record browser funnel event", "error", err, "event", req.Event)
		writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": false})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": true})
}

// GetContract handles GET /v1/analytics/funnel/contract. It publishes the single
// event contract — names, source channels, meanings and attributes — plus the
// current instruction version, so the browser emitter and any operator reader
// share one source of truth. Public: it is documentation, not data.
func (h *FunnelHandler) GetContract(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"instruction_version": ConnectInstructionVersion,
		"events":              models.FunnelEventContract(),
	})
}

// funnelActor classifies the caller from its credential and reduces it to a
// pseudonymous reference. The one direction that may never fail is calling an
// unidentified caller a person: an unrecognised request is anonymous.
func funnelActor(r *http.Request) (actorType, actorRef string) {
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		return models.FunnelActorHuman, db.PseudonymizeActor(claims.UserID)
	}
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		return models.FunnelActorAgent, db.PseudonymizeActor(agent.ID)
	}
	return models.FunnelActorAnonymous, ""
}
