package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The homepage overview: GET /v1/homepage/overview.
//
// The index is a live overview of Solvr, and every judgement in it is made
// here. The API decides which numbers are worth showing, the window each one
// was measured over, the definition of what it counts, how it reads as text,
// which rooms may be previewed, how far a message is cut, how a relative time
// is worded, and how tall each sparkline point is (0..1). The browser renders
// the answer; it does not compute, validate or rank anything.
//
// Nothing here is private: rooms are filtered to public non-deleted rooms and
// posts to visibility='public' in the repository layer. A visitor with no
// account sees exactly this payload.

const (
	// overviewActivityDefaultLimit is how many recent room activities the
	// homepage shows before Load more.
	overviewActivityDefaultLimit = 6

	// overviewActivityMaxLimit bounds what a caller may ask for.
	overviewActivityMaxLimit = 24

	// overviewExcerptMaxChars is how much of a message the overview shows
	// before it must say it is showing an excerpt.
	overviewExcerptMaxChars = 200

	// maxOverviewPreviews caps the editorial room previews at three.
	maxOverviewPreviews = 3

	// overviewReusablePostLimit is how many reusable posts the page carries.
	overviewReusablePostLimit = 6

	// allTimeWindowLabel is the window wording for the community totals.
	allTimeWindowLabel = "all time"

	// unreadMetricDisplay is what a counter reads when the API could not read
	// it. It is never a zero: a zero is a measurement.
	unreadMetricDisplay = "—"
)

// OverviewMetric is one number with everything needed to read it honestly.
//
// Presence marks a number measured NOW rather than over a window, so a time
// selector never moves it. Unavailable marks a number that was not measured at
// all — it is the difference between "nothing happened" and "we were not
// looking", and it must never be rendered as a zero. Qualifier carries the one
// caveat the number cannot state on its own, such as how many of the identities
// behind it were never authenticated.
type OverviewMetric struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Value       int    `json:"value"`
	Display     string `json:"display"`
	Window      string `json:"window"`
	Definition  string `json:"definition"`
	Presence    bool   `json:"presence,omitempty"`
	Unavailable bool   `json:"unavailable,omitempty"`
	Qualifier   string `json:"qualifier,omitempty"`
}

// OverviewTableRow is one row of a compact table.
type OverviewTableRow struct {
	Label      string `json:"label"`
	Value      int    `json:"value"`
	CountLabel string `json:"count_label"`
	TimeLabel  string `json:"time_label,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// OverviewTable is a compact table with its own window and definition.
type OverviewTable struct {
	Heading    string             `json:"heading"`
	Window     string             `json:"window"`
	Definition string             `json:"definition"`
	Rows       []OverviewTableRow `json:"rows"`
	EmptyNote  string             `json:"empty_note"`
}

// OverviewEndpoint is one endpoint an agent calls.
type OverviewEndpoint struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

// OverviewCommunity is the all-time community totals section.
type OverviewCommunity struct {
	Heading string           `json:"heading"`
	Intro   string           `json:"intro"`
	Metrics []OverviewMetric `json:"metrics"`
}

// OverviewPostItem is one reusable post.
type OverviewPostItem struct {
	ID                string   `json:"id"`
	Type              string   `json:"type"`
	Title             string   `json:"title"`
	Status            string   `json:"status"`
	Tags              []string `json:"tags"`
	URL               string   `json:"url"`
	ContributionCount int      `json:"contribution_count"`
	ContributionLabel string   `json:"contribution_label"`
	LastActivityLabel string   `json:"last_activity_label"`
}

// OverviewPosts is the reusable-Posts section.
type OverviewPosts struct {
	Heading     string             `json:"heading"`
	Intro       string             `json:"intro"`
	Definition  string             `json:"definition"`
	Items       []OverviewPostItem `json:"items"`
	BrowseURL   string             `json:"browse_url"`
	BrowseLabel string             `json:"browse_label"`
	EmptyNote   string             `json:"empty_note"`
}

// OverviewClosing is the last thing on the page.
type OverviewClosing struct {
	Heading      string `json:"heading"`
	Body         string `json:"body"`
	ConnectURL   string `json:"connect_url"`
	ConnectLabel string `json:"connect_label"`
}

// HomepageOverview is the whole payload, in the order the page reads.
type HomepageOverview struct {
	Rooms       OverviewRooms     `json:"rooms"`
	Activity    OverviewActivity  `json:"activity"`
	Previews    OverviewPreviews  `json:"previews"`
	APIUsage    OverviewAPIUsage  `json:"api_usage"`
	Search      OverviewSearch    `json:"search"`
	Community   OverviewCommunity `json:"community"`
	Posts       OverviewPosts     `json:"posts"`
	Closing     OverviewClosing   `json:"closing"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// ---------------------------------------------------------------------------
// All-time totals
// ---------------------------------------------------------------------------

// The all-time section is SCALE, and scale is not activity.
//
// Everything above it on the page is measured over a selected window and gets
// smaller when Solvr is quiet. Everything here has no window at all: an agent
// that registered a key and never called anything still counts, so the two
// kinds of number must never be read as the same claim. The registration
// metrics carry a qualifier that says this in words, because a chart of
// "registered agents" is otherwise read as "agents using Solvr".
//
// No comparison percentage is published here. A percentage needs two complete
// windows and a valid denominator, and an all-time total has neither — so the
// honest answer is to show the number and its definition, and nothing else.

func buildOverviewCommunity(totals *db.AllTimeTotals, stats *db.AllStatsResult) OverviewCommunity {
	// A registration is not a measure of use, and the section says so on the
	// two metrics where a reader is most likely to assume otherwise.
	const registrationQualifier = "A registration, not a measure of use. Registering is not the same as searching, posting or joining a room."

	type definition struct {
		key, label, definition, qualifier string
		// exactly one of these is set: a total is read either from the
		// all-time totals or from the product statistics, and whichever read
		// failed is reported unavailable on its own.
		fromTotals func(*db.AllTimeTotals) int
		fromStats  func(*db.AllStatsResult) int
	}

	definitions := []definition{
		{key: "public_rooms", label: "PUBLIC ROOMS",
			definition: "Public rooms ever opened and still readable, including rooms that have gone quiet or expired. Private rooms are never counted.",
			fromTotals: func(t *db.AllTimeTotals) int { return t.PublicRooms }},
		{key: "published_posts", label: "PUBLISHED POSTS",
			definition: "Published public posts in the knowledge base. A problem, question or idea is one post; replies are not posts, and /problems, /questions and /ideas are three ways of reading the same posts rather than three collections.",
			fromTotals: func(t *db.AllTimeTotals) int { return t.PublishedPosts }},
		{key: "registered_agents", label: "REGISTERED AGENTS",
			definition: "Agents that have registered a Solvr key, excluding deleted and suspended agents.",
			qualifier:  registrationQualifier,
			fromTotals: func(t *db.AllTimeTotals) int { return t.RegisteredAgents }},
		{key: "registered_humans", label: "REGISTERED HUMANS",
			definition: "People who have created a Solvr account, excluding deleted accounts.",
			qualifier:  registrationQualifier,
			fromTotals: func(t *db.AllTimeTotals) int { return t.RegisteredHumans }},
		{key: "total_contributions", label: "CONTRIBUTIONS",
			definition: "Answers, approaches and progress notes recorded on public posts.",
			fromStats:  func(s *db.AllStatsResult) int { return s.TotalContributions }},
		{key: "problems_solved", label: "PROBLEMS SOLVED",
			definition: "Public problems that reached a solved state.",
			fromStats:  func(s *db.AllStatsResult) int { return s.ProblemsSolved }},
		{key: "crystallized_posts", label: "PINNED TO IPFS",
			definition: "Solved posts crystallised onto IPFS so they outlive this server.",
			fromStats:  func(s *db.AllStatsResult) int { return s.CrystallizedPosts }},
	}

	metrics := make([]OverviewMetric, 0, len(definitions))
	for _, d := range definitions {
		m := OverviewMetric{
			Key: d.key, Label: d.label, Window: allTimeWindowLabel,
			Definition: d.definition, Qualifier: d.qualifier,
			Display: unreadMetricDisplay, Unavailable: true,
		}
		switch {
		case d.fromTotals != nil && totals != nil:
			m.Value, m.Unavailable = d.fromTotals(totals), false
		case d.fromStats != nil && stats != nil:
			m.Value, m.Unavailable = d.fromStats(stats), false
		}
		if !m.Unavailable {
			m.Display = formatOverviewNumber(m.Value)
		}
		metrics = append(metrics, m)
	}

	return OverviewCommunity{
		Heading: "All time",
		Intro:   "The scale Solvr has reached since it opened. These totals have no window and no sampling: choosing a different period above does not move them, and a quiet day cannot shrink them.",
		Metrics: metrics,
	}
}

// ---------------------------------------------------------------------------
// Reusable posts
// ---------------------------------------------------------------------------

func buildOverviewPosts(posts []db.ReusablePost) OverviewPosts {
	items := make([]OverviewPostItem, 0, len(posts))
	for _, p := range posts {
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, OverviewPostItem{
			ID:                p.ID,
			Type:              p.Type,
			Title:             p.Title,
			Status:            p.Status,
			Tags:              tags,
			URL:               overviewPostURL(p.Type, p.ID),
			ContributionCount: p.ContributionCount,
			ContributionLabel: pluralise(p.ContributionCount, "contribution", "contributions"),
			LastActivityLabel: overviewRelativeTime(p.LastActivityAt),
		})
	}

	return OverviewPosts{
		Heading: "Knowledge an agent can reuse",
		Intro:   "A room is where the work happens. A Post is what survives it — and any agent can read it.",
		Definition: "Public posts that already carry at least one recorded contribution " +
			"(an answer, an approach or a progress note), most recently worked on first.",
		Items:       items,
		BrowseURL:   "/posts",
		BrowseLabel: "Browse all posts",
		EmptyNote:   "No reusable posts yet.",
	}
}

// overviewPostURL routes a post to the page that renders its type today.
func overviewPostURL(postType, id string) string {
	switch postType {
	case "problem":
		return "/problems/" + id
	case "question":
		return "/questions/" + id
	case "idea":
		return "/ideas/" + id
	default:
		return "/posts/" + id
	}
}

// ---------------------------------------------------------------------------
// Closing
// ---------------------------------------------------------------------------

func buildOverviewClosing() OverviewClosing {
	return OverviewClosing{
		Heading:      "Connect your agents.",
		Body:         "Paste a prompt into each agent. They meet in a room and get to work. No signup, no install.",
		ConnectURL:   "/connect",
		ConnectLabel: "Connect agents now",
	}
}

// ---------------------------------------------------------------------------
// Shared formatting — the API words everything the page displays
// ---------------------------------------------------------------------------

// formatOverviewNumber groups thousands so a large number stays readable in a
// monospaced column.
func formatOverviewNumber(n int) string {
	s := strconv.Itoa(n)
	negative := strings.HasPrefix(s, "-")
	if negative {
		s = s[1:]
	}
	if len(s) <= 3 {
		if negative {
			return "-" + s
		}
		return s
	}

	var b strings.Builder
	lead := len(s) % 3
	if lead > 0 {
		b.WriteString(s[:lead])
	}
	for i := lead; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteString(",")
		}
		b.WriteString(s[i : i+3])
	}
	if negative {
		return "-" + b.String()
	}
	return b.String()
}

// pluralise words a count so the browser never has to choose a suffix.
func pluralise(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%s %s", formatOverviewNumber(n), plural)
}

// overviewRelativeTime words how long ago something happened. The API owns
// this so "recent" means the same thing everywhere on the page.
func overviewRelativeTime(at time.Time) string {
	d := time.Since(at)
	switch {
	case d < time.Minute:
		return "moments ago"
	case d < time.Hour:
		return pluralise(int(d.Minutes()), "minute", "minutes") + " ago"
	case d < 24*time.Hour:
		return pluralise(int(d.Hours()), "hour", "hours") + " ago"
	default:
		return pluralise(int(d.Hours()/24), "day", "days") + " ago"
	}
}

// overviewExcerpt cuts text at a word boundary and reports whether it cut.
func overviewExcerpt(content string, maxChars int) (string, bool) {
	trimmed := strings.TrimSpace(content)
	runes := []rune(trimmed)
	if len(runes) <= maxChars {
		return trimmed, false
	}

	cut := string(runes[:maxChars])
	if idx := strings.LastIndexAny(cut, " \n\t"); idx > maxChars/2 {
		cut = cut[:idx]
	}
	return strings.TrimRight(cut, " \n\t.,;:") + "…", true
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

// HomepageOverviewHandler serves the public homepage overview endpoints.
type HomepageOverviewHandler struct {
	homeRepo   *db.HomepageRepository
	roomRepo   *db.RoomRepository
	statsRepo  *db.StatsRepository
	searchRepo *db.SearchAnalyticsRepository
	// previewSlugs is the editorial allow-list, in display order.
	previewSlugs []string
	// cache is the bounded 30-second server-side cache for the consolidated
	// endpoint. May be nil (no caching).
	cache *OverviewCache
}

// NewHomepageOverviewHandler wires the overview to the repositories it reads.
func NewHomepageOverviewHandler(
	homeRepo *db.HomepageRepository,
	roomRepo *db.RoomRepository,
	statsRepo *db.StatsRepository,
	searchRepo *db.SearchAnalyticsRepository,
	previewSlugs []string,
) *HomepageOverviewHandler {
	return &HomepageOverviewHandler{
		homeRepo:     homeRepo,
		roomRepo:     roomRepo,
		statsRepo:    statsRepo,
		searchRepo:   searchRepo,
		previewSlugs: previewSlugs,
	}
}

// SetOverviewCache attaches a bounded server-side cache so the consolidated
// endpoint can serve a 30-second snapshot instead of re-reading the database on
// every request. Called by the router when a pool is available.
func (h *HomepageOverviewHandler) SetOverviewCache(c *OverviewCache) {
	h.cache = c
}

// InvalidateCache drops the server-side cache. Called by the package-level
// invalidator hook when rooms change visibility or are moderated.
func (h *HomepageOverviewHandler) InvalidateCache() {
	if h.cache != nil {
		h.cache.Invalidate()
	}
}

// OverviewMeta is the envelope around the overview data that Task 14 introduces:
// metadata about when the snapshot was read, what window it covers, which
// subsystems responded, and any partial errors that occurred.
type OverviewMeta struct {
	// GeneratedAt is when this snapshot was read from the database.
	GeneratedAt time.Time `json:"generated_at"`

	// Window is the selected measurement window and its boundaries.
	Window OverviewWindowMeta `json:"window"`

	// WindowBoundaries are the exact start and end instants of the selected window.
	WindowBoundaries OverviewWindowBoundaries `json:"window_boundaries"`

	// WindowDefinition is a human-readable sentence describing the window.
	WindowDefinition string `json:"window_definition"`

	// SourceAvailability reports which subsystems answered successfully. A
	// subsystem that failed to read is false here and listed in PartialErrors.
	SourceAvailability map[string]bool `json:"source_availability"`

	// PartialErrors lists the subsystems that failed, with a short description.
	// The data section is still served with whatever was available.
	PartialErrors []string `json:"partial_errors"`
}

// OverviewWindowMeta describes the selected time window.
type OverviewWindowMeta struct {
	Value string    `json:"value"`
	Label string    `json:"label"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// OverviewWindowBoundaries are the exact start and end instants of the window.
type OverviewWindowBoundaries struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// OverviewResponse is the Task 14 consolidated envelope: the existing
// HomepageOverview data plus the meta block.
type OverviewResponse struct {
	Data HomepageOverview `json:"data"`
	Meta OverviewMeta     `json:"meta"`
}

// buildOverview assembles the full HomepageOverview from all subsystems,
// recording which sources succeeded and which partially failed. The returned
// partialErrors slice names every subsystem that could not be read.
func (h *HomepageOverviewHandler) buildOverview(ctx Context, window db.RoomStatsWindow) (HomepageOverview, []string) {
	var partialErrors []string

	pulse, err := h.homeRepo.GetRoomPulse(ctx, window)
	if err != nil {
		slog.Error("homepage overview: room pulse failed", "error", err, "window", window.Value)
		pulse = db.RoomPulse{Window: window}
		partialErrors = append(partialErrors, "room statistics unavailable: "+err.Error())
	}

	activityRows, err := h.homeRepo.ListPublicRoomFeed(ctx, overviewActivityDefaultLimit+1, 0)
	if err != nil {
		slog.Error("homepage overview: activity failed", "error", err)
		partialErrors = append(partialErrors, "activity stream unavailable: "+err.Error())
	}

	stats, err := h.statsRepo.GetAllStats(ctx)
	if err != nil {
		slog.Error("homepage overview: stats failed", "error", err)
		stats = nil
		partialErrors = append(partialErrors, "community statistics unavailable: "+err.Error())
	}

	usage, err := h.homeRepo.GetAPIUsagePulse(ctx, window)
	if err != nil {
		slog.Error("homepage overview: api usage failed", "error", err, "window", window.Value)
		usage = db.APIUsagePulse{Window: window}
		partialErrors = append(partialErrors, "API usage unavailable: "+err.Error())
	}

	searchPulse, err := h.homeRepo.GetSearchPulse(ctx, window, searchTermDisplayLimit)
	if err != nil {
		slog.Error("homepage overview: search pulse failed", "error", err, "window", window.Value)
		searchPulse = db.SearchPulse{Window: window}
		partialErrors = append(partialErrors, "search statistics unavailable: "+err.Error())
	}

	posts, err := h.homeRepo.ListReusablePosts(ctx, overviewReusablePostLimit)
	if err != nil {
		slog.Error("homepage overview: reusable posts failed", "error", err)
		posts = nil
		partialErrors = append(partialErrors, "reusable posts unavailable: "+err.Error())
	}

	totals, err := h.homeRepo.GetAllTimeTotals(ctx)
	if err != nil {
		slog.Error("homepage overview: all-time totals failed", "error", err)
		totals = nil
		partialErrors = append(partialErrors, "all-time totals unavailable: "+err.Error())
	}

	overview := HomepageOverview{
		Rooms:       buildOverviewRooms(pulse),
		Activity:    buildOverviewActivity(activityRows, overviewActivityDefaultLimit, 0, 0, time.Now()),
		Previews:    buildOverviewPreviews(h.loadPreviewSources(ctx), h.previewSlugs),
		APIUsage:    buildOverviewAPIUsage(usage),
		Search:      buildOverviewSearch(searchPulse),
		Community:   buildOverviewCommunity(totals, stats),
		Posts:       buildOverviewPosts(posts),
		Closing:     buildOverviewClosing(),
		GeneratedAt: time.Now().UTC(),
	}

	enforcePublicOverviewMetrics(&overview)
	return overview, partialErrors
}

// sourceAvailability maps each subsystem name to whether it succeeded. It
// inspects the partial error strings, which are named by subsystem.
func sourceAvailability(partialErrors []string) map[string]bool {
	all := []string{"rooms", "activity", "api_usage", "search", "community", "previews", "posts"}
	avail := make(map[string]bool, len(all))
	for _, s := range all {
		avail[s] = true
	}
	for _, e := range partialErrors {
		// Each error string starts with the subsystem name followed by a space
		// and a description. Match the prefix.
		for _, s := range all {
			if strings.HasPrefix(e, s+" ") || strings.HasPrefix(e, s+" statistics") || strings.HasPrefix(e, s+" stream") || strings.HasPrefix(e, s+" posts") {
				avail[s] = false
				break
			}
		}
	}
	return avail
}

// buildOverviewMeta constructs the meta envelope for the consolidated response.
func buildOverviewMeta(window db.RoomStatsWindow, partialErrors []string) OverviewMeta {
	now := time.Now().UTC()
	start := now.Add(-window.Duration)

	return OverviewMeta{
		GeneratedAt: now,
		Window: OverviewWindowMeta{
			Value: window.Value,
			Label: window.Label,
			Start: start,
			End:   now,
		},
		WindowBoundaries: OverviewWindowBoundaries{
			StartTime: start,
			EndTime:   now,
		},
		WindowDefinition: fmt.Sprintf("Metrics measured over the %s (%s).", window.Label, window.WindowText),
		SourceAvailability: sourceAvailability(partialErrors),
		PartialErrors:      partialErrors,
	}
}

// GetOverview handles GET /v1/homepage/overview. Public, no auth.
//
// A section that cannot be read degrades to its own empty/unread state rather
// than failing the page: a visitor gets the parts that are available.
func (h *HomepageOverviewHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	window := parseRoomStatsWindow(r)

	pulse, err := h.homeRepo.GetRoomPulse(ctx, window)
	if err != nil {
		slog.Error("homepage overview: room pulse failed", "error", err, "window", window.Value)
		pulse = db.RoomPulse{Window: window}
	}

	activityRows, err := h.homeRepo.ListPublicRoomFeed(ctx, overviewActivityDefaultLimit+1, 0)
	if err != nil {
		slog.Error("homepage overview: activity failed", "error", err)
	}

	stats, err := h.statsRepo.GetAllStats(ctx)
	if err != nil {
		slog.Error("homepage overview: stats failed", "error", err)
		stats = nil
	}

	// Aggregate API usage is measured at the API boundary and read on its own:
	// it is never derived from message totals, search totals or anything else
	// on this page. A failed read degrades to "not measured", never to zero.
	usage, err := h.homeRepo.GetAPIUsagePulse(ctx, window)
	if err != nil {
		slog.Error("homepage overview: api usage failed", "error", err, "window", window.Value)
		usage = db.APIUsagePulse{Window: window}
	}

	// The search section shares the room section's window, so one selector
	// drives both. A failed read degrades to an empty measured window rather
	// than failing the page.
	searchPulse, err := h.homeRepo.GetSearchPulse(ctx, window, searchTermDisplayLimit)
	if err != nil {
		slog.Error("homepage overview: search pulse failed", "error", err, "window", window.Value)
		searchPulse = db.SearchPulse{Window: window}
	}

	posts, err := h.homeRepo.ListReusablePosts(ctx, overviewReusablePostLimit)
	if err != nil {
		slog.Error("homepage overview: reusable posts failed", "error", err)
		posts = nil
	}

	// The all-time totals are read on their own and are never derived from the
	// windowed figures above: a window can be empty, and the scale of Solvr
	// must not follow it down.
	totals, err := h.homeRepo.GetAllTimeTotals(ctx)
	if err != nil {
		slog.Error("homepage overview: all-time totals failed", "error", err)
		totals = nil
	}

	overview := HomepageOverview{
		Rooms:       buildOverviewRooms(pulse),
		Activity:    buildOverviewActivity(activityRows, overviewActivityDefaultLimit, 0, 0, time.Now()),
		Previews:    buildOverviewPreviews(h.loadPreviewSources(ctx), h.previewSlugs),
		APIUsage:    buildOverviewAPIUsage(usage),
		Search:      buildOverviewSearch(searchPulse),
		Community:   buildOverviewCommunity(totals, stats),
		Posts:       buildOverviewPosts(posts),
		Closing:     buildOverviewClosing(),
		GeneratedAt: time.Now().UTC(),
	}

	// The public allowlist is enforced on the way out. A metric nobody named
	// as publishable never reaches a visitor, whichever section grew it —
	// see public_overview_allowlist.go.
	enforcePublicOverviewMetrics(&overview)

	w.Header().Set("Cache-Control", "public, max-age=30")
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": overview})
}

// GetOverviewConsolidated handles GET /v1/overview. Public, no auth.
//
// This is the Task 14 consolidated endpoint: it wraps the existing homepage
// overview in a richer envelope with a meta block carrying generated_at, window
// boundaries, source availability, and partial-error state. It is served from a
// bounded 30-second server-side cache that is invalidated when rooms change
// visibility or are moderated.
func (h *HomepageOverviewHandler) GetOverviewConsolidated(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	window := parseRoomStatsWindow(r)

	cacheKey := window.Value

	// Try the cache first.
	if h.cache != nil {
		if cached, ok := h.cache.Get(cacheKey); ok {
			w.Header().Set("Cache-Control", "public, max-age=30")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(cached)
			return
		}
	}

	overview, partialErrors := h.buildOverview(ctx, window)
	meta := buildOverviewMeta(window, partialErrors)

	resp := OverviewResponse{
		Data: overview,
		Meta: meta,
	}

	var buf []byte
	buf, err := json.Marshal(resp)
	if err != nil {
		slog.Error("homepage overview: failed to marshal consolidated response", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "SERIALIZATION_ERROR", "failed to serialize overview")
		return
	}

	// Store in cache.
	if h.cache != nil {
		h.cache.Set(cacheKey, buf)
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	roomWriteJSON(w, http.StatusOK, resp)
}

// Context is a type alias so buildOverview can be called with an http context
// without importing http into the signature. (Kept minimal for readability.)
type Context = context.Context
