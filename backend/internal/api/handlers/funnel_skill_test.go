package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// skill_fetched (SPEC.md 25.7): the web server reports that the skill link of a copied
// sentence (skill.md?f=<flow code>) was fetched. The API decides everything about the
// step: it needs a well-formed flow code, it sets the entry surface itself from how the
// skill was requested, and it stores nothing else a client sends.

func newSkillFunnelHandler(t *testing.T) (*FunnelHandler, *db.FunnelEventRepository) {
	t.Helper()
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	repo := db.NewFunnelEventRepository(pool)
	return NewFunnelHandler(repo), repo
}

// freshFlowCode is a code no earlier step carries, removed again after the test.
func freshFlowCode(t *testing.T, pool *db.Pool) string {
	t.Helper()
	code := newFlowID()
	require.True(t, models.ValidFlowCode(code))
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM funnel_events WHERE flow_id = $1", code) //nolint:errcheck
	})
	return code
}

func funnelError(t *testing.T, rec *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	var out struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out.Error.Code, out.Error.Message
}

func TestFunnelIngest_SkillFetchedIsRecordedOnTheWebServerChannel(t *testing.T) {
	h, repo := newSkillFunnelHandler(t)
	flow := freshFlowCode(t, getTestPool(t))

	// The flow is unknown (an agent that read GET /v1/connect itself has no earlier step):
	// the step is recorded all the same.
	rec := ingest(t, h, `{"event":"skill_fetched","flow_id":"`+flow+`","request_mode":""}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"data":{"recorded":true}}`, rec.Body.String())

	events, err := repo.ListByFlow(context.Background(), flow)
	require.NoError(t, err)
	require.Len(t, events, 1)
	e := events[0]
	require.Equal(t, models.FunnelSkillFetched, e.EventName)
	require.Equal(t, "web_server", e.SourceChannel)
	require.Equal(t, "agent_fetch", e.EntrySurface)
	require.Equal(t, models.FunnelActorAnonymous, e.ActorType)
	require.Empty(t, e.ActorRef)
	require.Empty(t, e.RoomID)
}

// The API alone decides the entry surface: a person who opened the link in a browser
// (Sec-Fetch-Mode: navigate) is a browser visit, everything else is an agent's fetch.
func TestFunnelIngest_SkillFetchedSurfaceIsDecidedByTheAPI(t *testing.T) {
	h, repo := newSkillFunnelHandler(t)
	for mode, want := range map[string]string{
		"navigate":             "browser_visit",
		"":                     "agent_fetch",
		"cors":                 "agent_fetch",
		"no-cors":              "agent_fetch",
		"same-origin":          "agent_fetch",
		"NAVIGATE":             "agent_fetch", // browsers send it in lower case; nothing else is a navigation
		"navigate ":            "agent_fetch",
		"exactly-twenty-chars": "agent_fetch",
	} {
		flow := freshFlowCode(t, getTestPool(t))
		body, _ := json.Marshal(map[string]string{"event": "skill_fetched", "flow_id": flow, "request_mode": mode})
		rec := ingest(t, h, string(body))
		require.Equal(t, http.StatusAccepted, rec.Code, "request_mode %q: %s", mode, rec.Body.String())
		events, err := repo.ListByFlow(context.Background(), flow)
		require.NoError(t, err)
		require.Len(t, events, 1, "request_mode %q", mode)
		require.Equal(t, want, events[0].EntrySurface, "request_mode %q", mode)
	}

	// request_mode may be left out altogether.
	flow := freshFlowCode(t, getTestPool(t))
	rec := ingest(t, h, `{"event":"skill_fetched","flow_id":"`+flow+`"}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	events, err := repo.ListByFlow(context.Background(), flow)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "agent_fetch", events[0].EntrySurface)
}

// A bot is not an agent. The web server also reports the request's User-Agent; the API
// reads it after the navigation check, so the preview a chat app builds for a pasted
// sentence, or a crawler that follows the link, is a bot_fetch and not an agent's read.
func TestFunnelIngest_SkillFetchedByABotIsABotFetch(t *testing.T) {
	h, repo := newSkillFunnelHandler(t)
	const chrome = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
	for _, tc := range []struct{ mode, userAgent, want string }{
		{"", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", "bot_fetch"},
		{"", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "bot_fetch"},
		{"cors", "WhatsApp/2.23.20.0 A", "bot_fetch"},
		{"", "curl/8.7.1", "agent_fetch"},
		{"", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot", "agent_fetch"},
		{"", "", "agent_fetch"},
		{"navigate", chrome, "browser_visit"},
		{"navigate", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", "browser_visit"},
	} {
		flow := freshFlowCode(t, getTestPool(t))
		body, _ := json.Marshal(map[string]string{
			"event": "skill_fetched", "flow_id": flow, "request_mode": tc.mode, "user_agent": tc.userAgent,
		})
		rec := ingest(t, h, string(body))
		require.Equal(t, http.StatusAccepted, rec.Code, "%+v: %s", tc, rec.Body.String())
		events, err := repo.ListByFlow(context.Background(), flow)
		require.NoError(t, err)
		require.Len(t, events, 1, "%+v", tc)
		require.Equal(t, tc.want, events[0].EntrySurface, "%+v", tc)
		require.Equal(t, "web_server", events[0].SourceChannel)
	}
}

// The user agent is read for that one decision and never stored: no column of the row
// holds any part of it.
func TestFunnelIngest_SkillFetchedNeverStoresTheUserAgent(t *testing.T) {
	h, _ := newSkillFunnelHandler(t)
	pool := getTestPool(t)
	flow := freshFlowCode(t, pool)

	rec := ingest(t, h, `{"event":"skill_fetched","flow_id":"`+flow+`","request_mode":"cors",`+
		`"user_agent":"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots) marker-9f3k"}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())

	var row string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT row_to_json(f)::text FROM funnel_events f WHERE flow_id = $1`, flow).Scan(&row))
	require.Contains(t, row, `"entry_surface":"bot_fetch"`)
	lower := strings.ToLower(row)
	for _, part := range []string{"slackbot", "linkexpanding", "api.slack.com", "marker-9f3k", "cors"} {
		require.NotContains(t, lower, part, "the row holds nothing of the user agent or the request mode: %s", row)
	}
}

// user_agent is bounded like every client string: the web server cuts it to 200
// characters, and a longer one is refused before the store is touched.
func TestFunnelIngest_SkillFetchedRefusesALongUserAgent(t *testing.T) {
	h := NewFunnelHandler(nil)
	for _, userAgent := range []string{strings.Repeat("u", 201), strings.Repeat("é", 201), "Slackbot " + strings.Repeat("x", 500)} {
		body, _ := json.Marshal(map[string]string{"event": "skill_fetched", "flow_id": "k7m2p9xq", "user_agent": userAgent})
		rec := ingest(t, h, string(body))
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		code, message := funnelError(t, rec)
		require.Equal(t, "VALIDATION_ERROR", code)
		require.Contains(t, message, "user_agent")
	}
}

// The bound is 200 characters, not bytes: a user agent of 200 two-byte letters is the
// longest the web server sends, and it is accepted.
func TestFunnelIngest_SkillFetchedAcceptsAUserAgentOfExactly200Characters(t *testing.T) {
	h, repo := newSkillFunnelHandler(t)
	for userAgent, want := range map[string]string{
		strings.Repeat("u", 200):                     "agent_fetch",
		strings.Repeat("é", 200):                     "agent_fetch",
		strings.Repeat("é", 190) + "Twitterbot":      "bot_fetch",
		"Twitterbot/1.0 " + strings.Repeat("x", 185): "bot_fetch",
	} {
		require.Equal(t, 200, len([]rune(userAgent)))
		flow := freshFlowCode(t, getTestPool(t))
		body, _ := json.Marshal(map[string]string{"event": "skill_fetched", "flow_id": flow, "user_agent": userAgent})
		rec := ingest(t, h, string(body))
		require.Equal(t, http.StatusAccepted, rec.Code, "%d characters: %s", len([]rune(userAgent)), rec.Body.String())
		events, err := repo.ListByFlow(context.Background(), flow)
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, want, events[0].EntrySurface)
	}
}

// Whatever else a client sends with the step is ignored: its own entry_surface above all,
// and every attribute the step does not have. The raw request_mode is never stored.
func TestFunnelIngest_SkillFetchedIgnoresWhatTheClientClaims(t *testing.T) {
	h, repo := newSkillFunnelHandler(t)
	flow := freshFlowCode(t, getTestPool(t))

	rec := ingest(t, h, `{"event":"skill_fetched","flow_id":"`+flow+`","request_mode":"navigate",`+
		`"entry_surface":"connect_page","preset":"plan-and-build","role":"planner","instruction_version":"9.9",`+
		`"source":{"kind":"room","ref":"some-room"}}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())

	events, err := repo.ListByFlow(context.Background(), flow)
	require.NoError(t, err)
	require.Len(t, events, 1)
	e := events[0]
	require.Equal(t, "browser_visit", e.EntrySurface, "the API's surface, not the client's")
	require.Empty(t, e.Preset)
	require.Empty(t, e.Role)
	require.Empty(t, e.InstructionVersion)
	require.Empty(t, e.SourceKind)
	require.Empty(t, e.SourceID)
	stored, err := json.Marshal(e)
	require.NoError(t, err)
	require.NotContains(t, string(stored), "navigate", "request_mode is read, never stored")

	// An entry_surface too long for any other step is ignored too, not refused.
	other := freshFlowCode(t, getTestPool(t))
	rec = ingest(t, h, `{"event":"skill_fetched","flow_id":"`+other+`","entry_surface":"`+strings.Repeat("x", 200)+`"}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
}

// flow_id is required and must be a flow code. These refusals happen before the store is
// touched, so they need no database.
func TestFunnelIngest_SkillFetchedNeedsAWellFormedFlowID(t *testing.T) {
	h := NewFunnelHandler(nil)
	for _, body := range []string{
		`{"event":"skill_fetched"}`,
		`{"event":"skill_fetched","flow_id":""}`,
		`{"event":"skill_fetched","flow_id":"f_0123456789abcdef01234567"}`,
		`{"event":"skill_fetched","flow_id":"K7M2P9XQ"}`,
		`{"event":"skill_fetched","flow_id":"k7m2p9x"}`,
		`{"event":"skill_fetched","flow_id":"k7m2p9xqq"}`,
		`{"event":"skill_fetched","flow_id":"k7m2p9xi"}`,
		`{"event":"skill_fetched","flow_id":"k7m2p9xq\n"}`,
		`{"event":"skill_fetched","flow_id":"` + strings.Repeat("a", 65) + `"}`,
	} {
		rec := ingest(t, h, body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s -> %s", body, rec.Body.String())
		code, message := funnelError(t, rec)
		require.Equal(t, "VALIDATION_ERROR", code, body)
		require.Contains(t, message, "flow_id", body)
	}
}

func TestFunnelIngest_SkillFetchedRefusesALongRequestMode(t *testing.T) {
	h := NewFunnelHandler(nil)
	rec := ingest(t, h, `{"event":"skill_fetched","flow_id":"k7m2p9xq","request_mode":"`+strings.Repeat("m", 21)+`"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, message := funnelError(t, rec)
	require.Equal(t, "VALIDATION_ERROR", code)
	require.Contains(t, message, "request_mode")
}

// The refusal of an unknown or server-only step names exactly what a client may report,
// from the contract itself.
func TestFunnelIngest_TheRefusalNamesEveryStepAClientMayReport(t *testing.T) {
	h := NewFunnelHandler(nil)
	for _, event := range []string{"totally_made_up", "room_created", "participant_joined", "first_two_way_exchange", ""} {
		rec := ingest(t, h, `{"event":"`+event+`","flow_id":"k7m2p9xq"}`)
		require.Equal(t, http.StatusBadRequest, rec.Code, event)
		code, message := funnelError(t, rec)
		require.Equal(t, "VALIDATION_ERROR", code)
		require.Equal(t, "event must be one of connection_started, starter_prompt_copied, skill_fetched, "+
			"room_viewed, join_prompt_copied, share_visit, share_link_copied", message)
	}
}

// request_mode and user_agent belong to skill_fetched alone: another step is not refused
// for carrying them, whatever their length or content, and its own entry_surface is still
// the client's.
func TestFunnelIngest_OtherStepsKeepTheirOwnSurface(t *testing.T) {
	h, repo := newSkillFunnelHandler(t)
	flow := freshFlowCode(t, getTestPool(t))
	rec := ingest(t, h, `{"event":"connection_started","flow_id":"`+flow+`","entry_surface":"connect_page","request_mode":"`+strings.Repeat("m", 40)+
		`","user_agent":"Slackbot-LinkExpanding 1.0 `+strings.Repeat("x", 500)+`"}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	events, err := repo.ListByFlow(context.Background(), flow)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "browser", events[0].SourceChannel)
	require.Equal(t, "connect_page", events[0].EntrySurface)
}
