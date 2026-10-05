package handlers

import (
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// skill_fetched: the skill link of a copied sentence was fetched (SPEC.md 25.7).
//
// The sentence GET /v1/connect serves carries the visit's flow code on its skill link
// (https://solvr.dev/skill.md?f=<code>). The site's web server reports each fetch of that
// link here, beside the response it gives, with the code and the request's Sec-Fetch-Mode
// header. Everything about the step is decided by the API:
//
//   - flow_id is required and must be a flow code. The step is recorded whether or not
//     the flow is known: an agent that read GET /v1/connect itself has no earlier step.
//   - entry_surface is set here, never taken from the client: a person who opened the
//     link in a browser is a browser_visit, anything else is an agent's fetch.
//   - request_mode is read for that decision and never stored.
//   - nothing else the client sends is stored for this step.

// funnelRequestModeMax bounds request_mode. A Sec-Fetch-Mode value is one short word
// (navigate, cors, no-cors, same-origin, websocket).
const funnelRequestModeMax = 20

// skillFetchNavigate is the Sec-Fetch-Mode a browser sends when a person opens a link.
const skillFetchNavigate = "navigate"

// skillFetchSurface decides the entry surface of a skill fetch from how the skill was
// requested. Only a browser navigation is a person's visit; an agent's HTTP client
// sends no Sec-Fetch-Mode at all, and any other mode is a program's request.
func skillFetchSurface(requestMode string) string {
	if requestMode == skillFetchNavigate {
		return models.FunnelSurfaceBrowserVisit
	}
	return models.FunnelSurfaceAgentFetch
}

// ingestSkillFetched records one skill_fetched step reported to POST /v1/analytics/funnel.
func (h *FunnelHandler) ingestSkillFetched(w http.ResponseWriter, r *http.Request, req ingestFunnelRequest) {
	if !models.ValidFlowCode(req.FlowID) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"skill_fetched needs flow_id: the 8-character flow code the skill link carried (?f=)")
		return
	}
	if len(req.RequestMode) > funnelRequestModeMax {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "request_mode is too long")
		return
	}

	actorType, actorRef := funnelActor(r)
	if err := h.repo.RecordSkillFetched(r.Context(), req.FlowID, actorType, actorRef, skillFetchSurface(req.RequestMode)); err != nil {
		// As for every funnel step: a row that could not be written is a statistic
		// that is briefly short, never an error the reporter must handle.
		slog.Warn("failed to record skill_fetched funnel event", "error", err)
		writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": false})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": true})
}
