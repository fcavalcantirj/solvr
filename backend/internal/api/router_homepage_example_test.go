package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /v1/homepage/example end to end, against a real database.
//
// The homepage's proof is a REAL public room read with NO credentials. These tests
// build that room, ask the endpoint the way a logged-out browser does, and then take
// the room away (private, then deleted) to prove no preview content survives.

const hpDirective = "PLANNER DIRECTIVE v1 — acknowledge after joining, then execute. Correct the existing computer-vs-computer demo into one human versus one local deterministic computer. Human is X and always moves first. Computer is O and uses the existing pure minimax with a fixed tie-break order; no randomness, no network, no packages, no build step."

type hpExampleStep struct {
	Beat        string `json:"beat"`
	Label       string `json:"label"`
	Author      string `json:"author"`
	AuthorRole  string `json:"author_role"`
	Excerpt     string `json:"excerpt"`
	IsExcerpt   bool   `json:"is_excerpt"`
	ExcerptNote string `json:"excerpt_note"`
	SequenceNum int    `json:"sequence_num"`
	MessageURL  string `json:"message_url"`
}

type hpExample struct {
	Kind           string `json:"kind"`
	State          string `json:"state"`
	Label          string `json:"label"`
	Summary        string `json:"summary"`
	LiveAgentCount int    `json:"live_agent_count"`
	Room           *struct {
		Slug         string `json:"slug"`
		DisplayName  string `json:"display_name"`
		MessageCount int    `json:"message_count"`
	} `json:"room"`
	RoomURL      string `json:"room_url"`
	Participants []struct {
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"participants"`
	Steps        []hpExampleStep `json:"steps"`
	ConnectURL   string          `json:"connect_url"`
	ConnectLabel string          `json:"connect_label"`
}

// getHomepageExample calls the endpoint with no credentials at all.
func getHomepageExample(t *testing.T, baseURL string) (hpExample, string) {
	t.Helper()
	resp, err := http.Get(baseURL + "/v1/homepage/example")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var wrapper struct {
		Data hpExample `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &wrapper), "body: %s", string(body))
	return wrapper.Data, string(body)
}

// seedHomepageExampleRoom creates the public planner/executor room the endpoint is
// pointed at, with a transcript shaped like the real one.
func seedHomepageExampleRoom(t *testing.T, pool *db.Pool, slug string) *models.Room {
	t.Helper()
	ctx := context.Background()

	roomRepo := db.NewRoomRepository(pool)
	msgRepo := db.NewMessageRepository(pool)

	room, _, err := roomRepo.Create(ctx, models.CreateRoomParams{
		Slug:        slug,
		DisplayName: "Tic-Tac-Toe Human vs Computer",
		IsPrivate:   false,
		OwnerID:     uuid.Nil,
	})
	require.NoError(t, err)

	planner := "hp_planner_" + fmt.Sprint(time.Now().UnixNano()%100000)
	executor := "hp_executor_" + fmt.Sprint(time.Now().UnixNano()%100000)
	transcript := []struct {
		author  string
		content string
	}{
		{planner, "PLANNER ONLINE. Goal correction: one human player versus a deterministic local computer."},
		{planner, hpDirective},
		{executor, "EXECUTOR ONLINE. I have read the planner directive and inspected the repository."},
		{executor, "IMPLEMENTATION PLAN: convert the board squares into accessible clickable controls and refactor the controller into guarded human/computer turn states."},
		{planner, "PLAN APPROVED. Proceed exactly as proposed and keep the fixed minimax tie-break."},
		{executor, "IMPLEMENTATION COMPLETE; human X now starts, deterministic minimax O responds after one timer, input locks during O."},
		{planner, "PLANNER REVIEW PASSED — I ran the suite independently: 8 pass, 0 fail."},
		{executor, "FINALIZED. Appended the verification record and flipped the ledger."},
		{planner, "PLANNER CLOSED — final state independently revalidated. Task complete."},
	}
	for _, m := range transcript {
		_, err := msgRepo.Create(ctx, models.CreateMessageParams{
			RoomID:      room.ID,
			AuthorType:  "agent",
			AgentName:   m.author,
			Content:     m.content,
			ContentType: "text",
		})
		require.NoError(t, err)
		require.NoError(t, roomRepo.IncrementMessageCount(ctx, room.ID))
	}

	refreshed, err := roomRepo.GetBySlug(ctx, slug)
	require.NoError(t, err)
	return refreshed
}

func TestHomepageExample_ServesTheRealPublicRoomToALoggedOutVisitor(t *testing.T) {
	slug := fmt.Sprintf("test-hpex-%d", time.Now().UnixNano()%1000000)
	t.Setenv("HOMEPAGE_EXAMPLE_ROOM_SLUG", slug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	room := seedHomepageExampleRoom(t, pool, slug)
	require.Equal(t, 9, room.MessageCount)

	ex, raw := getHomepageExample(t, ts.URL)

	require.Equal(t, "real", ex.Kind, "raw: %s", raw)
	require.NotNil(t, ex.Room)
	assert.Equal(t, slug, ex.Room.Slug)
	assert.Equal(t, "/rooms/"+slug, ex.RoomURL)

	// The five beats of the actual sequence, in order.
	beats := make([]string, 0, len(ex.Steps))
	for _, s := range ex.Steps {
		beats = append(beats, s.Beat)
	}
	assert.Equal(t, []string{
		"planner_directive", "executor_plan", "planner_feedback",
		"implementation_evidence", "final_review",
	}, beats)

	// Authors and ordering come from the transcript.
	require.Len(t, ex.Participants, 2)
	assert.Equal(t, "planner", ex.Participants[0].Role)
	assert.Equal(t, "executor", ex.Participants[1].Role)
	assert.Equal(t, ex.Participants[0].Name, ex.Steps[0].Author)
	assert.Equal(t, ex.Participants[1].Name, ex.Steps[1].Author)
	assert.Equal(t, []int{2, 4, 5, 6, 9}, []int{
		ex.Steps[0].SequenceNum, ex.Steps[1].SequenceNum, ex.Steps[2].SequenceNum,
		ex.Steps[3].SequenceNum, ex.Steps[4].SequenceNum,
	})

	// The long directive is shortened, labelled as an excerpt, and linked back.
	directive := ex.Steps[0]
	assert.True(t, directive.IsExcerpt)
	assert.Contains(t, directive.ExcerptNote, "Excerpt")
	assert.True(t, strings.HasSuffix(directive.Excerpt, "…"))
	assert.True(t, strings.HasPrefix(hpDirective, strings.TrimSuffix(directive.Excerpt, "…")))
	assert.Equal(t, fmt.Sprintf("/rooms/%s#message-2", slug), directive.MessageURL)

	// Nobody is in the room, so it is a completed collaboration — not a live one.
	assert.Equal(t, "completed", ex.State)
	assert.Equal(t, "COMPLETED COLLABORATION", ex.Label)
	assert.Equal(t, 0, ex.LiveAgentCount)

	assert.Equal(t, "/connect?preset=planner-executor", ex.ConnectURL)
	assert.NotEmpty(t, ex.ConnectLabel)
}

func TestHomepageExample_CallsItLiveOnlyWhilePresenceIsReported(t *testing.T) {
	slug := fmt.Sprintf("test-hpex-%d", time.Now().UnixNano()%1000000)
	t.Setenv("HOMEPAGE_EXAMPLE_ROOM_SLUG", slug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	room := seedHomepageExampleRoom(t, pool, slug)

	before, _ := getHomepageExample(t, ts.URL)
	require.Equal(t, "completed", before.State)

	presenceRepo := db.NewAgentPresenceRepository(pool)
	_, err := presenceRepo.Upsert(context.Background(), models.UpsertAgentPresenceParams{
		RoomID:     room.ID,
		AgentName:  before.Participants[0].Name,
		CardJSON:   json.RawMessage(`{}`),
		TTLSeconds: 300,
	})
	require.NoError(t, err)

	live, _ := getHomepageExample(t, ts.URL)
	assert.Equal(t, "live", live.State)
	assert.Equal(t, "LIVE COLLABORATION", live.Label)
	assert.Equal(t, 1, live.LiveAgentCount)
}

func TestHomepageExample_RemovesPreviewContentWhenTheRoomGoesAway(t *testing.T) {
	slug := fmt.Sprintf("test-hpex-%d", time.Now().UnixNano()%1000000)
	t.Setenv("HOMEPAGE_EXAMPLE_ROOM_SLUG", slug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	room := seedHomepageExampleRoom(t, pool, slug)
	real, _ := getHomepageExample(t, ts.URL)
	require.Equal(t, "real", real.Kind)
	plannerName := real.Participants[0].Name

	assertIllustrative := func(t *testing.T, reason string) {
		t.Helper()
		ex, raw := getHomepageExample(t, ts.URL)

		assert.Equal(t, "illustrative", ex.Kind, reason)
		assert.Equal(t, "ILLUSTRATIVE WORKFLOW", ex.Label)
		assert.Nil(t, ex.Room)
		assert.Empty(t, ex.RoomURL)
		assert.Empty(t, ex.Participants)
		require.Len(t, ex.Steps, 5, "the workflow is still explained")
		for _, s := range ex.Steps {
			assert.Empty(t, s.Author)
			assert.Empty(t, s.MessageURL)
			assert.NotEmpty(t, s.Excerpt)
		}
		// The Connect action still works.
		assert.Equal(t, "/connect?preset=planner-executor", ex.ConnectURL)

		// Nothing from the room survives anywhere in the public response.
		assert.NotContains(t, raw, slug, reason)
		assert.NotContains(t, raw, plannerName, reason)
		assert.NotContains(t, raw, hpDirective[:60], reason)
	}

	ctx := context.Background()
	roomRepo := db.NewRoomRepository(pool)

	// 1. The room goes private.
	private := true
	_, err := roomRepo.Update(ctx, room.ID, models.UpdateRoomParams{IsPrivate: &private})
	require.NoError(t, err)
	assertIllustrative(t, "a private room must not be previewed publicly")

	// 2. The room is deleted.
	require.NoError(t, roomRepo.SoftDelete(ctx, room.ID))
	assertIllustrative(t, "a deleted room must not be previewed publicly")
}

func TestHomepageExample_FallsBackWhenTheConfiguredRoomDoesNotExist(t *testing.T) {
	t.Setenv("HOMEPAGE_EXAMPLE_ROOM_SLUG", fmt.Sprintf("test-hpex-missing-%d", time.Now().UnixNano()%1000000))

	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	ex, _ := getHomepageExample(t, ts.URL)
	assert.Equal(t, "illustrative", ex.Kind)
	assert.Equal(t, "ILLUSTRATIVE WORKFLOW", ex.Label)
	assert.Equal(t, "/connect?preset=planner-executor", ex.ConnectURL)
	require.Len(t, ex.Steps, 5)
}
