package jobs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/jackc/pgx/v5"
)

// Every write route the API serves (POST, PATCH, DELETE) runs through the real router in
// one fixed order on two scratch databases seeded alike: one migrated to head that still
// holds the legacy tables, one where they are dropped. A write changes the state later
// writes see, so unlike the GET probe the two runs cannot share one database. Call i of
// the dropped run is judged against call i of the present run: a call that answered below
// 500 there and here answers 5xx, or raises a database error it did not raise there, must
// belong to a legacy route family (idx 52). Anywhere else it is a hidden dependency.

// writeProbeMissing stands in for an id a previous call could not create, so both runs
// issue the same calls in the same order.
const writeProbeMissing = "00000000-0000-0000-0000-000000000000"

type writeProbeState struct {
	routeProbeFixture
	humanNotification, agentNotification string
	agent2ID, agent2Key                  string
	human2ID, human2JWT                  string
	claimToken, apiKeyID                 string
	created                              map[string]string // author caller + type -> post id made by POST /v1/posts
	roomSlug2, roomToken, messageID      string
	savedPostID, blogSlug, pinID         string
	captureFailures                      []string
}

type writeProbeCall struct {
	route, caller, path, body, contentType string
	// keep reads what later calls need from a 2xx response; it reports whether it found it.
	keep func(s *writeProbeState, v jsonValue) bool
}

type writeProbeResult struct {
	call   writeProbeCall
	family string
	status int
	errs   []tracedError
	body   string
}

// jsonValue is a decoded JSON response that can be walked by key.
type jsonValue struct{ v any }

func (j jsonValue) at(keys ...string) string {
	cur := j.v
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[k]
	}
	switch x := cur.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	}
	return ""
}

// or returns the id, or writeProbeMissing when an earlier call could not create it.
func or(id string) string {
	if id == "" {
		return writeProbeMissing
	}
	return id
}

// seedWriteProbeData seeds what the GET probe seeds and records each caller's notification.
func seedWriteProbeData(ctx context.Context, t *testing.T, pool *db.Pool) *writeProbeState {
	t.Helper()
	s := &writeProbeState{routeProbeFixture: seedRouteProbeData(ctx, t, pool), created: map[string]string{}}
	if err := pool.QueryRow(ctx, `SELECT id::text FROM notifications WHERE user_id = $1::uuid`, s.userID).Scan(&s.humanNotification); err != nil {
		t.Fatalf("seed: human notification: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id::text FROM notifications WHERE agent_id = $1`, s.agentID).Scan(&s.agentNotification); err != nil {
		t.Fatalf("seed: agent notification: %v", err)
	}
	return s
}

func writeProbeFamilies() map[string]string {
	families := map[string]string{}
	for _, f := range api.RouteFamilies {
		for _, route := range f.Routes {
			families[route] = f.Name
		}
	}
	return families
}

// serveWriteProbe runs one call and returns its status and the database errors traced
// while it ran and shortly after (handlers finish some writes in the background).
func serveWriteProbe(t *testing.T, router http.Handler, tracer *dbErrorTracer, s *writeProbeState, c writeProbeCall) (int, []tracedError, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	method := strings.SplitN(c.route, " ", 2)[0]
	bearer := map[string]string{
		"human": s.jwt, "agent": s.agentKey, "human2": s.human2JWT, "agent2": s.agent2Key, "room": s.roomToken,
	}[c.caller]
	// An edit sends the ETag of the caller's own read as If-Match, as a client does: edits of
	// posts, replies and rooms require it since spec.json idx 74 step 5 (428 without it).
	ifMatch := ""
	if method == http.MethodPatch {
		read := httptest.NewRequest(http.MethodGet, c.path, nil).WithContext(ctx)
		if bearer != "" {
			read.Header.Set("Authorization", "Bearer "+bearer)
		}
		readRec := httptest.NewRecorder()
		router.ServeHTTP(readRec, read)
		ifMatch = readRec.Header().Get("ETag")
	}
	tracer.take()
	req := httptest.NewRequest(method, c.path, strings.NewReader(c.body)).WithContext(ctx)
	contentType := c.contentType
	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	if c.caller == "admin" {
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
	case <-time.After(20 * time.Second):
		t.Fatalf("%s %s as %s did not return within 20s", method, c.path, c.caller)
	}
	time.Sleep(20 * time.Millisecond)
	_, errs := tracer.take()
	return rec.Code, errs, rec.Body.Bytes()
}

// writeProbeDrain outlasts the background work a write leaves behind: an async pin retries
// the probe's closed IPFS port at 1s, 2s and 3s before it records the failure.
const writeProbeDrain = 8 * time.Second

// runWriteProbe issues every step's calls in order on one database. A step builds its
// calls from what earlier calls created, so it runs only after they did. After the last
// call it waits drain and returns the database errors background work raised meanwhile.
func runWriteProbe(t *testing.T, router http.Handler, tracer *dbErrorTracer, s *writeProbeState, drain time.Duration) ([]writeProbeResult, []tracedError) {
	t.Helper()
	families := writeProbeFamilies()
	var results []writeProbeResult
	for _, step := range writeProbeSteps {
		for _, c := range step(s) {
			family, ok := families[c.route]
			if !ok {
				t.Fatalf("write probe call names %q, which is not a route in api.RouteFamilies", c.route)
			}
			status, errs, body := serveWriteProbe(t, router, tracer, s, c)
			if c.keep != nil {
				var v any
				dec := json.NewDecoder(strings.NewReader(string(body)))
				dec.UseNumber()
				_ = dec.Decode(&v)
				if status < 200 || status >= 300 || !c.keep(s, jsonValue{v}) {
					s.captureFailures = append(s.captureFailures,
						fmt.Sprintf("%s %s as %s: %d %.300s", c.route, c.path, c.caller, status, body))
				}
			}
			results = append(results, writeProbeResult{call: c, family: family, status: status, errs: errs, body: string(body)})
		}
	}
	time.Sleep(drain)
	_, late := tracer.take()
	return results, late
}

// writeProbeValidationOnly are the canonical write routes no call can complete in this
// harness, so the probe runs them only up to validation or an external dependency.
var writeProbeValidationOnly = map[string]string{
	"POST /v1/auth/oauth/exchange":     "a login code is minted only by the OAuth callback round trip with the provider",
	"POST /v1/auth/moltbook":           "a valid identity token is verified against the external Moltbook service",
	"POST /v1/add":                     "uploads go straight to IPFS, which the probe points at a closed port",
	"POST /admin/ipfs/gc":              "repo/gc goes straight to IPFS, which the probe points at a closed port",
	"POST /admin/jobs/translation/run": "the job runner is wired only when GROQ_API_KEY is set; the probe unsets it (the scheduled-jobs probe runs TranslationJob)",
	"POST /admin/email/broadcast":      "the email sender is wired only when RESEND_API_KEY is set; the probe unsets it so nothing is ever sent",
}

type writeProbeVerdict struct {
	hidden, expected, gated, limited []string
	controlSeen                      bool
	mismatch                         string
	reached                          map[string]bool  // routes a call completed (2xx) with the legacy tables present
	statuses                         map[string][]int // per route, the statuses with the legacy tables present
}

// judgeWriteProbe compares call i of the dropped run with call i of the present run. A call
// that answered 5xx or 429 with the legacy tables present is not judged. Otherwise a 5xx, or
// a database error the present run did not raise (even one the handler swallowed), is
// expected inside a legacy route family and a hidden dependency anywhere else.
func judgeWriteProbe(before, after []writeProbeResult) writeProbeVerdict {
	v := writeProbeVerdict{reached: map[string]bool{}, statuses: map[string][]int{}}
	if len(before) != len(after) {
		v.mismatch = fmt.Sprintf("the two runs issued %d and %d calls: the call sequence must not depend on the database", len(before), len(after))
		return v
	}
	for i, b := range before {
		a := after[i]
		c := b.call
		if a.call.route != c.route || a.call.caller != c.caller {
			v.mismatch = fmt.Sprintf("call %d is %s as %s in one run and %s as %s in the other", i, c.route, c.caller, a.call.route, a.call.caller)
			return v
		}
		v.statuses[c.route] = append(v.statuses[c.route], b.status)
		if b.status >= 200 && b.status < 300 {
			v.reached[c.route] = true
		}
		if b.status >= 500 || b.status == http.StatusTooManyRequests {
			v.gated = append(v.gated, fmt.Sprintf("%s %s as %s: %d with the legacy tables present", c.route, c.path, c.caller, b.status))
			continue
		}
		if a.status == http.StatusTooManyRequests {
			v.limited = append(v.limited, fmt.Sprintf("%s as %s", c.route, c.caller))
			continue
		}
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
		line := fmt.Sprintf("%s %s (%s) as %s: %d -> %d, missing %v", c.route, c.path, b.family, c.caller, b.status, a.status, missing)
		if legacyRepositoryFamilies[b.family] {
			v.expected = append(v.expected, line)
			if c.route == "POST /v1/questions/{id}/answers" && slices.Contains(missing, "answers") {
				v.controlSeen = true
			}
			continue
		}
		v.hidden = append(v.hidden, line, fmt.Sprintf("    response: %.200s", strings.TrimSpace(a.body)))
		for _, e := range fresh {
			v.hidden = append(v.hidden, fmt.Sprintf("    %s %s | %.200s", e.Code, e.Message, strings.Join(strings.Fields(e.SQL), " ")))
		}
	}
	return v
}

func TestLegacyDroppedDatabase_WriteRoutesExposeOnlyLegacyRouteFamilies(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", routeProbeAdminKey)
	t.Setenv("GROQ_API_KEY", "")                   // no external moderation or translation calls
	t.Setenv("RESEND_API_KEY", "")                 // never send email
	t.Setenv("IPFS_API_URL", "http://127.0.0.1:9") // refuse IPFS at once on both databases

	presentURL := newMigratedScratchURL(t, "solvr_legacy_present_")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	presentTracer := &dbErrorTracer{}
	presentPool, err := db.NewPool(ctx, presentURL, db.WithQueryTracer(presentTracer))
	if err != nil {
		t.Fatalf("open present pool: %v", err)
	}
	t.Cleanup(presentPool.Close)
	presentState := seedWriteProbeData(ctx, t, presentPool)
	before, _ := runWriteProbe(t, newRouteProbeRouter(presentPool), presentTracer, presentState, 0)
	if len(presentState.captureFailures) > 0 {
		t.Errorf("with the legacy tables present every capturing call must succeed, or the calls after it only probe a missing id:\n%s",
			strings.Join(presentState.captureFailures, "\n"))
	}

	var droppedState *writeProbeState
	d := newLegacyDroppedDatabase(t, func(ctx context.Context, conn *pgx.Conn) {
		pool, err := db.NewPool(ctx, conn.Config().ConnString())
		if err != nil {
			t.Fatalf("open pre-drop pool: %v", err)
		}
		defer pool.Close()
		droppedState = seedWriteProbeData(ctx, t, pool)
	})
	after, late := runWriteProbe(t, newRouteProbeRouter(d.pool), d.tracer, droppedState, writeProbeDrain)

	v := judgeWriteProbe(before, after)
	if v.mismatch != "" {
		t.Fatal(v.mismatch)
	}
	for _, line := range v.limited {
		t.Errorf("%s was rate limited on the dropped database: the probe measured the limiter", line)
	}
	hidden, expected, gated, reached, statuses := v.hidden, v.expected, v.gated, v.reached, v.statuses
	for _, e := range late {
		if table, ok := missingLegacyObject(e); ok {
			hidden = append(hidden, fmt.Sprintf("background work after the last call reached dropped %s: %s %s | %.200s",
				table, e.Code, e.Message, strings.Join(strings.Fields(e.SQL), " ")))
		}
	}

	// Coverage: every write route is called, and every canonical one completes at least once
	// with the legacy tables present, unless it is listed as validation-only with a reason.
	called := map[string]bool{}
	for _, b := range before {
		called[b.call.route] = true
	}
	var uncalled, unreached, stale []string
	for _, f := range api.RouteFamilies {
		for _, route := range f.Routes {
			if strings.HasPrefix(route, "GET ") {
				continue
			}
			if !called[route] {
				uncalled = append(uncalled, route)
				continue
			}
			_, validationOnly := writeProbeValidationOnly[route]
			switch {
			case legacyRepositoryFamilies[f.Name]:
			case !reached[route] && !validationOnly:
				unreached = append(unreached, fmt.Sprintf("%s %v", route, statuses[route]))
			case reached[route] && validationOnly:
				stale = append(stale, route)
			}
		}
	}
	if len(uncalled) > 0 {
		t.Errorf("write routes the probe never calls:\n%s", strings.Join(uncalled, "\n"))
	}
	if len(unreached) > 0 {
		t.Errorf("canonical write routes no call completed with the legacy tables present (the probe ran their validation only):\n%s",
			strings.Join(unreached, "\n"))
	}
	if len(stale) > 0 {
		t.Errorf("routes listed as validation-only now complete; drop them from writeProbeValidationOnly:\n%s", strings.Join(stale, "\n"))
	}
	sort.Strings(gated)
	t.Logf("%d write calls over %d write routes; %d answered 5xx with the legacy tables present and are not judged:\n%s",
		len(before), len(called), len(gated), strings.Join(gated, "\n"))
	t.Logf("%d calls in legacy route families reach dropped legacy tables:\n%s", len(expected), strings.Join(expected, "\n"))
	if len(hidden) > 0 {
		t.Errorf("hidden legacy dependencies: served outside the legacy route families, these fail once the legacy tables are gone:\n%s",
			strings.Join(hidden, "\n"))
	}

	// The legacy writes are retired (idx 52): on both databases every call to one answers the
	// migration error and raises no database error.
	retired := map[string]bool{}
	for _, r := range api.LegacyWriteRetirements {
		retired[r.Route] = true
	}
	var notRetired []string
	for run, results := range map[string][]writeProbeResult{"present": before, "dropped": after} {
		for _, r := range results {
			if !retired[r.call.route] {
				continue
			}
			if r.status != http.StatusGone || !strings.Contains(r.body, `"code":"`+api.ErrCodeEndpointRetired+`"`) || len(r.errs) > 0 {
				notRetired = append(notRetired, fmt.Sprintf("%s %s as %s on the %s database: %d, %d database errors, %.200s",
					r.call.route, r.call.path, r.call.caller, run, r.status, len(r.errs), strings.TrimSpace(r.body)))
			}
		}
	}
	if len(notRetired) > 0 {
		t.Errorf("retired legacy write routes must answer 410 %s and touch no table:\n%s",
			api.ErrCodeEndpointRetired, strings.Join(notRetired, "\n"))
	}

	// Positive control: the dropped database really lacks the legacy tables, and its tracer
	// (the one every call above was judged by) sees a statement that reaches for one.
	d.tracer.take()
	if _, err := d.pool.Exec(ctx, "SELECT 1 FROM answers LIMIT 1"); err == nil {
		t.Errorf("positive control: the answers table still exists on the dropped database")
	}
	_, controlErrs := d.tracer.take()
	controlSeen := false
	for _, e := range controlErrs {
		if table, ok := missingLegacyObject(e); ok && table == "answers" {
			controlSeen = true
		}
	}
	if !controlSeen {
		t.Errorf("positive control: the dropped database's tracer must record the missing answers table, got %v", controlErrs)
	}
}

// The judge on synthetic runs, no database: each clause of the verdict, including the one no
// live call exercises today (a canonical write that swallows a missing-table error into 2xx).
func TestJudgeWriteProbe_EachClause(t *testing.T) {
	missingAnswers := tracedError{Code: "42P01", Message: `relation "answers" does not exist`, SQL: "SELECT 1 FROM answers"}
	uniqueViolation := tracedError{Code: "23505", Message: `duplicate key value violates unique constraint "bookmarks_pkey"`}
	result := func(route, family string, status int, errs ...tracedError) writeProbeResult {
		return writeProbeResult{call: writeProbeCall{route: route, caller: "agent", path: "/x"}, family: family, status: status, errs: errs}
	}
	before := []writeProbeResult{
		result("DELETE /v1/me", "user-accounts", 200),                            // swallowed on the dropped side
		result("POST /v1/posts", "canonical-posts", 201),                         // 5xx on the dropped side
		result("POST /v1/users/me/bookmarks", "bookmarks", 409, uniqueViolation), // same error on both sides
		result("POST /v1/reports", "reports", 400),                               // unchanged
		result("POST /v1/questions/{id}/answers", "legacy-typed-writes", 201),    // legacy family
		result("POST /v1/add", "storage", 500),                                   // gated
		result("POST /v1/follow", "follows", 201),                                // limiter
	}
	after := []writeProbeResult{
		result("DELETE /v1/me", "user-accounts", 200, missingAnswers),
		result("POST /v1/posts", "canonical-posts", 500),
		result("POST /v1/users/me/bookmarks", "bookmarks", 409, uniqueViolation),
		result("POST /v1/reports", "reports", 400),
		result("POST /v1/questions/{id}/answers", "legacy-typed-writes", 500, missingAnswers),
		result("POST /v1/add", "storage", 500, missingAnswers),
		result("POST /v1/follow", "follows", 429),
	}
	v := judgeWriteProbe(before, after)
	if v.mismatch != "" {
		t.Fatal(v.mismatch)
	}
	var hiddenRoutes []string
	for _, line := range v.hidden {
		if !strings.HasPrefix(line, " ") {
			hiddenRoutes = append(hiddenRoutes, strings.SplitN(line, " /x", 2)[0])
		}
	}
	if want := []string{"DELETE /v1/me", "POST /v1/posts"}; !slices.Equal(hiddenRoutes, want) {
		t.Errorf("hidden = %v, want %v (full: %v)", hiddenRoutes, want, v.hidden)
	}
	if len(v.expected) != 1 || !strings.HasPrefix(v.expected[0], "POST /v1/questions/{id}/answers") || !v.controlSeen {
		t.Errorf("the legacy-family call must be expected and satisfy the positive control: %v, control %v", v.expected, v.controlSeen)
	}
	if len(v.gated) != 1 || !strings.HasPrefix(v.gated[0], "POST /v1/add") {
		t.Errorf("gated = %v, want the call that was 5xx with the legacy tables present", v.gated)
	}
	if len(v.limited) != 1 || v.limited[0] != "POST /v1/follow as agent" {
		t.Errorf("limited = %v", v.limited)
	}
	if !v.reached["DELETE /v1/me"] || v.reached["POST /v1/reports"] {
		t.Errorf("reached = %v", v.reached)
	}
	if v := judgeWriteProbe(before, after[:3]); v.mismatch == "" {
		t.Errorf("runs of different lengths must be refused")
	}
}
