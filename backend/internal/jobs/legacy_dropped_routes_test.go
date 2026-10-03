package jobs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Every GET route the API serves runs through the real router against a database without
// the legacy tables, once per caller. The static-query probe cannot see SQL built at run
// time (list filters, sort orders, search content types); this one runs it. A route that
// answered below 500 before the drop and afterwards answers 5xx, or raises a database error
// the same request did not raise before (a missing legacy relation, even one the handler
// swallows into an empty 200), must belong to a legacy route family still served by the
// legacy repositories (idx 52). Anywhere else it is a hidden dependency.

// legacyRepositoryFamilies are the route families whose reads are still served by the legacy
// repositories: they retire with the legacy tables, so reaching for them is expected. Their
// write routes are already retired (idx 52) and must reach no table at all.
var legacyRepositoryFamilies = map[string]bool{
	"legacy-feed":            true,
	"legacy-typed-discovery": true,
	"legacy-typed-reads":     true,
	"legacy-typed-writes":    true,
	"legacy-comments":        true,
	"legacy-status-commands": true,
}

const routeProbeAdminKey = "route-probe-admin-key"

type routeProbeFixture struct {
	userID, jwt, agentID, agentKey, roomSlug string
	posts                                    map[string]string // post type -> id
	replies                                  []string
}

type routeProbeRequest struct {
	route, family, path, caller string
}

type routeProbeResult struct {
	status int
	errs   []tracedError
}

func routeProbeJWTSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "test-jwt-secret-32-chars-long!!" // the router's own fallback
}

func newRouteProbeRouter(pool *db.Pool) http.Handler {
	registry := hub.NewPresenceRegistry()
	return api.NewRouter(pool, hub.NewHubManager(context.Background(), registry, slog.Default(), 0), registry)
}

// seedRouteProbeData runs on the migrated schema before the drop: it lifts the rate limits
// (the probe sends hundreds of requests per caller), registers an agent through the router,
// and seeds a user, one post of each type with native and migrated replies, a room seeded
// from a post, a bookmark and a notification for each caller.
func seedRouteProbeData(ctx context.Context, t *testing.T, pool *db.Pool) routeProbeFixture {
	t.Helper()
	must := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("seed %s: %v", what, err)
		}
	}
	_, err := pool.Exec(ctx, `UPDATE rate_limit_config SET value = CASE WHEN key = 'new_account_threshold_hours' THEN 0 ELSE 1000000 END`)
	must("rate limits", err)

	fx := routeProbeFixture{posts: map[string]string{}}
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	user, err := db.NewUserRepository(pool).Create(ctx, &models.User{
		Username: "probe_" + suffix, DisplayName: "Route Probe", Email: "probe_" + suffix + "@test.solvr.dev",
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: "probe_" + suffix, Role: "user",
	})
	must("user", err)
	fx.userID = user.ID
	fx.jwt, err = auth.GenerateJWT(routeProbeJWTSecret(), user.ID, user.Email, "user", time.Hour)
	must("jwt", err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register",
		strings.NewReader(`{"name":"route_probe_`+suffix+`","description":"route probe agent"}`))
	req.Header.Set("Content-Type", "application/json")
	newRouteProbeRouter(pool).ServeHTTP(rec, req)
	var reg struct {
		APIKey string `json:"api_key"`
		Agent  struct {
			ID string `json:"id"`
		} `json:"agent"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &reg) != nil || reg.APIKey == "" {
		t.Fatalf("register probe agent: %d %s", rec.Code, rec.Body.String())
	}
	fx.agentID, fx.agentKey = reg.Agent.ID, reg.APIKey

	for i, typ := range []string{"problem", "question", "idea", "post"} {
		authorType, authorID := "agent", fx.agentID
		if i%2 == 1 {
			authorType, authorID = "human", fx.userID
		}
		var id string
		must("post "+typ, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
			                   publication_state, moderation_state, visibility)
			VALUES ($1, $4, $5, ARRAY['probe'], 'open', $2, $3, 'published', 'approved', 'public')
			RETURNING id::text`, typ, authorType, authorID,
			"Route probe "+typ, "A "+typ+" the route probe reads through every route").Scan(&id))
		fx.posts[typ] = id
		var native, migrated string
		must("native reply", pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'human', $2, 'a native probe reply')
			RETURNING id::text`, id, fx.userID).Scan(&native))
		legacy := map[string]string{"problem": "approach", "question": "answer", "idea": "response", "post": "comment"}[typ]
		// Each legacy type's provenance holds only the keys its migration writes (000118).
		provenance := map[string]string{
			"approach": `{"legacy_table": "approaches", "status": "succeeded", "angle": "probe angle"}`,
			"answer":   `{"legacy_table": "answers", "is_accepted": true}`,
			"response": `{"legacy_table": "responses", "response_type": "build"}`,
			"comment":  `{"legacy_table": "comments", "target_type": "post", "target_id": "` + id + `"}`,
		}[legacy]
		must("migrated reply", pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
			VALUES ($1, 'agent', $2, 'a migrated probe reply', $3, gen_random_uuid(), $4::jsonb)
			RETURNING id::text`, id, fx.agentID, legacy, provenance).Scan(&migrated))
		fx.replies = append(fx.replies, native, migrated)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO bookmarks (user_type, user_id, post_id) VALUES ('human', $1, $3), ('agent', $2, $3)`,
		fx.userID, fx.agentID, fx.posts["post"])
	must("bookmarks", err)
	_, err = pool.Exec(ctx, `
		INSERT INTO notifications (user_id, agent_id, type, title, link)
		VALUES ($1::uuid, NULL, 'reply', 'probe', '/posts/'||$3), (NULL, $2, 'reply', 'probe', '/posts/'||$3)`,
		fx.userID, fx.agentID, fx.posts["post"])
	must("notifications", err)
	source := fx.posts["problem"]
	room, err := db.NewRoomRepository(pool).Create(ctx, models.CreateRoomParams{
		DisplayName: "Route probe room", OwnerID: uuid.MustParse(fx.userID), CreatorAgentID: fx.agentID, SourcePostID: &source,
	})
	must("room", err)
	fx.roomSlug = room.Slug
	return fx
}

// routeProbePaths fills a route template's parameters from the fixture, one path per
// combination of values. {id} is chosen by the segment before it; a parameter the fixture
// has no row for gets a well-formed value that names nothing, so the route still runs its
// lookup.
func routeProbePaths(route string, fx routeProbeFixture) []string {
	path := strings.SplitN(route, " ", 2)[1]
	segs := strings.Split(path, "/")
	variants := [][]string{{}}
	for i, seg := range segs {
		vals := []string{seg}
		if strings.HasPrefix(seg, "{") {
			prev := ""
			if i > 0 {
				prev = segs[i-1]
			}
			switch {
			case seg == "{slug}" && (prev == "rooms" || prev == "r"):
				vals = []string{fx.roomSlug}
			case seg == "{slug}":
				vals = []string{"route-probe-missing"}
			case seg == "{tag}":
				vals = []string{"probe"}
			case seg == "{agent_name}":
				vals = []string{fx.agentID}
			case seg == "{entry_id}" || prev == "messages": // room entries have integer ids
				vals = []string{"1"}
			case prev == "posts" || prev == "bookmarks":
				vals = []string{fx.posts["problem"], fx.posts["question"], fx.posts["idea"], fx.posts["post"]}
			case prev == "problems":
				vals = []string{fx.posts["problem"]}
			case prev == "questions":
				vals = []string{fx.posts["question"]}
			case prev == "ideas":
				vals = []string{fx.posts["idea"]}
			case prev == "replies":
				vals = fx.replies
			case prev == "agents":
				vals = []string{fx.agentID}
			case prev == "users":
				vals = []string{fx.userID}
			default:
				vals = []string{uuid.NewString()}
			}
		}
		var next [][]string
		for _, v := range variants {
			for _, val := range vals {
				next = append(next, append(append([]string{}, v...), val))
			}
		}
		variants = next
	}
	paths := make([]string, 0, len(variants))
	for _, v := range variants {
		paths = append(paths, strings.Join(v, "/"))
	}
	return paths
}

// routeProbeQueries are the query variants that build SQL at run time.
func routeProbeQueries(fx routeProbeFixture) map[string][]string {
	return map[string][]string{
		"GET /v1/posts": {
			"type=problem", "type=question", "type=idea", "type=post", "status=open", "status=solved", "tags=probe",
			"has_answer=true", "has_answer=false", "needs_help=true", "sort=newest", "sort=votes", "sort=top",
			"sort=hot", "sort=approaches", "sort=answers", "timeframe=week",
			"author_type=agent&author_id=" + fx.agentID, "author_type=human&author_id=" + fx.userID,
		},
		"GET /v1/search": {
			"q=probe", "q=probe&type=problem", "q=probe&type=question", "q=probe&status=open", "q=probe&sort=votes",
			"q=probe&sort=newest", "q=probe&tags=probe", "q=probe&author_type=agent",
			"q=probe&content_types=posts", "q=probe&content_types=answers", "q=probe&content_types=approaches",
			"q=probe&content_types=posts,answers,approaches",
		},
		"GET /v1/leaderboard": {
			"type=agents", "type=users", "timeframe=weekly", "timeframe=monthly", "timeframe=all_time",
		},
		"GET /v1/leaderboard/tags/{tag}": {"timeframe=weekly", "type=agents"},
		"GET /v1/sitemap/urls":           {"type=posts", "type=agents", "type=users", "type=blog_posts", "type=rooms"},
		"GET /v1/agents":                 {"sort=reputation", "sort=newest"},
		"GET /v1/users":                  {"sort=reputation", "sort=newest"},
		"GET /v1/overview":               {"window=24h", "window=7d", "window=30d"},
		"GET /v1/me/diff":                {"since=" + url.QueryEscape(time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339))},
		"GET /admin/cohort-comparison":   {"launch=" + url.QueryEscape(time.Now().Add(-7*24*time.Hour).UTC().Format(time.RFC3339))},
		"GET /v1/reports/check": {
			"target_type=post&target_id=" + fx.posts["post"], "target_type=reply&target_id=" + fx.replies[1],
			"target_type=answer&target_id=" + fx.replies[3], "target_type=approach&target_id=" + fx.replies[1],
			"target_type=response&target_id=" + fx.replies[5], "target_type=comment&target_id=" + fx.replies[7],
		},
	}
}

func routeProbeRequests(fx routeProbeFixture) []routeProbeRequest {
	queries := routeProbeQueries(fx)
	var reqs []routeProbeRequest
	for _, f := range api.RouteFamilies {
		for _, route := range f.Routes {
			if !strings.HasPrefix(route, "GET ") {
				continue
			}
			for _, path := range routeProbePaths(route, fx) {
				targets := []string{path}
				for _, q := range queries[route] {
					targets = append(targets, path+"?"+q)
				}
				for _, target := range targets {
					for _, caller := range []string{"anonymous", "human", "agent"} {
						reqs = append(reqs, routeProbeRequest{route: route, family: f.Name, path: target, caller: caller})
					}
				}
			}
		}
	}
	return reqs
}

// serveRouteProbe runs one request with a deadline and returns its status and the database
// errors traced meanwhile. Stream routes hold the connection open until their context ends,
// so they get a short one; every other route gets the write probe's per-call 10s.
func serveRouteProbe(t *testing.T, router http.Handler, tracer *dbErrorTracer, fx routeProbeFixture, r routeProbeRequest) routeProbeResult {
	t.Helper()
	tracer.take()
	deadline := 10 * time.Second
	if strings.HasSuffix(r.route, "/stream") {
		deadline = 500 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, r.path, nil).WithContext(ctx)
	switch r.caller {
	case "human":
		req.Header.Set("Authorization", "Bearer "+fx.jwt)
	case "agent":
		req.Header.Set("Authorization", "Bearer "+fx.agentKey)
	}
	if strings.HasPrefix(r.path, "/admin/") {
		req.Header.Set("X-Admin-API-Key", routeProbeAdminKey)
	}
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		router.ServeHTTP(rec, req)
	}()
	select {
	case <-done:
	case <-time.After(deadline + 15*time.Second):
		t.Fatalf("%s as %s did not return within 15s of its %s deadline", r.path, r.caller, deadline)
	}
	_, errs := tracer.take()
	return routeProbeResult{status: rec.Code, errs: errs}
}

// The short deadline exists for stream routes, which hold the connection open until their
// context ends. An ordinary GET that is merely slow (a loaded machine) must not be cut off
// by it and answer 500 after the drop: the judge would report a hidden legacy dependency
// for a query that reads no legacy table (2026-09-29: /admin/search-analytics/summary,
// 509ms, "200 -> 500, missing []"). Needs no database.
func TestServeRouteProbe_OnlyStreamRoutesGetTheShortDeadline(t *testing.T) {
	router := http.NewServeMux()
	router.HandleFunc("/v1/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(700 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	router.HandleFunc("/v1/rooms/probe/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		<-r.Context().Done()
	})
	tracer := &dbErrorTracer{}

	slow := serveRouteProbe(t, router, tracer, routeProbeFixture{},
		routeProbeRequest{route: "GET /v1/slow", family: "probe", path: "/v1/slow", caller: "anonymous"})
	if slow.status != http.StatusOK {
		t.Errorf("a GET that takes 700ms answered %d: the stream deadline cut it off", slow.status)
	}

	start := time.Now()
	stream := serveRouteProbe(t, router, tracer, routeProbeFixture{},
		routeProbeRequest{route: "GET /v1/rooms/{slug}/stream", family: "probe", path: "/v1/rooms/probe/stream", caller: "anonymous"})
	if elapsed := time.Since(start); stream.status != http.StatusOK || elapsed > 2*time.Second {
		t.Errorf("stream route: status %d after %s, want 200 once its short deadline ends", stream.status, elapsed)
	}
}

func TestLegacyDroppedDatabase_GetRoutesExposeOnlyLegacyRouteFamilies(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", routeProbeAdminKey)
	var fx routeProbeFixture
	var reqs []routeProbeRequest
	before := map[int]routeProbeResult{}
	d := newLegacyDroppedDatabase(t, func(ctx context.Context, conn *pgx.Conn) {
		tracer := &dbErrorTracer{}
		pool, err := db.NewPool(ctx, conn.Config().ConnString(), db.WithQueryTracer(tracer))
		if err != nil {
			t.Fatalf("open pre-drop pool: %v", err)
		}
		defer pool.Close()
		fx = seedRouteProbeData(ctx, t, pool)
		reqs = routeProbeRequests(fx)
		router := newRouteProbeRouter(pool)
		for i, r := range reqs {
			before[i] = serveRouteProbe(t, router, tracer, fx, r)
		}
	})

	router := newRouteProbeRouter(d.pool)
	var hidden, expected, gated []string
	controlSeen := false
	routes, reached := map[string]bool{}, map[string]bool{}
	statuses := map[string][]int{}
	for i, r := range reqs {
		routes[r.route] = true
		b := before[i]
		if b.status >= 500 || b.status == http.StatusTooManyRequests {
			gated = append(gated, fmt.Sprintf("%s as %s: %d before the drop", r.path, r.caller, b.status))
			continue
		}
		a := serveRouteProbe(t, router, d.tracer, fx, r)
		statuses[r.route] = append(statuses[r.route], a.status)
		if a.status >= 200 && a.status < 300 {
			reached[r.route] = true
		}
		if a.status == http.StatusTooManyRequests {
			t.Errorf("%s as %s was rate limited after the drop: the probe measured the limiter", r.path, r.caller)
			continue
		}
		// A database error the same request did not raise before the drop is the drop's doing.
		seen := map[string]bool{}
		for _, e := range b.errs {
			seen[e.Code+" "+e.Message] = true
		}
		var fresh []tracedError
		var missing []string
		for _, e := range a.errs {
			if seen[e.Code+" "+e.Message] {
				continue
			}
			fresh = append(fresh, e)
			if table, ok := missingLegacyObject(e); ok {
				missing = append(missing, table)
			}
		}
		if a.status < 500 && len(fresh) == 0 {
			continue
		}
		line := fmt.Sprintf("%s (%s) as %s: %d -> %d, missing %v", r.path, r.family, r.caller, b.status, a.status, missing)
		if legacyRepositoryFamilies[r.family] {
			expected = append(expected, line)
			continue
		}
		hidden = append(hidden, line)
		for _, e := range fresh {
			hidden = append(hidden, fmt.Sprintf("    %s %s | %.200s", e.Code, e.Message, strings.Join(strings.Fields(e.SQL), " ")))
		}
	}
	var unreached []string
	for route := range routes {
		if !reached[route] {
			unreached = append(unreached, fmt.Sprintf("%s %v", route, statuses[route]))
		}
	}
	sort.Strings(gated)
	sort.Strings(unreached)
	t.Logf("%d GET requests over %d GET routes; %d answered 5xx before the drop and are not judged:\n%s",
		len(reqs), len(routes), len(gated), strings.Join(gated, "\n"))
	t.Logf("%d routes answered no caller with 2xx after the drop (they ran auth, validation or a lookup only):\n%s",
		len(unreached), strings.Join(unreached, "\n"))
	t.Logf("%d requests in legacy route families reach dropped legacy tables:\n%s", len(expected), strings.Join(expected, "\n"))
	if len(hidden) > 0 {
		t.Errorf("hidden legacy dependencies: served outside the legacy route families, these fail once the legacy tables are gone:\n%s",
			strings.Join(hidden, "\n"))
	}

	// Positive control: the probe must still see a request that reaches a dropped legacy table.
	// No served route does since the legacy reads were retired (idx 73 step 3), and the handler
	// GET /v1/questions/{id}/answers was mounted on went with the legacy handlers (idx 68), so the
	// control serves the read it made, the question's answers, through the same probe, tracer and
	// judge.
	legacy := chi.NewRouter()
	legacy.Get("/v1/questions/{id}/answers", func(w http.ResponseWriter, req *http.Request) {
		var n int
		if err := d.pool.QueryRow(req.Context(), `SELECT count(*) FROM answers WHERE question_id::text = $1`,
			chi.URLParam(req, "id")).Scan(&n); err != nil {
			http.Error(w, "list answers failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	control := serveRouteProbe(t, legacy, d.tracer, fx, routeProbeRequest{route: "GET /v1/questions/{id}/answers",
		family: "legacy-typed-reads", path: "/v1/questions/" + fx.posts["question"] + "/answers", caller: "anonymous"})
	for _, e := range control.errs {
		if table, ok := missingLegacyObject(e); ok && table == "answers" {
			controlSeen = true
		}
	}
	if !controlSeen {
		t.Errorf("positive control: the legacy answers read (GET /v1/questions/{id}/answers until idx 73) must reach "+
			"the dropped answers table; status %d, errors %v", control.status, control.errs)
	}
}
