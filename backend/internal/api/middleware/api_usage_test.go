package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Measuring aggregate API usage at the boundary.
//
// The rules proven here are the ones that decide whether a published number
// is honest:
//
//   - a recorded row carries the stable route TEMPLATE and nothing that
//     identifies anybody — no slug, no id, no query string, no body;
//   - displaying the statistics never inflates them: health checks, admin
//     queries, internal probes, overview refreshes, transport heartbeats,
//     streams and preflights are not usage;
//   - a request that matched no application route is not a confirmed
//     application request;
//   - passive polling is recorded apart from create/send/search;
//   - two route adapters of one operation record the SAME canonical
//     operation, so counting distinct operations counts distinct work.

// recordingSpy captures what the middleware decided to record.
type recordingSpy struct {
	events []db.APIRequestEvent
}

func (s *recordingSpy) Record(event db.APIRequestEvent) {
	s.events = append(s.events, event)
}

// usageRouter builds a router shaped like the real one: the recording
// middleware wraps routes mounted under the same templates the API serves.
func usageRouter(spy *recordingSpy) *chi.Mux {
	r := chi.NewRouter()
	r.Use(APIUsage(spy))

	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	created := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }
	rejected := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }
	broken := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }

	r.Get("/health", ok)
	r.Get("/health/ready", ok)
	r.Get("/v1/health/ipfs", ok)
	r.Post("/admin/query", ok)
	r.Get("/v1/homepage/overview", ok)
	r.Get("/v1/homepage/rooms", ok)
	r.Get("/v1/homepage/api-usage", ok)
	r.Get("/robots.txt", ok)
	r.Get("/v1/heartbeat", ok)

	r.Get("/v1/search", ok)
	r.Post("/v1/posts", created)
	r.Get("/v1/posts", ok)
	r.Get("/v1/posts/{id}", ok)
	r.Post("/v1/agents/register", created)

	r.Route("/v1/rooms", func(r chi.Router) {
		r.Get("/", ok)
		r.Post("/", created)
		r.Get("/{slug}", ok)
		r.Get("/{slug}/messages", ok)
		r.Post("/{slug}/messages", created)
		r.Get("/{slug}/stream", ok)
		r.Delete("/{slug}", rejected)
		r.Patch("/{slug}", broken)
	})

	r.Route("/r/{slug}", func(r chi.Router) {
		r.Post("/message", created)
		r.Get("/messages", ok)
		r.Post("/heartbeat", ok)
		r.Get("/stream", ok)
	})

	return r
}

// call serves one request and returns everything recorded for it.
func call(t *testing.T, method, target string, headers map[string]string) (*recordingSpy, *httptest.ResponseRecorder) {
	t.Helper()
	spy := &recordingSpy{}
	req := httptest.NewRequest(method, target, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	usageRouter(spy).ServeHTTP(rec, req)
	return spy, rec
}

func TestAPIUsage_RecordsTheRouteTemplateNotTheConcretePath(t *testing.T) {
	spy, rec := call(t, http.MethodPost, "/v1/rooms/secret-launch-room/messages?token=solvr_rm_abc", map[string]string{
		"X-Request-ID":  "req-77",
		"Authorization": "Bearer solvr_live_agentkey",
	})

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, spy.events, 1)
	event := spy.events[0]

	assert.Equal(t, "/v1/rooms/{slug}/messages", event.RouteTemplate)
	assert.Equal(t, http.MethodPost, event.Method)
	assert.Equal(t, "req-77", event.RequestID)
	assert.Equal(t, db.APIActorAgent, event.ActorType)
	assert.Equal(t, db.APIFamilyRoom, event.OperationFamily)
	assert.Equal(t, db.APIOperationWrite, event.OperationKind)
	assert.Equal(t, 2, event.StatusClass)
	assert.False(t, event.OccurredAt.IsZero())

	// Nothing recorded may carry the room anybody could be identified by.
	for _, field := range []string{event.RouteTemplate, event.Operation, event.Method, event.ActorType} {
		assert.NotContains(t, field, "secret-launch-room")
		assert.NotContains(t, field, "solvr_")
		assert.NotContains(t, field, "?")
	}
}

// Displaying the dashboard must not inflate the dashboard, and neither may a
// health check, an admin query, a heartbeat or a stream.
func TestAPIUsage_DoesNotCountItsOwnDisplayOrTheInfrastructure(t *testing.T) {
	excluded := []struct{ method, target string }{
		{http.MethodGet, "/health"},
		{http.MethodGet, "/health/ready"},
		{http.MethodGet, "/v1/health/ipfs"},
		{http.MethodPost, "/admin/query"},
		{http.MethodGet, "/v1/homepage/overview"},
		{http.MethodGet, "/v1/homepage/rooms"},
		{http.MethodGet, "/v1/homepage/api-usage"},
		{http.MethodGet, "/robots.txt"},
		{http.MethodGet, "/v1/heartbeat"},
		{http.MethodPost, "/r/some-room/heartbeat"},
		{http.MethodGet, "/r/some-room/stream"},
		{http.MethodGet, "/v1/rooms/some-room/stream"},
	}

	for _, c := range excluded {
		t.Run(c.method+" "+c.target, func(t *testing.T) {
			spy, _ := call(t, c.method, c.target, nil)
			assert.Empty(t, spy.events, "%s %s must not count as API usage", c.method, c.target)
		})
	}
}

// A request that matched no application route was not served by the
// application, so it is not a confirmed application request.
func TestAPIUsage_IgnoresRequestsThatMatchedNoRoute(t *testing.T) {
	spy, rec := call(t, http.MethodGet, "/wp-login.php", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, spy.events, "a scanner hitting nothing is not usage")
}

// A CORS preflight is the browser asking permission, not an application call.
func TestAPIUsage_IgnoresPreflight(t *testing.T) {
	spy, _ := call(t, http.MethodOptions, "/v1/posts", nil)
	assert.Empty(t, spy.events)
}

func TestAPIUsage_ReadsTheActorFromTheCredentialPresented(t *testing.T) {
	cases := []struct {
		name, header, expected string
	}{
		{"agent api key", "Bearer solvr_live_agentkey", db.APIActorAgent},
		{"shared room token", "Bearer solvr_rm_sharedtoken", db.APIActorAgent},
		{"per-agent room token", "Bearer solvr_rt_agenttoken", db.APIActorAgent},
		{"personal api key", "Bearer solvr_sk_personalkey", db.APIActorHuman},
		{"session token", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig", db.APIActorHuman},
		{"no credential", "", db.APIActorAnonymous},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			headers := map[string]string{}
			if c.header != "" {
				headers["Authorization"] = c.header
			}
			spy, _ := call(t, http.MethodGet, "/v1/posts", headers)
			require.Len(t, spy.events, 1)
			assert.Equal(t, c.expected, spy.events[0].ActorType)
		})
	}
}

// An unauthenticated caller is never reported as a person.
func TestAPIUsage_NeverCallsAnUnauthenticatedCallerHuman(t *testing.T) {
	spy, _ := call(t, http.MethodGet, "/v1/search?q=timeout", nil)
	require.Len(t, spy.events, 1)
	assert.Equal(t, db.APIActorAnonymous, spy.events[0].ActorType)
}

func TestAPIUsage_SeparatesPassivePollingFromCreateSendAndSearch(t *testing.T) {
	cases := []struct {
		method, target, kind string
	}{
		{http.MethodGet, "/v1/rooms/a-room/messages", db.APIOperationPoll},
		{http.MethodGet, "/v1/posts", db.APIOperationPoll},
		{http.MethodPost, "/v1/rooms/a-room/messages", db.APIOperationWrite},
		{http.MethodPost, "/v1/posts", db.APIOperationWrite},
		{http.MethodGet, "/v1/search?q=deadlock", db.APIOperationSearch},
	}

	for _, c := range cases {
		t.Run(c.method+" "+c.target, func(t *testing.T) {
			spy, _ := call(t, c.method, c.target, nil)
			require.Len(t, spy.events, 1)
			assert.Equal(t, c.kind, spy.events[0].OperationKind)
		})
	}
}

func TestAPIUsage_FilesEveryRouteUnderAFamily(t *testing.T) {
	cases := []struct {
		method, target, family string
	}{
		{http.MethodPost, "/v1/rooms", db.APIFamilyRoom},
		{http.MethodPost, "/r/a-room/message", db.APIFamilyRoom},
		{http.MethodGet, "/v1/search?q=x", db.APIFamilyKnowledge},
		{http.MethodPost, "/v1/posts", db.APIFamilyKnowledge},
		{http.MethodPost, "/v1/agents/register", db.APIFamilyIdentity},
	}

	for _, c := range cases {
		t.Run(c.method+" "+c.target, func(t *testing.T) {
			spy, _ := call(t, c.method, c.target, nil)
			require.Len(t, spy.events, 1)
			assert.Equal(t, c.family, spy.events[0].OperationFamily)
		})
	}
}

// One operation reached through two route adapters is one operation.
func TestAPIUsage_GivesTwoAdaptersOfOneOperationTheSameName(t *testing.T) {
	rest, _ := call(t, http.MethodPost, "/v1/rooms/a-room/messages", nil)
	a2a, _ := call(t, http.MethodPost, "/r/a-room/message", nil)

	require.Len(t, rest.events, 1)
	require.Len(t, a2a.events, 1)
	assert.Equal(t, rest.events[0].Operation, a2a.events[0].Operation,
		"sending a message is one operation however it was reached")
	assert.NotEqual(t, rest.events[0].RouteTemplate, a2a.events[0].RouteTemplate,
		"the two adapters are still recorded as the templates they are")

	restRead, _ := call(t, http.MethodGet, "/v1/rooms/a-room/messages", nil)
	a2aRead, _ := call(t, http.MethodGet, "/r/a-room/messages", nil)
	require.Len(t, restRead.events, 1)
	require.Len(t, a2aRead.events, 1)
	assert.Equal(t, restRead.events[0].Operation, a2aRead.events[0].Operation)
	assert.NotEqual(t, rest.events[0].Operation, restRead.events[0].Operation,
		"sending and reading are different operations")
}

// Every operation name is a stable name, never a URL carrying an identifier.
func TestAPIUsage_NamesOperationsWithoutIdentifiers(t *testing.T) {
	for _, target := range []string{
		"/v1/rooms/launch-room", "/v1/posts/8b1f0a2c", "/r/launch-room/messages",
	} {
		spy, _ := call(t, http.MethodGet, target, nil)
		require.Len(t, spy.events, 1, target)
		assert.NotContains(t, spy.events[0].Operation, "launch-room")
		assert.NotContains(t, spy.events[0].Operation, "8b1f0a2c")
		assert.NotEmpty(t, spy.events[0].Operation)
	}
}

func TestAPIUsage_RecordsTheResponseClass(t *testing.T) {
	cases := []struct {
		method, target string
		class          int
	}{
		{http.MethodGet, "/v1/posts", 2},
		{http.MethodDelete, "/v1/rooms/a-room", 4},
		{http.MethodPatch, "/v1/rooms/a-room", 5},
	}

	for _, c := range cases {
		t.Run(c.method+" "+c.target, func(t *testing.T) {
			spy, _ := call(t, c.method, c.target, nil)
			require.Len(t, spy.events, 1)
			assert.Equal(t, c.class, spy.events[0].StatusClass)
		})
	}
}

// The request identifier is what makes counting idempotent, so it is carried
// through exactly as it arrived.
func TestAPIUsage_CarriesTheIncomingRequestIdentifier(t *testing.T) {
	spy, _ := call(t, http.MethodGet, "/v1/posts", map[string]string{"X-Request-ID": "abc-123"})
	require.Len(t, spy.events, 1)
	assert.Equal(t, "abc-123", spy.events[0].RequestID)

	without, _ := call(t, http.MethodGet, "/v1/posts", nil)
	require.Len(t, without.events, 1)
	assert.Empty(t, without.events[0].RequestID,
		"a request with no identifier is still counted; it simply cannot be deduplicated")
}

// A route template chi reports with a trailing slash is the same route.
func TestAPIUsageRouteTemplate_IsNormalised(t *testing.T) {
	spy, _ := call(t, http.MethodGet, "/v1/rooms", nil)
	require.Len(t, spy.events, 1)
	assert.Equal(t, "/v1/rooms", spy.events[0].RouteTemplate)
	assert.False(t, strings.HasSuffix(spy.events[0].RouteTemplate, "/"))
}

// Without a recorder there is nothing to record, and the middleware must stay
// out of the way rather than failing the request.
func TestAPIUsage_WithoutARecorderServesTheRequest(t *testing.T) {
	r := chi.NewRouter()
	r.Use(APIUsage(nil))
	r.Get("/v1/posts", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/posts", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}
