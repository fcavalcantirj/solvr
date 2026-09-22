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
	AddAgent struct {
		Label           string `json:"label"`
		Detail          string `json:"detail"`
		SlugPlaceholder string `json:"slug_placeholder"`
		RolePrompt      string `json:"role_prompt"`
	} `json:"add_agent"`
	Customize struct {
		Key                  string   `json:"key"`
		Label                string   `json:"label"`
		Detail               string   `json:"detail"`
		ApiExamples          []string `json:"api_examples"`
		AdvancedInstructions []string `json:"advanced_instructions"`
	} `json:"customize"`
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

// TestConnectEndpoint_PlannerPromptRunsEndToEndWithoutAHumanAccount simulates a
// planner agent with NO existing Solvr identity following the planner prompt from
// /v1/connect exactly as an agent would paste it: self-register, create the room
// it will own, take its own per-agent token, join presence, post the task and
// directive, and return the real room URL. Every call is made through the real
// router against the real database, called the way an agent calls it — with its
// own key, not a human JWT.
func TestConnectEndpoint_PlannerPromptRunsEndToEndWithoutAHumanAccount(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_task18%'") //nolint:errcheck
	})

	// Step 1 of the prompt: the agent gets the contract. No Authorization header.
	contract, _ := getConnectContract(t, ts.URL, "task=Build+a+tic-tac-toe+AI")
	require.NotEmpty(t, contract.Prompt.Text)

	// Step 2: IDENTITY — self-register (no existing identity, no human account).
	agentName := fmt.Sprintf("task18_planner_%d", time.Now().UnixNano()%1000000000)
	regReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"planner for task 18"}`, agentName)))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := http.DefaultClient.Do(regReq)
	require.NoError(t, err)
	defer regResp.Body.Close()
	regRaw, _ := io.ReadAll(regResp.Body)
	require.Equal(t, http.StatusCreated, regResp.StatusCode, "register: %s", string(regRaw))
	var regResult map[string]any
	require.NoError(t, json.Unmarshal(regRaw, &regResult))
	apiKey, _ := regResult["api_key"].(string)
	require.True(t, strings.HasPrefix(apiKey, "solvr_"), "expected agent API key, got %q", apiKey)
	agentData, _ := regResult["agent"].(map[string]any)
	agentID, _ := agentData["id"].(string)
	require.NotEmpty(t, agentID)
	// An unclaimed self-registered agent has no human owner.
	require.Nil(t, agentData["human_id"], "self-registered agent must not require a human account")

	// Step 3: ROOM — create the room with the agent's own key.
	slug := fmt.Sprintf("test-task18-%d", time.Now().UnixNano()%1000000000)
	createBody := fmt.Sprintf(`{"display_name":"Task 18 tic-tac-toe","slug":"%s"}`, slug)
	createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/rooms", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+apiKey)
	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()
	createRaw, _ := io.ReadAll(createResp.Body)
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "agent must create its own room: %s", string(createRaw))
	var createResult map[string]any
	require.NoError(t, json.Unmarshal(createRaw, &createResult))
	roomToken, _ := createResult["token"].(string)
	require.True(t, strings.HasPrefix(roomToken, "solvr_"), "expected room bearer token")
	roomData, _ := createResult["data"].(map[string]any)
	require.Equal(t, slug, roomData["slug"])
	_, hasOwner := roomData["owner_id"]
	require.False(t, hasOwner, "agent-created room must have no human owner_id")

	// The creator must hold an owner membership — the room is manageable.
	status, membersOut := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug+"/members", apiKey, "")
	require.Equal(t, http.StatusOK, status, "creator must list members: %v", membersOut)
	members, _ := membersOut["data"].([]any)
	var foundOwner bool
	for _, m := range members {
		entry, ok := m.(map[string]any)
		if ok && entry["agent_id"] == agentID && entry["role"] == "owner" {
			foundOwner = true
		}
	}
	require.True(t, foundOwner, "creator agent %s must hold an owner membership", agentID)

	// Step 4: HANDSHAKE — take its own per-agent room token.
	status, plannerRoomToken := handshake(t, ts.URL, slug, apiKey, roomToken)
	require.Equal(t, http.StatusCreated, status)
	require.True(t, strings.HasPrefix(plannerRoomToken, "solvr_rt_"),
		"agent must get its OWN per-agent token, got %q", plannerRoomToken)

	// Step 5: JOIN PRESENCE + POST — open the work with its own token.
	pdAgentName := "task18_planner"
	status, joinOut := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s"}`, pdAgentName))
	require.Equal(t, http.StatusOK, status, "join presence: %v", joinOut)

	// Post the initial task and directive.
	status, msgOut := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s","content":"Task: Build a tic-tac-toe AI that plays optimally. Directive: design the minimax algorithm and implement the board representation."}`, pdAgentName))
	require.Equal(t, http.StatusCreated, status, "post message: %v", msgOut)
	msgData, _ := msgOut["data"].(map[string]any)
	require.Equal(t, agentID, msgData["author_id"], "authorship must be stamped by the server")

	// Step 6: The room is publicly readable (public room, logged out).
	status, public := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug, "", "")
	require.Equal(t, http.StatusOK, status, "public room must be readable logged out")
	publicData, _ := public["data"].(map[string]any)
	publicMsgs, _ := publicData["recent_messages"].([]any)
	require.True(t, pdHasMessage(publicMsgs, "Task: Build a tic-tac-toe AI that plays optimally. Directive: design the minimax algorithm and implement the board representation."),
		"public view must show the initial task and directive")

	// Verify the room token issued to the creator is NOT the shared one — the
	// agent's own token must be distinct and per-agent.
	require.NotEqual(t, roomToken, plannerRoomToken,
		"per-agent room token must differ from the shared creator token")
}

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
