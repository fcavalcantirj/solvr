package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Measuring aggregate API usage at the boundary.
//
// This middleware answers one question per request: was this a CONFIRMED
// APPLICATION REQUEST, and if so, what kind? Everything it hands on is an
// aggregate's raw material — a stable route template, the canonical operation
// that template maps to, the response class, the actor type and the instant.
// It never passes on a path, a slug, an id, a query string, a body, an
// address or a credential, because the counts built from it are published.
//
// Three exclusions keep the published figures honest:
//
//   - THE STATISTICS MAY NOT INFLATE THEMSELVES. The homepage overview
//     endpoints are the page reading its own numbers; counting them would
//     mean the figure grows every time somebody looks at it.
//   - INFRASTRUCTURE IS NOT USAGE. Health checks, internal probes, admin
//     queries, static assets, transport heartbeats and stream connections are
//     the system keeping itself alive, not agents and people doing work.
//   - A REQUEST THAT MATCHED NO ROUTE WAS NOT SERVED. A scanner hitting
//     /wp-login.php, a rate-limited request that never reached a handler and
//     a CORS preflight are not application calls.

// APIUsageRecorder accepts one confirmed application request. Implementations
// must not block the request that produced it.
type APIUsageRecorder interface {
	Record(event db.APIRequestEvent)
}

// apiUsageExcludedPrefixes are the route-template prefixes that are never
// counted, whatever method reached them.
var apiUsageExcludedPrefixes = []string{
	// Health checks and internal probes.
	"/health",
	"/v1/health",
	"/metrics",
	// Operator surfaces.
	"/admin",
	// The overview reading its own numbers.
	"/v1/homepage",
	// Static assets and crawler files.
	"/robots.txt",
	"/favicon.ico",
	"/sitemap",
	"/static",
	"/assets",
	"/_next",
}

// apiUsageExcludedSuffixes are transport concerns rather than operations: a
// stream is one connection held open, a stream ticket is the browser preparing
// to open one (minted again on every reconnect), and a heartbeat is a timer.
var apiUsageExcludedSuffixes = []string{
	"/stream",
	"/stream-ticket",
	"/heartbeat",
	"/events/stream",
}

// apiUsageOperations maps "METHOD template" to the canonical operation it is.
//
// Two ADAPTERS of one operation share a name on purpose: POST
// /v1/rooms/{slug}/messages and POST /r/{slug}/message are the same work
// reached two ways, and counting distinct operations must count distinct
// work rather than distinct URLs.
var apiUsageOperations = map[string]string{
	// Rooms, both adapters.
	"POST /v1/rooms":                             "room.create",
	"GET /v1/rooms":                              "room.list",
	"GET /v1/rooms/{slug}":                       "room.read",
	"PATCH /v1/rooms/{slug}":                     "room.update",
	"DELETE /v1/rooms/{slug}":                    "room.delete",
	"POST /v1/rooms/{slug}/messages":             "room.message.send",
	"POST /r/{slug}/message":                     "room.message.send",
	"GET /v1/rooms/{slug}/messages":              "room.message.read",
	"GET /r/{slug}/messages":                     "room.message.read",
	"GET /v1/rooms/{slug}/agents":                "room.participants.read",
	"GET /r/{slug}/agents":                       "room.participants.read",
	"GET /r/{slug}/agents/{agent_name}":          "room.participants.read",
	"POST /r/{slug}/join":                        "room.join",
	"POST /r/{slug}/leave":                       "room.leave",
	"POST /v1/rooms/{slug}/handshake":            "room.handshake",
	"GET /v1/rooms/{slug}/members":               "room.members.read",
	"POST /v1/rooms/{slug}/members":              "room.members.add",
	"DELETE /v1/rooms/{slug}/members/{agent_id}": "room.members.remove",
	"GET /v1/rooms/{slug}/connect":               "room.read",
	"POST /r/{slug}/claim":                       "room.claim",
	"POST /r/{slug}/claim/renew":                 "room.claim.renew",
	"POST /r/{slug}/claim/release":               "room.claim.release",
	"GET /r/{slug}/claims":                       "room.claims.read",
	"POST /r/{slug}/events":                      "room.event.send",
	"GET /r/{slug}/events":                       "room.events.read",

	// Knowledge.
	"GET /v1/search":         "knowledge.search",
	"POST /v1/posts":         "knowledge.post.create",
	"GET /v1/posts":          "knowledge.post.list",
	"GET /v1/posts/{id}":     "knowledge.post.read",
	"PATCH /v1/posts/{id}":   "knowledge.post.update",
	"DELETE /v1/posts/{id}":  "knowledge.post.delete",
	"GET /v1/problems":       "knowledge.post.list",
	"GET /v1/problems/{id}":  "knowledge.post.read",
	"GET /v1/questions":      "knowledge.post.list",
	"GET /v1/questions/{id}": "knowledge.post.read",
	"GET /v1/ideas":          "knowledge.post.list",
	"GET /v1/ideas/{id}":     "knowledge.post.read",
}

// apiUsageFamilyPrefixes files a route template under the family it belongs
// to. The list is ORDERED: the first prefix that matches wins, so a more
// specific prefix is placed before a broader one.
var apiUsageFamilyPrefixes = []struct {
	prefix, family string
}{
	{"/v1/rooms", db.APIFamilyRoom},
	{"/r/{slug}", db.APIFamilyRoom},
	{"/v1/search", db.APIFamilyKnowledge},
	{"/v1/posts", db.APIFamilyKnowledge},
	{"/v1/problems", db.APIFamilyKnowledge},
	{"/v1/questions", db.APIFamilyKnowledge},
	{"/v1/ideas", db.APIFamilyKnowledge},
	{"/v1/answers", db.APIFamilyKnowledge},
	{"/v1/approaches", db.APIFamilyKnowledge},
	{"/v1/responses", db.APIFamilyKnowledge},
	{"/v1/comments", db.APIFamilyKnowledge},
	{"/v1/progress-notes", db.APIFamilyKnowledge},
	{"/v1/feed", db.APIFamilyKnowledge},
	{"/v1/tags", db.APIFamilyKnowledge},
	{"/v1/mcp", db.APIFamilyKnowledge},
	{"/v1/agents", db.APIFamilyIdentity},
	{"/v1/auth", db.APIFamilyIdentity},
	{"/v1/users", db.APIFamilyIdentity},
	{"/v1/me", db.APIFamilyIdentity},
}

// APIUsage records every confirmed application request as aggregate call
// volume. A nil recorder turns the middleware into a pass-through.
func APIUsage(recorder APIUsageRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if recorder == nil {
				next.ServeHTTP(w, r)
				return
			}

			wrapped := &apiUsageWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			// The route pattern is only known once routing has happened, so
			// the decision is taken here rather than on the way in.
			template := apiUsageRouteTemplate(r)
			if !recordableAPIRequest(r.Method, template) {
				return
			}

			recorder.Record(db.APIRequestEvent{
				RequestID:       r.Header.Get("X-Request-ID"),
				RouteTemplate:   template,
				Method:          r.Method,
				Operation:       apiUsageOperation(r.Method, template),
				OperationFamily: apiUsageFamily(template),
				OperationKind:   apiUsageKind(r.Method, template),
				ActorType:       apiUsageActorType(r),
				StatusClass:     wrapped.status / 100,
				OccurredAt:      time.Now(),
			})
		})
	}
}

// apiUsageWriter captures the response class and nothing else.
type apiUsageWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *apiUsageWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *apiUsageWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}

// Flush keeps streaming handlers working: they assert on http.Flusher, and a
// wrapper that swallowed it would break SSE.
func (w *apiUsageWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// apiUsageRouteTemplate is the stable pattern the request matched, normalised
// so one route has one name. An unmatched request has no template at all.
func apiUsageRouteTemplate(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}

	template := rctx.RoutePattern()
	if template == "" {
		return ""
	}
	// chi reports a catch-all for a request that reached no route of its own.
	if strings.HasSuffix(template, "/*") {
		return ""
	}
	// A subrouter's index route reads "/v1/rooms/"; it is the same route as
	// "/v1/rooms" and must not be counted as a second operation.
	if len(template) > 1 {
		template = strings.TrimSuffix(template, "/")
	}
	return template
}

// recordableAPIRequest reports whether this is a confirmed application
// request worth counting as usage.
func recordableAPIRequest(method, template string) bool {
	if template == "" || template == "/" {
		return false
	}
	// A preflight is the browser asking permission, not an application call.
	if method == http.MethodOptions {
		return false
	}
	for _, prefix := range apiUsageExcludedPrefixes {
		if template == prefix || strings.HasPrefix(template, prefix+"/") {
			return false
		}
	}
	for _, suffix := range apiUsageExcludedSuffixes {
		if strings.HasSuffix(template, suffix) {
			return false
		}
	}
	return true
}

// apiUsageOperation names the work a request did. A route with no explicit
// name falls back to a stable name built from the template, which carries no
// identifier because a template carries none.
func apiUsageOperation(method, template string) string {
	if operation, ok := apiUsageOperations[method+" "+template]; ok {
		return operation
	}
	return strings.ToLower(method) + " " + template
}

// apiUsageFamily files a template under room, knowledge, identity or other.
func apiUsageFamily(template string) string {
	for _, entry := range apiUsageFamilyPrefixes {
		if template == entry.prefix || strings.HasPrefix(template, entry.prefix+"/") {
			return entry.family
		}
	}
	return db.APIFamilyOther
}

// apiUsageKind separates passive polling from the calls that make something
// happen. A read an agent repeats on a timer is a poll however useful it is;
// a search is neither a poll nor a write, because it is the call the whole
// knowledge base exists to serve.
func apiUsageKind(method, template string) string {
	if strings.Contains(template, "/search") {
		return db.APIOperationSearch
	}
	switch method {
	case http.MethodGet, http.MethodHead:
		return db.APIOperationPoll
	default:
		return db.APIOperationWrite
	}
}

// apiUsageActorType reads the actor from the credential the request
// presented. An unrecognised or absent credential is ANONYMOUS: the one
// direction this may never fail in is calling an unidentified caller a person.
func apiUsageActorType(r *http.Request) string {
	credential := apiUsageCredential(r)
	switch {
	case credential == "":
		return db.APIActorAnonymous
	case strings.HasPrefix(credential, "solvr_sk_"):
		// A personal API key belongs to a person.
		return db.APIActorHuman
	case strings.HasPrefix(credential, "solvr_"):
		// An agent key or a room token; both belong to an agent.
		return db.APIActorAgent
	case strings.Count(credential, ".") == 2:
		// A session token belongs to a signed-in person.
		return db.APIActorHuman
	default:
		return db.APIActorAnonymous
	}
}

// apiUsageCredential returns the credential presented, without storing,
// logging or publishing it. It reads the Authorization header ONLY, the one
// place the API honours a credential: a token in the query string authenticates
// nobody (the room routes refuse it), so it cannot say who the caller is.
func apiUsageCredential(r *http.Request) string {
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	return ""
}
