package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// GET /v1/connect is the one API-owned contract behind every surface that
// starts a connection: the compact panel on the index and the full /connect
// page render the SAME payload. Every label, every explanation, every option
// and the prompt text itself are decided here, so the two surfaces cannot drift
// apart and the browser never writes a prompt of its own.

// fakeConnectRooms stands in for the room repository.
type fakeConnectRooms struct {
	room *models.Room
	err  error
	// asked records the slug the handler looked up.
	asked string
}

func (f *fakeConnectRooms) GetBySlug(_ context.Context, slug string) (*models.Room, error) {
	f.asked = slug
	if f.err != nil {
		return nil, f.err
	}
	return f.room, nil
}

// getConnect calls the handler with no credentials and decodes the contract.
func getConnect(t *testing.T, h *ConnectHandler, query string) (ConnectStart, string) {
	t.Helper()
	url := "/v1/connect"
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	h.GetConnect(w, req)

	body := w.Body.String()
	require.Equal(t, http.StatusOK, w.Code, "body: %s", body)

	var wrapper struct {
		Data ConnectStart `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &wrapper), "body: %s", body)
	return wrapper.Data, body
}

// publicExampleRoom is a real, public example room.
func publicExampleRoom(slug string) *models.Room {
	return &models.Room{Slug: slug, DisplayName: "Tic-Tac-Toe Human vs Computer", IsPrivate: false}
}

func newTestConnectHandler(t *testing.T, rooms connectRoomLookup, slug string) *ConnectHandler {
	t.Helper()
	t.Setenv("HOMEPAGE_EXAMPLE_ROOM_SLUG", slug)
	return NewConnectHandler(rooms)
}

func TestConnect_DefaultsToPlanAndBuildAndPublicWithExactlyOneSelection(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, body := getConnect(t, h, "")

	require.Equal(t, ConnectPresetPlanAndBuild, start.Selected.Preset, "body: %s", body)
	require.Equal(t, ConnectVisibilityPublic, start.Selected.Visibility)
	require.Empty(t, start.Selected.Task)

	var presetSelected, visibilitySelected int
	for _, p := range start.Presets {
		if p.Selected {
			presetSelected++
			require.Equal(t, ConnectPresetPlanAndBuild, p.Value)
		}
		require.NotEmpty(t, p.Label)
		require.NotEmpty(t, p.Description)
	}
	for _, v := range start.VisibilityOptions {
		if v.Selected {
			visibilitySelected++
			require.Equal(t, ConnectVisibilityPublic, v.Value)
		}
		require.NotEmpty(t, v.Label)
		require.NotEmpty(t, v.Description)
	}
	require.Equal(t, 1, presetSelected, "exactly one preset is selected")
	require.Equal(t, 1, visibilitySelected, "exactly one visibility is selected")

	// Both surfaces are named by the contract itself: the panel can always
	// point at the full page, and the page is a real URL.
	require.Equal(t, "/connect", start.PageURL)
	require.NotEmpty(t, start.PageLabel)
	require.NotEmpty(t, start.Heading)
	require.NotEmpty(t, start.Intro)
}

func TestConnect_PublicAndPrivateCarryTheirOwnExplanation(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, _ := getConnect(t, h, "")

	byValue := map[string]ConnectOption{}
	for _, v := range start.VisibilityOptions {
		byValue[v.Value] = v
	}
	require.Contains(t, byValue, ConnectVisibilityPublic)
	require.Contains(t, byValue, ConnectVisibilityPrivate)
	require.Contains(t, byValue[ConnectVisibilityPublic].Description, "Anyone can read this room")
	require.Contains(t, strings.ToLower(byValue[ConnectVisibilityPrivate].Description), "admitted")
}

func TestConnect_WithNoTaskThePlannerPromptStillCopiesAndSaysWhereToPasteIt(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, body := getConnect(t, h, "")

	// The exact sentence the panel must show beside the copy control.
	require.Equal(t,
		"Paste this into your planner. It will give you the prompt for your executor.",
		start.Prompt.Instruction, "body: %s", body)
	require.Equal(t, "planner", start.Prompt.Key)
	require.Equal(t, "Copy planner prompt", start.Prompt.Label)
	require.NotEmpty(t, start.Prompt.CopiedLabel)
	require.NotEmpty(t, start.Prompt.NextStep)

	// A prompt with no task is a COMPLETE prompt: the planner asks for the task
	// itself rather than the page refusing to copy.
	require.NotEmpty(t, start.Prompt.Text)
	lower := strings.ToLower(start.Prompt.Text)
	require.Contains(t, lower, "ask me")
	require.Contains(t, lower, "before you create the room")
}

func TestConnect_TheTypedTaskIsCarriedIntoThePromptVerbatim(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	task := `Fix the "flaky" login test; it fails ~1 in 5 runs`
	start, body := getConnect(t, h, "task="+url.QueryEscape(task))

	require.Equal(t, task, start.Selected.Task, "body: %s", body)
	require.Contains(t, start.Prompt.Text, task)
	// The task is carried as text, never as an argument the agent would expand:
	// nothing in the prompt may look like a shell or template substitution.
	require.NotContains(t, start.Prompt.Text, "$")
	require.NotContains(t, start.Prompt.Text, "`")
}

func TestConnect_PrivateVisibilityChangesTheRoomTheAgentIsToldToCreate(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	public, _ := getConnect(t, h, "visibility=public")
	private, _ := getConnect(t, h, "visibility=private")

	require.Contains(t, public.Prompt.Text, `"is_private": false`)
	require.Contains(t, private.Prompt.Text, `"is_private": true`)
	require.Equal(t, ConnectVisibilityPrivate, private.Selected.Visibility)
	for _, v := range private.VisibilityOptions {
		require.Equal(t, v.Value == ConnectVisibilityPrivate, v.Selected)
	}
}

func TestConnect_CollaboratePresetCopiesAStarterPromptInstead(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, body := getConnect(t, h, "preset=collaborate")

	require.Equal(t, ConnectPresetCollaborate, start.Selected.Preset, "body: %s", body)
	require.Equal(t, "starter", start.Prompt.Key)
	require.Equal(t, "Copy starter prompt", start.Prompt.Label)
	require.NotEqual(t, "", start.Prompt.Text)
	// A peer collaboration has no planner directing an executor.
	require.NotContains(t, strings.ToLower(start.Prompt.Text), "executor")
}

func TestConnect_PromptOnlyNamesProductionEndpointsAndNeverAFabricatedRoom(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	for _, query := range []string{"", "preset=collaborate", "visibility=private"} {
		start, _ := getConnect(t, h, query)
		text := start.Prompt.Text

		require.Contains(t, text, "https://api.solvr.dev/v1/agents/register")
		require.Contains(t, text, "https://api.solvr.dev/v1/rooms")
		require.Contains(t, text, "https://solvr.dev/rooms/")
		require.NotContains(t, text, "http://localhost")

		// A documentation placeholder must never be able to read as a finished
		// room link: no /rooms/$ and no /rooms/${...}.
		require.NotContains(t, text, "/rooms/$")
		require.NotContains(t, text, "${")

		// The prompt must tell the agent to report failure as failure.
		require.Contains(t, strings.ToLower(text), "never invent")
	}
}

func TestConnect_PlannerPromptReusesTheAgentsOwnIdentityAndOwnsTheRoom(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, _ := getConnect(t, h, "")
	lower := strings.ToLower(start.Prompt.Text)

	require.Contains(t, lower, "reuse")
	require.Contains(t, lower, "register")
	require.Contains(t, lower, "handshake")
	// No human step is required anywhere in the flow.
	require.NotContains(t, lower, "log in to solvr")
	require.NotContains(t, lower, "install the solvr cli")
}

func TestConnect_ExplainsTheTwoCopyPasteActionsAndLinksTheRealExample(t *testing.T) {
	rooms := &fakeConnectRooms{room: publicExampleRoom("tictactoe-human-vs-computer-20260920")}
	h := newTestConnectHandler(t, rooms, "tictactoe-human-vs-computer-20260920")

	start, body := getConnect(t, h, "")

	require.Len(t, start.Steps, 2, "the two initial copy/paste actions: body %s", body)
	for i, s := range start.Steps {
		require.Equal(t, i+1, s.Number)
		require.NotEmpty(t, s.Label)
		require.NotEmpty(t, s.Detail)
	}

	require.Equal(t, "real", start.Example.Kind)
	require.Equal(t, "/rooms/tictactoe-human-vs-computer-20260920", start.Example.URL)
	require.NotEmpty(t, start.Example.Label)
	require.NotEmpty(t, start.Example.Detail)
	require.Equal(t, "tictactoe-human-vs-computer-20260920", rooms.asked)
}

func TestConnect_ExampleFallsBackToPublicRoomsWhenTheShowcaseIsUnavailable(t *testing.T) {
	cases := map[string]*fakeConnectRooms{
		"missing": {err: errors.New("room not found")},
		"private": {room: &models.Room{Slug: "secret", IsPrivate: true}},
		"none":    {},
	}
	for name, rooms := range cases {
		t.Run(name, func(t *testing.T) {
			h := newTestConnectHandler(t, rooms, "example-room")
			start, body := getConnect(t, h, "")

			require.Equal(t, "directory", start.Example.Kind, "body: %s", body)
			require.Equal(t, "/rooms", start.Example.URL)
			require.NotEmpty(t, start.Example.Label)
			require.NotContains(t, start.Example.URL, "secret")
		})
	}
}

func TestConnect_WorksWithNoRoomRepositoryAtAll(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")

	start, _ := getConnect(t, h, "")

	require.Equal(t, "directory", start.Example.Kind)
	require.NotEmpty(t, start.Prompt.Text, "the prompt never depends on the example room")
}

func TestConnect_RejectsAnUnknownPresetOrVisibilityInsteadOfGuessing(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	for _, query := range []string{"preset=solo", "visibility=unlisted"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/connect?"+query, nil)
		w := httptest.NewRecorder()
		h.GetConnect(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, "query %s body %s", query, w.Body.String())
		require.Contains(t, w.Body.String(), "error")
	}
}

func TestConnect_RejectsATaskTooLongToBeAPrompt(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	req := httptest.NewRequest(http.MethodGet,
		"/v1/connect?task="+strings.Repeat("a", ConnectTaskMaxChars+1), nil)
	w := httptest.NewRecorder()
	h.GetConnect(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
}

func TestConnect_TellsTheVisitorThatCopyingAloneConnectsNothing(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, _ := getConnect(t, h, "")

	require.NotEmpty(t, start.Note)
	require.Contains(t, strings.ToLower(start.Note), "copying")
}

func TestConnect_NamesTheGroupsTheOptionsBelongTo(t *testing.T) {
	// The two option groups need their own question in words, and that wording
	// is a product decision — so it is the API's, not the browser's.
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, body := getConnect(t, h, "")

	require.NotEmpty(t, start.PresetsLabel, "body: %s", body)
	require.NotEmpty(t, start.VisibilityLabel)
	require.NotEqual(t, start.PresetsLabel, start.VisibilityLabel)
}

func TestConnect_ProvidesAnAddAnotherAgentControlWithRoleLabels(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, body := getConnect(t, h, "")

	require.NotEmpty(t, start.AddAgent.Label, "body: %s", body)
	require.NotEmpty(t, start.AddAgent.Detail)
	require.NotEmpty(t, start.AddAgent.SlugPlaceholder)
	require.NotEmpty(t, start.AddAgent.RolePrompt, "the role prompt the visitor copies for an extra agent")
	// No "$" or "${" — placeholders must be unmistakable, not shell-expandable.
	require.NotContains(t, start.AddAgent.SlugPlaceholder, "$")
	require.NotContains(t, start.AddAgent.RolePrompt, "$")
	require.NotContains(t, start.AddAgent.RolePrompt, "${")
}

func TestConnect_CollaboratePresetProvidesASuitableAddAgentRole(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, _ := getConnect(t, h, "preset=collaborate")

	require.NotEmpty(t, start.AddAgent.Label)
	require.NotEmpty(t, start.AddAgent.RolePrompt)
	// A peer collaboration has no planner directing an executor, so the add-another-agent
	// role must not name the executor either.
	require.NotContains(t, strings.ToLower(start.AddAgent.RolePrompt), "executor")
}

func TestConnect_CustomizeSectionCarriesAdvancedInstructionsAndApiExamples(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, body := getConnect(t, h, "")

	require.Equal(t, "customize", start.Customize.Key, "body: %s", body)
	require.NotEmpty(t, start.Customize.Label)
	require.NotEmpty(t, start.Customize.Detail)
	require.NotEmpty(t, start.Customize.ApiExamples, "body: %s", body)
	require.NotEmpty(t, start.Customize.AdvancedInstructions)

	// The API examples must reference real production endpoints, not localhost.
	for _, example := range start.Customize.ApiExamples {
		require.Contains(t, example, "https://api.solvr.dev")
	}
	for _, example := range start.Customize.AdvancedInstructions {
		require.NotEqual(t, "", strings.TrimSpace(example))
	}
}

func TestConnect_CopyingTheStarterPromptAloneDoesNotCreateARoom(t *testing.T) {
	h := newTestConnectHandler(t, &fakeConnectRooms{room: publicExampleRoom("example-room")}, "example-room")

	start, _ := getConnect(t, h, "")

	// The note that says copying alone connects nothing is a product guarantee.
	require.Contains(t, strings.ToLower(start.Note), "copying")
	require.Contains(t, strings.ToLower(start.Note), "does not create")

	// The add-agent role prompt must also carry the same guarantee: it is a
	// prompt to paste, not an action that creates anything on its own.
	require.Contains(t, strings.ToLower(start.AddAgent.Detail), "paste")
}
