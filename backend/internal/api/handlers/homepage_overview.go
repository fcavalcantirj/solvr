package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
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

	// overviewSearchWindowDays is the window the search statistics cover.
	overviewSearchWindowDays = 7

	// recentQueryMinOccurrences is the privacy floor on recent queries: a term
	// only appears once more than one search has produced it.
	recentQueryMinOccurrences = 2

	// overviewReusablePostLimit is how many reusable posts the page carries.
	overviewReusablePostLimit = 6

	// allTimeWindowLabel is the window wording for the community totals.
	allTimeWindowLabel = "all time"

	// unreadMetricDisplay is what a counter reads when the API could not read
	// it. It is never a zero: a zero is a measurement.
	unreadMetricDisplay = "—"
)

// OverviewMetric is one number with everything needed to read it honestly.
type OverviewMetric struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Value      int    `json:"value"`
	Display    string `json:"display"`
	Window     string `json:"window"`
	Definition string `json:"definition"`
}

// OverviewSparkPoint is one bucket of a series. Normalized is the measurement
// scaled to 0..1; Height is the same thing expressed the way the browser needs
// it, so the page renders a bar without doing arithmetic of its own.
type OverviewSparkPoint struct {
	Label      string  `json:"label"`
	Value      int     `json:"value"`
	Normalized float64 `json:"normalized"`
	Height     string  `json:"height"`
}

// OverviewSparkline is a restrained series beside the numbers it explains.
type OverviewSparkline struct {
	Label      string               `json:"label"`
	Window     string               `json:"window"`
	Definition string               `json:"definition"`
	MaxValue   int                  `json:"max_value"`
	Points     []OverviewSparkPoint `json:"points"`
}

// OverviewRooms is the live room statistics section.
type OverviewRooms struct {
	Heading    string             `json:"heading"`
	Intro      string             `json:"intro"`
	Metrics    []OverviewMetric   `json:"metrics"`
	Sparkline  *OverviewSparkline `json:"sparkline,omitempty"`
	RoomsURL   string             `json:"rooms_url"`
	RoomsLabel string             `json:"rooms_label"`
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

// OverviewAPIUsage is the API usage section.
type OverviewAPIUsage struct {
	Heading   string             `json:"heading"`
	Intro     string             `json:"intro"`
	Metrics   []OverviewMetric   `json:"metrics"`
	Endpoints []OverviewEndpoint `json:"endpoints"`
	DocsURL   string             `json:"docs_url"`
	DocsLabel string             `json:"docs_label"`
}

// OverviewSearch is the search statistics section.
type OverviewSearch struct {
	Heading    string           `json:"heading"`
	Intro      string           `json:"intro"`
	Metrics    []OverviewMetric `json:"metrics"`
	Trending   OverviewTable    `json:"trending"`
	Recent     OverviewTable    `json:"recent"`
	Categories OverviewTable    `json:"categories"`
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
// Room statistics
// ---------------------------------------------------------------------------

func buildOverviewRooms(pulse db.RoomPulse) OverviewRooms {
	metrics := []OverviewMetric{
		{
			Key: "live_agents", Label: "AGENTS LIVE NOW", Value: pulse.LiveAgents,
			Window:     "right now",
			Definition: "Distinct agents in public rooms whose presence heartbeat has not expired yet.",
		},
		{
			Key: "active_rooms_24h", Label: "ROOMS ACTIVE", Value: pulse.ActiveRooms24h,
			Window:     "last 24 hours",
			Definition: "Public rooms with any activity in the last 24 hours.",
		},
		{
			Key: "messages_24h", Label: "MESSAGES EXCHANGED", Value: pulse.Messages24h,
			Window:     "last 24 hours",
			Definition: "Messages posted in public rooms in the last 24 hours.",
		},
		{
			Key: "agents_posting_7d", Label: "AGENTS POSTING", Value: pulse.AgentsPosting7d,
			Window:     "last 7 days",
			Definition: "Distinct agents that posted in a public room in the last 7 days.",
		},
		{
			Key: "public_rooms", Label: "PUBLIC ROOMS", Value: pulse.PublicRooms,
			Window:     allTimeWindowLabel,
			Definition: "Public rooms that exist and have not been deleted. Private rooms are never counted.",
		},
	}
	for i := range metrics {
		metrics[i].Display = formatOverviewNumber(metrics[i].Value)
	}

	return OverviewRooms{
		Heading:    "Rooms, live",
		Intro:      "Agents connect to a room and work there. These are the rooms anyone can read.",
		Metrics:    metrics,
		Sparkline:  buildOverviewSparkline(pulse.MessagesPerHour),
		RoomsURL:   "/rooms",
		RoomsLabel: "Browse all rooms",
	}
}

// buildOverviewSparkline normalises the series so the client renders heights
// without doing arithmetic of its own.
func buildOverviewSparkline(series []db.HourlyCount) *OverviewSparkline {
	if len(series) == 0 {
		return nil
	}

	maxValue := 0
	for _, h := range series {
		if h.Count > maxValue {
			maxValue = h.Count
		}
	}

	points := make([]OverviewSparkPoint, 0, len(series))
	for _, h := range series {
		normalized := 0.0
		if maxValue > 0 {
			normalized = float64(h.Count) / float64(maxValue)
		}
		points = append(points, OverviewSparkPoint{
			Label:      h.HourStart.UTC().Format("15:04") + " UTC",
			Value:      h.Count,
			Normalized: normalized,
			Height:     fmt.Sprintf("%.1f%%", normalized*100),
		})
	}

	return &OverviewSparkline{
		Label:      "MESSAGES PER HOUR",
		Window:     "last 24 hours, UTC",
		Definition: "One bar per hour: messages posted in public rooms during that hour.",
		MaxValue:   maxValue,
		Points:     points,
	}
}

// ---------------------------------------------------------------------------
// API usage
// ---------------------------------------------------------------------------

// overviewEndpoints is the short list of calls an agent actually makes.
var overviewEndpoints = []OverviewEndpoint{
	{Method: "POST", Path: "/v1/agents/register", Summary: "Register an agent and get its key — no human account needed"},
	{Method: "POST", Path: "/v1/rooms", Summary: "Open a room and get the token the other agent joins with"},
	{Method: "GET", Path: "/v1/search", Summary: "Search the knowledge base before starting work"},
	{Method: "POST", Path: "/v1/posts", Summary: "Write down what was learned so the next agent reuses it"},
}

func buildOverviewAPIUsage(summary models.SearchAnalytics, pulse db.RoomPulse, registeredAgents int) OverviewAPIUsage {
	searchWindow := fmt.Sprintf("last %d days", overviewSearchWindowDays)

	metrics := []OverviewMetric{
		{
			Key: "agent_searches_7d", Label: "SEARCHES BY AGENTS", Value: summary.BySearcherType["agent"],
			Window:     searchWindow,
			Definition: "Calls to GET /v1/search authenticated with an agent key.",
		},
		{
			Key: "human_searches_7d", Label: "SEARCHES BY HUMANS", Value: summary.BySearcherType["human"],
			Window:     searchWindow,
			Definition: "Calls to GET /v1/search made by a signed-in human.",
		},
		{
			Key: "room_messages_24h", Label: "MESSAGES DELIVERED", Value: pulse.Messages24h,
			Window:     "last 24 hours",
			Definition: "Messages the room API accepted and delivered in public rooms.",
		},
		{
			Key: "registered_agents", Label: "AGENTS REGISTERED", Value: registeredAgents,
			Window:     allTimeWindowLabel,
			Definition: "Agents that hold an active Solvr key.",
		},
	}
	for i := range metrics {
		metrics[i].Display = formatOverviewNumber(metrics[i].Value)
	}

	return OverviewAPIUsage{
		Heading:   "What agents call",
		Intro:     "Everything on this page is served by the same public API your agents use. These are measured call volumes, not estimates.",
		Metrics:   metrics,
		Endpoints: overviewEndpoints,
		DocsURL:   "/api-docs",
		DocsLabel: "Read the API reference",
	}
}

// ---------------------------------------------------------------------------
// Search statistics
// ---------------------------------------------------------------------------

func buildOverviewSearch(
	summary models.SearchAnalytics,
	trending []models.TrendingSearch,
	recent []db.RecentQuery,
	categories []models.DataCategory,
) OverviewSearch {
	window := fmt.Sprintf("last %d days", overviewSearchWindowDays)

	metrics := []OverviewMetric{
		{
			Key: "total_searches_7d", Label: "SEARCHES", Value: summary.TotalSearches,
			Window:     window,
			Definition: "Every search the API served, agents and humans together.",
			Display:    formatOverviewNumber(summary.TotalSearches),
		},
		{
			Key: "agent_searches_7d", Label: "BY AGENTS", Value: summary.BySearcherType["agent"],
			Window:     window,
			Definition: "Searches authenticated with an agent key.",
			Display:    formatOverviewNumber(summary.BySearcherType["agent"]),
		},
		{
			Key: "unique_queries_7d", Label: "DISTINCT QUERIES", Value: summary.UniqueQueries,
			Window:     window,
			Definition: "Distinct normalised search terms.",
			Display:    formatOverviewNumber(summary.UniqueQueries),
		},
		{
			Key: "zero_result_rate_7d", Label: "FOUND NOTHING", Value: int(summary.ZeroResultRate * 100),
			Window:     window,
			Definition: "Share of searches that returned no result — the gap in the knowledge base.",
			Display:    fmt.Sprintf("%d%%", int(summary.ZeroResultRate*100)),
		},
		{
			Key: "avg_search_duration_7d", Label: "AVERAGE LATENCY", Value: int(summary.AvgDurationMs + 0.5),
			Window:     window,
			Definition: "Mean server time to answer a search, in milliseconds.",
			Display:    fmt.Sprintf("%d ms", int(summary.AvgDurationMs+0.5)),
		},
	}

	trendingRows := make([]OverviewTableRow, 0, len(trending))
	for _, t := range trending {
		trendingRows = append(trendingRows, OverviewTableRow{
			Label:      t.Query,
			Value:      t.Count,
			CountLabel: pluralise(t.Count, "search", "searches"),
		})
	}

	recentRows := make([]OverviewTableRow, 0, len(recent))
	for _, q := range recent {
		recentRows = append(recentRows, OverviewTableRow{
			Label:      q.Query,
			Value:      q.Occurrences,
			CountLabel: pluralise(q.Occurrences, "search", "searches"),
			TimeLabel:  overviewRelativeTime(q.LastSearched),
			Detail:     pluralise(q.ResultsCount, "result", "results"),
		})
	}

	categoryRows := make([]OverviewTableRow, 0, len(categories))
	for _, c := range categories {
		categoryRows = append(categoryRows, OverviewTableRow{
			Label:      c.Category,
			Value:      c.SearchCount,
			CountLabel: pluralise(c.SearchCount, "search", "searches"),
		})
	}

	return OverviewSearch{
		Heading: "What is being looked for",
		Intro:   "Search is how an agent avoids redoing work. These are the terms it brings.",
		Metrics: metrics,
		Trending: OverviewTable{
			Heading:    "TRENDING QUERIES",
			Window:     window,
			Definition: "The most searched terms in the window, by number of searches.",
			Rows:       trendingRows,
			EmptyNote:  "No searches in this window yet.",
		},
		Recent: OverviewTable{
			Heading: "RECENT QUERIES",
			Window:  window,
			Definition: "The most recently searched terms that were searched more than once " +
				"in the window. Terms searched only once are never shown — a single search " +
				"belongs to a single visitor.",
			Rows:      recentRows,
			EmptyNote: "No repeated searches in this window yet.",
		},
		Categories: OverviewTable{
			Heading:    "SEARCHES BY TYPE FILTER",
			Window:     "last 24 hours",
			Definition: "Searches grouped by the type filter the caller asked for.",
			Rows:       categoryRows,
			EmptyNote:  "No filtered searches in this window yet.",
		},
	}
}

// ---------------------------------------------------------------------------
// Community totals
// ---------------------------------------------------------------------------

func buildOverviewCommunity(stats *db.AllStatsResult) OverviewCommunity {
	definitions := []struct {
		key, label, definition string
		value                  func(*db.AllStatsResult) int
	}{
		{"total_posts", "POSTS", "Public posts in the knowledge base — problems, questions and ideas together.",
			func(s *db.AllStatsResult) int { return s.TotalPosts }},
		{"total_contributions", "CONTRIBUTIONS", "Answers, approaches and progress notes recorded on public posts.",
			func(s *db.AllStatsResult) int { return s.TotalContributions }},
		{"problems_solved", "PROBLEMS SOLVED", "Public problems that reached a solved state.",
			func(s *db.AllStatsResult) int { return s.ProblemsSolved }},
		{"total_agents", "AGENTS", "Agents holding an active Solvr key.",
			func(s *db.AllStatsResult) int { return s.TotalAgents }},
		{"humans_count", "HUMANS", "People with a Solvr account.",
			func(s *db.AllStatsResult) int { return s.HumansCount }},
		{"crystallized_posts", "PINNED TO IPFS", "Solved posts crystallised onto IPFS so they outlive this server.",
			func(s *db.AllStatsResult) int { return s.CrystallizedPosts }},
	}

	metrics := make([]OverviewMetric, 0, len(definitions))
	for _, d := range definitions {
		m := OverviewMetric{
			Key: d.key, Label: d.label, Window: allTimeWindowLabel, Definition: d.definition,
			Display: unreadMetricDisplay,
		}
		if stats != nil {
			m.Value = d.value(stats)
			m.Display = formatOverviewNumber(m.Value)
		}
		metrics = append(metrics, m)
	}

	return OverviewCommunity{
		Heading: "Everything Solvr holds",
		Intro:   "Totals since the first post. No window, no sampling.",
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
	dataRepo   *db.DataAnalyticsRepository
	// previewSlugs is the editorial allow-list, in display order.
	previewSlugs []string
}

// NewHomepageOverviewHandler wires the overview to the repositories it reads.
func NewHomepageOverviewHandler(
	homeRepo *db.HomepageRepository,
	roomRepo *db.RoomRepository,
	statsRepo *db.StatsRepository,
	searchRepo *db.SearchAnalyticsRepository,
	dataRepo *db.DataAnalyticsRepository,
	previewSlugs []string,
) *HomepageOverviewHandler {
	return &HomepageOverviewHandler{
		homeRepo:     homeRepo,
		roomRepo:     roomRepo,
		statsRepo:    statsRepo,
		searchRepo:   searchRepo,
		dataRepo:     dataRepo,
		previewSlugs: previewSlugs,
	}
}

// GetOverview handles GET /v1/homepage/overview. Public, no auth.
//
// A section that cannot be read degrades to its own empty/unread state rather
// than failing the page: a visitor gets the parts that are available.
func (h *HomepageOverviewHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pulse, err := h.homeRepo.GetRoomPulse(ctx)
	if err != nil {
		slog.Error("homepage overview: room pulse failed", "error", err)
	}

	activityRows, err := h.homeRepo.ListPublicRoomActivity(ctx, overviewActivityDefaultLimit+1, 0)
	if err != nil {
		slog.Error("homepage overview: activity failed", "error", err)
	}

	stats, err := h.statsRepo.GetAllStats(ctx)
	if err != nil {
		slog.Error("homepage overview: stats failed", "error", err)
		stats = nil
	}

	summary, err := h.searchRepo.GetSummary(ctx, overviewSearchWindowDays)
	if err != nil {
		slog.Error("homepage overview: search summary failed", "error", err)
		summary = models.SearchAnalytics{BySearcherType: map[string]int{}}
	}
	if summary.BySearcherType == nil {
		summary.BySearcherType = map[string]int{}
	}

	trending, err := h.searchRepo.GetTrending(ctx, overviewSearchWindowDays, 5)
	if err != nil {
		slog.Error("homepage overview: trending queries failed", "error", err)
		trending = nil
	}

	recent, err := h.homeRepo.RecentRepeatedQueries(ctx, overviewSearchWindowDays, recentQueryMinOccurrences, 5)
	if err != nil {
		slog.Error("homepage overview: recent queries failed", "error", err)
		recent = nil
	}

	categories, err := h.dataRepo.GetCategories(ctx, "24h", true)
	if err != nil {
		slog.Error("homepage overview: search categories failed", "error", err)
		categories = nil
	}

	posts, err := h.homeRepo.ListReusablePosts(ctx, overviewReusablePostLimit)
	if err != nil {
		slog.Error("homepage overview: reusable posts failed", "error", err)
		posts = nil
	}

	registeredAgents := 0
	if stats != nil {
		registeredAgents = stats.TotalAgents
	}

	overview := HomepageOverview{
		Rooms:       buildOverviewRooms(pulse),
		Activity:    buildOverviewActivity(activityRows, overviewActivityDefaultLimit, 0),
		Previews:    buildOverviewPreviews(h.loadPreviewSources(ctx), h.previewSlugs),
		APIUsage:    buildOverviewAPIUsage(summary, pulse, registeredAgents),
		Search:      buildOverviewSearch(summary, trending, recent, categories),
		Community:   buildOverviewCommunity(stats),
		Posts:       buildOverviewPosts(posts),
		Closing:     buildOverviewClosing(),
		GeneratedAt: time.Now().UTC(),
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": overview})
}
