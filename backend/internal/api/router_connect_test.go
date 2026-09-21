package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// GET /v1/connect through the real router, called the way a logged-out visitor
// calls it: no Authorization header, no cookie, no account.
//
// The index panel and the /connect page both read this endpoint, so whatever is
// proven here is true of both surfaces.

type connectContract struct {
	Heading   string `json:"heading"`
	Intro     string `json:"intro"`
	PageURL   string `json:"page_url"`
	PageLabel string `json:"page_label"`
	TaskField struct {
		Label    string `json:"label"`
		Optional bool   `json:"optional"`
		Note     string `json:"note"`
		MaxChars int    `json:"max_chars"`
	} `json:"task_field"`
	Presets []struct {
		Value       string `json:"value"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Selected    bool   `json:"selected"`
	} `json:"presets"`
	VisibilityOptions []struct {
		Value       string `json:"value"`
		Label       string `json:"label"`
		Description string `json:"description"`
		Selected    bool   `json:"selected"`
	} `json:"visibility_options"`
	Selected struct {
		Task       string `json:"task"`
		Preset     string `json:"preset"`
		Visibility string `json:"visibility"`
	} `json:"selected"`
	Prompt struct {
		Key         string `json:"key"`
		Label       string `json:"label"`
		CopiedLabel string `json:"copied_label"`
		Instruction string `json:"instruction"`
		NextStep    string `json:"next_step"`
		Text        string `json:"text"`
	} `json:"prompt"`
	Steps []struct {
		Number int    `json:"number"`
		Label  string `json:"label"`
		Detail string `json:"detail"`
	} `json:"steps"`
	Example struct {
		Kind   string `json:"kind"`
		URL    string `json:"url"`
		Label  string `json:"label"`
		Detail string `json:"detail"`
	} `json:"example"`
	Note string `json:"note"`
}

// getConnectContract calls the endpoint with no credentials at all.
func getConnectContract(t *testing.T, baseURL, query string) (connectContract, string) {
	t.Helper()
	url := baseURL + "/v1/connect"
	if query != "" {
		url += "?" + query
	}
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var wrapper struct {
		Data connectContract `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &wrapper), "body: %s", string(body))
	return wrapper.Data, string(body)
}

func TestConnectEndpoint_ServesTheWholeStartContractToALoggedOutVisitor(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	contract, body := getConnectContract(t, ts.URL, "")

	require.NotEmpty(t, contract.Heading, "body: %s", body)
	require.NotEmpty(t, contract.Intro)
	require.Equal(t, "/connect", contract.PageURL)
	require.NotEmpty(t, contract.PageLabel)
	require.NotEmpty(t, contract.TaskField.Label)
	require.True(t, contract.TaskField.Optional)
	require.Greater(t, contract.TaskField.MaxChars, 0)
	require.Len(t, contract.Presets, 2)
	require.Len(t, contract.VisibilityOptions, 2)
	require.Len(t, contract.Steps, 2)
	require.Equal(t, "plan-and-build", contract.Selected.Preset)
	require.Equal(t, "public", contract.Selected.Visibility)
	require.Equal(t, "planner", contract.Prompt.Key)
	require.Equal(t,
		"Paste this into your planner. It will give you the prompt for your executor.",
		contract.Prompt.Instruction)
	require.NotEmpty(t, contract.Prompt.Text)
	require.NotEmpty(t, contract.Note)

	// No credential, identifier or private detail may ride along with a public
	// contract.
	lower := strings.ToLower(body)
	for _, forbidden := range []string{"solvr_rt_2", "api_key\":", "room_token\":", "owner_id"} {
		require.NotContains(t, lower, strings.ToLower(forbidden), "public contract leaked %q", forbidden)
	}
}

func TestConnectEndpoint_CarriesTheTypedTaskAndTheChosenVisibilityIntoThePrompt(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	contract, body := getConnectContract(t, ts.URL, "task=Port+the+billing+job+to+the+new+queue&visibility=private")

	require.Equal(t, "Port the billing job to the new queue", contract.Selected.Task, "body: %s", body)
	require.Contains(t, contract.Prompt.Text, "Port the billing job to the new queue")
	require.Equal(t, "private", contract.Selected.Visibility)
	require.Contains(t, contract.Prompt.Text, `"is_private": true`)
}

func TestConnectEndpoint_RefusesAnUnknownPresetOrVisibility(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	for _, query := range []string{"preset=solo", "visibility=unlisted"} {
		resp, err := http.Get(ts.URL + "/v1/connect?" + query)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "query %s body %s", query, string(body))
	}
}

func TestConnectEndpoint_LinksTheRealPublicExampleRoomItIsPointedAt(t *testing.T) {
	slug := fmt.Sprintf("test-connect-example-%d", time.Now().UnixNano()%1000000)
	t.Setenv("HOMEPAGE_EXAMPLE_ROOM_SLUG", slug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	room, _, err := db.NewRoomRepository(pool).Create(t.Context(), models.CreateRoomParams{
		Slug:        slug,
		DisplayName: "A public planner and executor collaboration",
		IsPrivate:   false,
		OwnerID:     uuid.Nil,
	})
	require.NoError(t, err)
	require.Equal(t, slug, room.Slug)

	contract, body := getConnectContract(t, ts.URL, "")

	require.Equal(t, "real", contract.Example.Kind, "body: %s", body)
	require.Equal(t, "/rooms/"+slug, contract.Example.URL)
	require.NotEmpty(t, contract.Example.Label)
}

// promptEndpoint is one API call a prompt tells an agent to make.
var promptEndpointRE = regexp.MustCompile(`(GET|POST) https://api\.solvr\.dev(/[A-Za-z0-9_/{}.-]+)`)

func TestConnectEndpoint_PromptsOnlyNameRoutesThisAPIActuallyServes(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	// The routes are walked on a router built exactly like the one under test,
	// hub included: without the hub the room transport routes are not mounted
	// at all and the walk would prove nothing.
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(context.Background(), registry, slog.Default(), 0)
	router := NewRouter(pool, hubMgr, registry)

	// Every route the router really serves, method by method.
	served := map[string]bool{}
	require.NoError(t, chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		served[method+" "+strings.TrimSuffix(route, "/")] = true
		return nil
	}))

	for _, query := range []string{"", "preset=collaborate", "visibility=private"} {
		contract, _ := getConnectContract(t, ts.URL, query)
		matches := promptEndpointRE.FindAllStringSubmatch(contract.Prompt.Text, -1)
		require.NotEmpty(t, matches, "the prompt names no endpoint at all (query %q)", query)

		for _, m := range matches {
			method, path := m[1], m[2]
			// The prompt's slug placeholder is the router's slug parameter.
			path = strings.ReplaceAll(path, "ROOM_SLUG", "{slug}")
			key := method + " " + strings.TrimSuffix(path, "/")
			require.True(t, served[key],
				"prompt (query %q) tells the agent to call %s, which this API does not serve", query, key)
		}
	}
}
