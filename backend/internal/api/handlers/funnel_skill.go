package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// skill_fetched: the skill link of a copied sentence was fetched (SPEC.md 25.7).
//
// The sentence GET /v1/connect serves carries the visit's flow code on its skill link
// (https://solvr.dev/skill.md?f=<code>). The site's web server reports each fetch of that
// link here, beside the response it gives, with the code and two headers of the request:
// Sec-Fetch-Mode and User-Agent. Everything about the step is decided by the API:
//
//   - flow_id is required and must be a flow code. The step is recorded whether or not
//     the flow is known: an agent that read GET /v1/connect itself has no earlier step.
//   - entry_surface is set here, never taken from the client, as one of three
//     (skillFetchSurface): a person who opened the link in a browser is a browser_visit,
//     a link preview or a crawler is a bot_fetch, anything else is an agent's fetch.
//   - request_mode and user_agent are read for that decision and never stored.
//   - nothing else the client sends is stored for this step.

// funnelRequestModeMax bounds request_mode. A Sec-Fetch-Mode value is one short word
// (navigate, cors, no-cors, same-origin, websocket).
const funnelRequestModeMax = 20

// funnelUserAgentMax bounds user_agent, in characters. The web server cuts the request's
// User-Agent to this before it reports it. A long crawler agent still names its bot
// inside the cut: Googlebot's smartphone agent is 200 characters and the name ends at its
// 162nd (funnel_skill_surface_test.go).
//
// Characters, not bytes: the web server hands on each non-ASCII byte of the header as one
// character, and JSON carries that character as two bytes, so its 200 characters can
// arrive as 400 bytes. A bound in bytes would refuse the report and lose the fetch.
const funnelUserAgentMax = 200

// skillFetchNavigate is the Sec-Fetch-Mode a browser sends when a person opens a link.
const skillFetchNavigate = "navigate"

// skillFetchBotTokens is the ONE list of what a link preview or a crawler calls itself in
// its User-Agent, spelled as its owner spells it. A user agent that contains any of them,
// in any letter case, is a bot's:
//
//   - a link preview: somebody pasted the sentence into Slack, WhatsApp, Discord, Telegram,
//     X, LinkedIn, Facebook, Skype or Teams, Reddit, Mastodon or an e-mail, and the app
//     fetched the link to show what it points at;
//   - a crawler: a search engine, an SEO tool or a model trainer that rendered /connect
//     and followed the link.
//
// Neither is an agent reading the skill, so neither may be counted as one.
//
// An agent's tool is deliberately NOT here and must never be added: ChatGPT-User,
// Claude-User, claude-code, curl, python-requests, node, Go-http-client, axios, undici,
// okhttp. Each is a person's assistant or an agent's HTTP client fetching the skill, which
// is exactly what agent_fetch counts, and so is a request that names no user agent at all.
// funnel_skill_surface_test.go holds both lists against each other.
var skillFetchBotTokens = []string{
	// Link previews.
	"Slackbot", "Slack-ImgProxy", "WhatsApp", "Discordbot", "TelegramBot", "Twitterbot", "LinkedInBot",
	"facebookexternalhit", "Facebot", "SkypeUriPreview", "Iframely", "Embedly", "redditbot", "Mastodon",
	// Crawlers.
	"Googlebot", "Google-InspectionTool", "bingbot", "DuckDuckBot", "YandexBot", "Baiduspider", "Applebot",
	"AhrefsBot", "SemrushBot", "MJ12bot", "PetalBot", "Bytespider", "GPTBot", "OAI-SearchBot", "ClaudeBot",
	"PerplexityBot", "CCBot",
}

// skillFetchBotNeedles is skillFetchBotTokens in lower case, made once from that list: a
// user agent is lowered and searched for each.
var skillFetchBotNeedles = func() []string {
	needles := make([]string, len(skillFetchBotTokens))
	for i, token := range skillFetchBotTokens {
		needles[i] = strings.ToLower(token)
	}
	return needles
}()

// isSkillFetchBot reports whether a User-Agent names a link preview or a crawler.
func isSkillFetchBot(userAgent string) bool {
	if userAgent == "" {
		return false
	}
	lowered := strings.ToLower(userAgent)
	for _, needle := range skillFetchBotNeedles {
		if strings.Contains(lowered, needle) {
			return true
		}
	}
	return false
}

// skillFetchSurface decides the entry surface of a skill fetch from how the skill was
// requested, in this order:
//
//  1. A browser navigation (Sec-Fetch-Mode: navigate) is a person's visit, whatever the
//     user agent says.
//  2. A link-preview or crawler user agent is a bot's fetch.
//  3. Anything else is an agent's fetch. An agent's HTTP client sends no Sec-Fetch-Mode,
//     and often no user agent either.
func skillFetchSurface(requestMode, userAgent string) string {
	switch {
	case requestMode == skillFetchNavigate:
		return models.FunnelSurfaceBrowserVisit
	case isSkillFetchBot(userAgent):
		return models.FunnelSurfaceBotFetch
	default:
		return models.FunnelSurfaceAgentFetch
	}
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
	if utf8.RuneCountInString(req.UserAgent) > funnelUserAgentMax {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"user_agent is too long: send at most the first 200 characters of the User-Agent header")
		return
	}

	actorType, actorRef := funnelActor(r)
	surface := skillFetchSurface(req.RequestMode, req.UserAgent)
	if err := h.repo.RecordSkillFetched(r.Context(), req.FlowID, actorType, actorRef, surface); err != nil {
		// As for every funnel step: a row that could not be written is a statistic
		// that is briefly short, never an error the reporter must handle.
		slog.Warn("failed to record skill_fetched funnel event", "error", err)
		writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": false})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"recorded": true})
}
