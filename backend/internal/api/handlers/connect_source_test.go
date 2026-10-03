package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// "Try this workflow" (idx 88): a public room or an outcome post seeds a FRESH start
// flow. Only the room's public task structure travels — scrubbed of credentials and
// private references — and the prompt carries the source so the new room records where
// it came from. Nothing else of the source room (members, tokens, approvals) ever does.

type fakeRoomSources struct {
	templates map[string]*models.RoomTemplate
	public    map[string]bool
	asked     []string
}

func (f *fakeRoomSources) FindPublicRoomTemplate(_ context.Context, slug string) (*models.RoomTemplate, error) {
	f.asked = append(f.asked, slug)
	if t, ok := f.templates[slug]; ok {
		return t, nil
	}
	return nil, db.ErrRoomNotFound
}

func (f *fakeRoomSources) PublicRoomSlugs(_ context.Context, slugs []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, s := range slugs {
		if f.public[s] {
			out[s] = true
		}
	}
	return out, nil
}

type fakeConnectPosts struct {
	id, title string
}

func (f *fakeConnectPosts) FindPublicPostRef(_ context.Context, id string) (string, string, error) {
	if id == f.id {
		return f.id, f.title, nil
	}
	return "", "", db.ErrPostNotFound
}

const leakyInitialTask = "Build a tic-tac-toe game. Key: solvr_rt_abcdefghijklmnopqrstuvwx and solvr_sk_ABCDEFGHIJKLMNOPQRST1234. " +
	"Stream at https://api.solvr.dev/r/ttt/stream?token=solvr_rt_zzzzzzzzzzzzzzzzzzzz&after=3 and " +
	"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.c2lnbmF0dXJlLXZhbHVl. " +
	"Rules are in https://solvr.dev/rooms/secret-planning-room and the demo is /rooms/open-demo."

func newSourceConnectHandler(t *testing.T) (*ConnectHandler, *fakeRoomSources) {
	t.Helper()
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")
	desc := "A two-agent game build"
	src := &fakeRoomSources{
		templates: map[string]*models.RoomTemplate{
			"ttt-public": {
				RoomID:      uuid.New(),
				Slug:        "ttt-public",
				DisplayName: "Tic-tac-toe build",
				Description: &desc,
				Tags:        []string{"game"},
				InitialTask: leakyInitialTask,
			},
		},
		public: map[string]bool{"open-demo": true},
	}
	h.SetRoomSourceLookup(src)
	h.SetPostLookup(&fakeConnectPosts{id: "6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11", title: "Port the billing job"})
	return h, src
}

func TestConnect_RoomSourceSeedsTheTaskFromThePublicRoomsInitialTask(t *testing.T) {
	h, _ := newSourceConnectHandler(t)
	start, _ := getConnect(t, h, "from_room=ttt-public")

	require.NotNil(t, start.Source, "a public room must produce a source")
	require.Equal(t, "room", start.Source.Kind)
	require.Equal(t, "ttt-public", start.Source.RoomSlug)
	require.Equal(t, "Tic-tac-toe build", start.Source.Title)
	require.Equal(t, "/rooms/ttt-public", start.Source.URL)
	require.Equal(t, "ttt-public", start.Selected.SourceRoom)
	require.Empty(t, start.Selected.SourcePostID)

	require.Contains(t, start.Selected.Task, "Build a tic-tac-toe game.")
	require.Contains(t, start.Prompt.Text, "Build a tic-tac-toe game.")
}

func TestConnect_RoomSourceTaskCarriesNoCredentialsOrPrivateReferences(t *testing.T) {
	h, _ := newSourceConnectHandler(t)
	start, body := getConnect(t, h, "from_room=ttt-public")

	for _, leaked := range []string{
		"solvr_rt_abcdefghijklmnopqrstuvwx", "solvr_sk_ABCDEFGHIJKLMNOPQRST1234",
		"token=solvr_rt_zzzz", "eyJhbGciOiJIUzI1NiJ9", "secret-planning-room",
	} {
		require.NotContains(t, start.Selected.Task, leaked)
		require.NotContains(t, body, leaked, "the whole contract must not carry %q", leaked)
	}
	require.Contains(t, start.Selected.Task, "[private room]")
	// A link to another PUBLIC room is public information and stays.
	require.Contains(t, start.Selected.Task, "/rooms/open-demo")
	// The non-secret part of a URL survives so the instruction still reads.
	require.Contains(t, start.Selected.Task, "after=3")
}

func TestConnect_RoomSourceTravelsInTheCreateBodyAndTheRoomStartsFresh(t *testing.T) {
	h, _ := newSourceConnectHandler(t)
	for _, preset := range []string{ConnectPresetPlanAndBuild, ConnectPresetBuildAndReview, ConnectPresetCollaborate} {
		t.Run(preset, func(t *testing.T) {
			start, _ := getConnect(t, h, "from_room=ttt-public&preset="+preset)
			require.Contains(t, start.Prompt.Text, `"source_room": "ttt-public"`)
			require.Contains(t, start.Prompt.Text, "This room starts fresh")
			require.Contains(t, start.Prompt.Text, "no earlier members, credentials, approvals, reviews or results carry over")
		})
	}
}

func TestConnect_TypedTaskWinsOverTheRoomSourceTask(t *testing.T) {
	h, _ := newSourceConnectHandler(t)
	start, _ := getConnect(t, h, "from_room=ttt-public&task=ship+the+scoreboard")

	require.Equal(t, "ship the scoreboard", start.Selected.Task)
	require.NotNil(t, start.Source, "the source is still recorded")
	require.Contains(t, start.Prompt.Text, `"source_room": "ttt-public"`)
}

func TestConnect_PrivateOrMissingSourceRoomDegradesToTheOrdinaryContract(t *testing.T) {
	h, src := newSourceConnectHandler(t)
	start, body := getConnect(t, h, "from_room=secret-planning-room")

	require.Equal(t, []string{"secret-planning-room"}, src.asked)
	require.Nil(t, start.Source)
	require.Empty(t, start.Selected.SourceRoom)
	require.Empty(t, start.Selected.Task)
	require.NotContains(t, body, "secret-planning-room")
	require.NotContains(t, start.Prompt.Text, "source_room")
	require.NotContains(t, start.Prompt.Text, "This room starts fresh")
}

func TestConnect_PostSourceTravelsInTheCreateBody(t *testing.T) {
	h, _ := newSourceConnectHandler(t)
	start, _ := getConnect(t, h, "post=6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11")

	require.NotNil(t, start.Source)
	require.Equal(t, "post", start.Source.Kind)
	require.Equal(t, "6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11", start.Selected.SourcePostID)
	require.Contains(t, start.Prompt.Text, `"source_post_id": "6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11"`)
}

func TestConnect_RoomAndPostTogetherIsRefusedRatherThanGuessed(t *testing.T) {
	h, _ := newSourceConnectHandler(t)
	req := httptest.NewRequest(http.MethodGet,
		"/v1/connect?from_room=ttt-public&post=6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11", nil)
	w := httptest.NewRecorder()
	h.GetConnect(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "AMBIGUOUS_SOURCE")
}

func TestConnect_NoSourceLookupWiredIgnoresTheRoomParameter(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")
	start, _ := getConnect(t, h, "from_room=ttt-public")
	require.Nil(t, start.Source)
	require.NotContains(t, start.Prompt.Text, "source_room")
}

func TestPublicTemplateText_ScrubsCredentialsAndPrivateRoomLinks(t *testing.T) {
	src := &fakeRoomSources{public: map[string]bool{"open-demo": true}}
	cases := []struct {
		name, in   string
		gone, kept []string
	}{
		{"room token", "use solvr_rt_abcdefghijklmnopqrstuvwx now", []string{"solvr_rt_abc"}, []string{"use", "now"}},
		{"user key", "key solvr_sk_ABCDEFGHIJKLMNOPQRST1234", []string{"solvr_sk_ABC"}, []string{"key"}},
		{"agent key", "agent solvr_9f8e7d6c5b4a39281706f5e4d3c2b1a0", []string{"solvr_9f8e7d"}, []string{"agent"}},
		{"jwt", "jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.c2lnbmF0dXJlLXZhbHVl ok", []string{"eyJhbGci"}, []string{"jwt", "ok"}},
		{"bearer", "Authorization: Bearer abc.def-123", []string{"abc.def-123"}, []string{"Authorization"}},
		{"query token", "GET /x?access_token=abc123&type=DONE", []string{"abc123"}, []string{"type=DONE"}},
		{"private room", "see https://solvr.dev/rooms/hidden-room/entries", []string{"hidden-room"}, []string{"[private room]"}},
		{"public room", "see /rooms/open-demo", nil, []string{"/rooms/open-demo"}},
		{"product word", "the solvr_dev database and solvr_ prefix", nil, []string{"solvr_dev", "solvr_ prefix"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := publicTemplateText(context.Background(), tc.in, src)
			for _, g := range tc.gone {
				require.NotContains(t, out, g)
			}
			for _, k := range tc.kept {
				require.Contains(t, out, k)
			}
			require.False(t, strings.Contains(out, "$"), "no template-looking residue")
		})
	}
}

func TestScrubRoomTemplate_RemovesSecretsFromTheCopiedDescription(t *testing.T) {
	desc := "Use solvr_rt_abcdefghijklmnopqrstuvwx and see /rooms/hidden-room or /rooms/open-demo"
	tmpl := &models.RoomTemplate{Description: &desc, InitialTask: "x"}
	scrubRoomTemplate(context.Background(), tmpl, &fakeRoomSources{public: map[string]bool{"open-demo": true}})

	require.NotContains(t, *tmpl.Description, "solvr_rt_abc")
	require.NotContains(t, *tmpl.Description, "hidden-room")
	require.Contains(t, *tmpl.Description, "/rooms/open-demo")
	require.Equal(t, "Use solvr_rt_abcdefghijklmnopqrstuvwx and see /rooms/hidden-room or /rooms/open-demo", desc,
		"the caller's string is not mutated in place")
}
