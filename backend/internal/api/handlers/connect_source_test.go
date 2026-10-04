package handlers

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// "Try this workflow" and "Discuss with agents": a public room or a published post seeds a
// fresh start. The sentence's intent names the source by its public link; the skill tells
// the agent to carry it into the create call (source_room / source_post_id). Nothing else
// of the source — its members, tokens, approvals, or its task text — travels.

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

const (
	sourcePostID     = "6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11"
	leakyInitialTask = "Build a tic-tac-toe game. Key: solvr_rt_abcdefghijklmnopqrstuvwx and solvr_sk_ABCDEFGHIJKLMNOPQRST1234. " +
		"Rules are in https://solvr.dev/rooms/secret-planning-room."
)

func newSourceConnectHandler(t *testing.T) *ConnectHandler {
	t.Helper()
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")
	desc := "A two-agent game build"
	h.SetRoomSourceLookup(&fakeRoomSources{
		templates: map[string]*models.RoomTemplate{
			"ttt-public": {RoomID: uuid.New(), Slug: "ttt-public", DisplayName: "Tic-tac-toe build",
				Description: &desc, Tags: []string{"game"}, InitialTask: leakyInitialTask},
		},
	})
	h.SetPostLookup(&fakeConnectPosts{id: sourcePostID, title: "Port the billing job"})
	return h
}

func TestConnectSource_APublicRoomSeedsTheIntentWithItsLink(t *testing.T) {
	start, body := getConnect(t, newSourceConnectHandler(t), "from_room=ttt-public")
	require.NotNil(t, start.Source)
	require.Equal(t, "room", start.Source.Kind)
	require.Equal(t, "ttt-public", start.Source.RoomSlug)
	require.Equal(t, "ttt-public", start.Selected.SourceRoom)
	require.Equal(t, "run your own version of the Solvr room https://solvr.dev/rooms/ttt-public", start.Selected.Intent)
	require.Equal(t, start.Selected.Intent, segmentsOf(start.Prompt, SegmentIntent)[0].Text)
	require.Equal(t, start.Prompt.Text, joined(start.Prompt))
	for _, leak := range []string{"solvr_rt_", "solvr_sk_", "secret-planning-room", "Build a tic-tac-toe game"} {
		require.NotContains(t, body, leak, "only the source's public link travels")
	}
}

func TestConnectSource_APublishedPostSeedsTheIntentWithItsLink(t *testing.T) {
	start, _ := getConnect(t, newSourceConnectHandler(t), "post="+sourcePostID)
	require.NotNil(t, start.Source)
	require.Equal(t, "post", start.Source.Kind)
	require.Equal(t, "Port the billing job", start.Source.Title)
	require.Equal(t, sourcePostID, start.Selected.SourcePostID)
	require.Equal(t, "build on the Solvr post https://solvr.dev/posts/"+sourcePostID, start.Selected.Intent)
	require.Less(t, start.Prompt.WordCount, 120)
}

func TestConnectSource_ATypedIntentWinsOverTheSource(t *testing.T) {
	start, _ := getConnect(t, newSourceConnectHandler(t), "from_room=ttt-public&intent=ship+it")
	require.Equal(t, "ship it", start.Selected.Intent)
	require.Equal(t, "ttt-public", start.Selected.SourceRoom, "the source is still recorded for the page")
}

func TestConnectSource_AProtectedOrMissingSourceDegradesToTheOrdinaryContract(t *testing.T) {
	h := newSourceConnectHandler(t)
	for _, q := range []string{"from_room=secret-room", "post=00000000-0000-0000-0000-000000000000"} {
		start, _ := getConnect(t, h, q)
		require.Nil(t, start.Source, q)
		require.Empty(t, start.Selected.SourceRoom, q)
		require.Empty(t, start.Selected.SourcePostID, q)
		require.True(t, segmentsOf(start.Prompt, SegmentIntent)[0].Empty, q)
	}
}

func TestConnectSource_OneSourceAtATime(t *testing.T) {
	w := serveConnect(newSourceConnectHandler(t), "from_room=ttt-public&post="+sourcePostID)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.True(t, strings.Contains(w.Body.String(), "AMBIGUOUS_SOURCE"))
}
