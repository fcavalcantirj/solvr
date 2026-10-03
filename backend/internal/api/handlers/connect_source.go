package handlers

import (
	"context"
	"regexp"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// "Try this workflow" — a public room seeds a FRESH start flow (idx 88).
//
// GET /v1/connect?from_room=<slug> reads the room's public task structure through
// FindPublicRoomTemplate, which answers only for a public, existing room. The
// structure's text is scrubbed here before it reaches the contract: credential-shaped
// strings and links to rooms that are not public are removed. The prompt then carries
// the source slug into the create-room body, so the new room records where it came
// from, and tells the agent the room starts fresh — no members, credentials, approvals,
// reviews or results of the source room carry over.

// connectRoomSourceLookup is the slice of the room repository a source-room start needs.
type connectRoomSourceLookup interface {
	FindPublicRoomTemplate(ctx context.Context, slug string) (*models.RoomTemplate, error)
	PublicRoomSlugs(ctx context.Context, slugs []string) (map[string]bool, error)
}

// SetRoomSourceLookup enables seeding the start flow from a public room (?from_room=).
// Optional: with no lookup wired, ?from_room= is ignored and the ordinary contract served.
func (h *ConnectHandler) SetRoomSourceLookup(rooms connectRoomSourceLookup) {
	h.roomSources = rooms
}

// connectSourceRoomFresh is what every prompt seeded from a room says about the new room.
const connectSourceRoomFresh = "This room starts fresh: no earlier members, credentials, approvals, reviews or results carry over."

// resolveRoomSource returns the source and the scrubbed task it seeds, only for a public
// room. Anything else returns nil so nothing about a protected room leaks.
func (h *ConnectHandler) resolveRoomSource(ctx context.Context, slug string) (*ConnectSource, string) {
	if h.roomSources == nil || slug == "" {
		return nil, ""
	}
	tmpl, err := h.roomSources.FindPublicRoomTemplate(ctx, slug)
	if err != nil || tmpl == nil {
		return nil, ""
	}
	url := "/rooms/" + tmpl.Slug
	src := &ConnectSource{
		Kind:     "room",
		RoomSlug: tmpl.Slug,
		Title:    publicTemplateText(ctx, tmpl.DisplayName, h.roomSources),
		URL:      url,
		Detail:   "This collaboration reuses the public task of a Solvr room. Only the task travels; " + connectSourceRoomFresh,
	}

	task := truncateRunes(publicTemplateText(ctx, strings.TrimSpace(tmpl.InitialTask), h.roomSources), ConnectTaskMaxChars)
	if task == "" {
		task = "Run your own version of the work in the public Solvr room \"" + src.Title + "\" (" +
			connectAppBaseURL + url + "). Read it first, then plan the work."
	}
	return src, task
}

// promptSourceSection names a room source in a starter prompt, right under the task:
// where the task came from and that nothing else of that room carries over. Empty for
// every other flow.
func promptSourceSection(sel ConnectSelection) string {
	if sel.SourceRoom == "" {
		return ""
	}
	return "\n\nSOURCE\n" +
		"This task reuses the public task structure of the Solvr room " + connectAppBaseURL + "/rooms/" + sel.SourceRoom + ".\n" +
		connectSourceRoomFresh
}

var (
	// templateQuerySecret is a credential carried in a query string; the parameter name
	// stays so the instruction still reads, the value goes.
	templateQuerySecret = regexp.MustCompile(`(?i)([?&](?:token|access_token|room_token|api_key|apikey|key)=)[^&\s#"']+`)
	templateBearer      = regexp.MustCompile(`(?i)(bearer\s+)[^\s"']+`)
	templateJWT         = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	// templateSolvrKey matches agent keys (solvr_…), user keys (solvr_sk_…) and room
	// tokens (solvr_rt_…). The 16-character floor keeps words like solvr_dev intact.
	templateSolvrKey = regexp.MustCompile(`\bsolvr_(?:rt_|sk_)?[A-Za-z0-9_-]{16,}`)
	// templateRoomLink is a link to a Solvr room, page or transport URL alike.
	templateRoomLink = regexp.MustCompile(`(?:https?://[a-z.]*solvr\.dev)?(?:/v1)?/(?:rooms|r)/([a-z0-9][a-z0-9-]*)`)
)

const (
	templateRedacted    = "[redacted]"
	templatePrivateRoom = "[private room]"
)

// publicTemplateText removes what must never travel from a source room into a new one:
// credentials (query-string secrets, bearer values, JWTs, Solvr keys and room tokens) and
// links to rooms that are not public. A failed room lookup removes every room link: when
// visibility cannot be proven, the reference is treated as private.
func publicTemplateText(ctx context.Context, text string, rooms connectRoomSourceLookup) string {
	if text == "" {
		return ""
	}
	out := templateQuerySecret.ReplaceAllString(text, "${1}"+templateRedacted)
	out = templateBearer.ReplaceAllString(out, "${1}"+templateRedacted)
	out = templateJWT.ReplaceAllString(out, templateRedacted)
	out = templateSolvrKey.ReplaceAllString(out, templateRedacted)

	matches := templateRoomLink.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return out
	}
	slugs := make([]string, 0, len(matches))
	for _, m := range matches {
		slugs = append(slugs, m[1])
	}
	public := map[string]bool{}
	if rooms != nil {
		if found, err := rooms.PublicRoomSlugs(ctx, slugs); err == nil {
			public = found
		}
	}
	return templateRoomLink.ReplaceAllStringFunc(out, func(link string) string {
		if public[templateRoomLink.FindStringSubmatch(link)[1]] {
			return link
		}
		return templatePrivateRoom
	})
}

// truncateRunes cuts s to at most max runes, marking the cut.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
