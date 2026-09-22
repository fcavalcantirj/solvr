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

// roomConnectContract is the response envelope from GET /v1/rooms/{slug}/connect.
type roomConnectContract struct {
	InstructionVersion string `json:"instruction_version"`
	RoomSlug           string `json:"room_slug"`
	RoomURL            string `json:"room_url"`
	Private            bool   `json:"private"`
	Task               string `json:"task"`
	ExpectedPlanner    string `json:"expected_planner_identity"`
	ExecutorPrompt     string `json:"executor_prompt"`
	FirstMessageID     int64  `json:"first_message_id"`
	FirstMessageURL    string `json:"first_message_url"`
}

// getRoomConnectContract calls the room-specific connect endpoint with no credentials.
func getRoomConnectContract(t *testing.T, baseURL, slug string) (roomConnectContract, string) {
	t.Helper()
	resp, err := http.Get(baseURL + "/v1/rooms/" + slug + "/connect")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var wrapper struct {
		Data roomConnectContract `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &wrapper), "body: %s", string(body))
	return wrapper.Data, string(body)
}

// extractRoomToken reads the "token" field from a room creation JSON response.
func extractRoomToken(t *testing.T, raw string) string {
	t.Helper()
	token, _ := extractRoomTokenAndSlug(t, raw)
	return token
}

// extractRoomTokenAndSlug reads both the "token" and the nested "data.slug" from a
// room creation JSON response (which returns {"data": {...}, "token": ...}).
func extractRoomTokenAndSlug(t *testing.T, raw string) (string, string) {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &result))
	token, _ := result["token"].(string)
	require.NotEmpty(t, token)
	data, _ := result["data"].(map[string]any)
	slug, _ := data["slug"].(string)
	require.NotEmpty(t, slug)
	return token, slug
}

// messageContains scans a list of message maps (as returned by the room endpoint)
// and reports whether any message's content contains the given substring.
func messageContains(messages []any, needle string) bool {
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if content, _ := msg["content"].(string); strings.Contains(content, needle) {
			return true
		}
	}
	return false
}

// TestConnectEndpoint_RoomInstructionsEndpoint verifies the GET
// /v1/rooms/{slug}/connect endpoint serves room-specific executor instructions to a
// logged-out visitor for a public room: real slug (not ROOM_SLUG placeholder), real
// planner identity and task from the first message, real room URL, and NO credentials.
func TestConnectEndpoint_RoomInstructionsEndpoint(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_task19%'") //nolint:errcheck
	})

	// Create a public room with a registered planner agent.
	agentName := fmt.Sprintf("task19planner%d", time.Now().UnixNano()%1000000000)
	_, agentKey := registerTestAgent(t, ts, agentName)

	// Slug is auto-generated from display_name via slugify (no underscore allowed).
	createBody := fmt.Sprintf(`{"display_name":"Task 19 room %d"}`, time.Now().UnixNano()%1000000000)
	createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/rooms", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+agentKey)
	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()
	createRaw, _ := io.ReadAll(createResp.Body)
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "create room: %s", string(createRaw))

	// Handshake to get the per-agent token, then post the initial task + directive.
	sharedToken, roomSlug := extractRoomTokenAndSlug(t, string(createRaw))
	_, plannerRoomToken := handshake(t, ts.URL, roomSlug, agentKey, sharedToken)
	require.True(t, strings.HasPrefix(plannerRoomToken, "solvr_rt_"))

	doJSON(t, http.MethodPost, ts.URL+"/r/"+roomSlug+"/join", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s"}`, agentName))
	taskContent := "Task: Build a distributed key-value store. Directive: design the sharding strategy."
	st, _ := doJSON(t, http.MethodPost, ts.URL+"/r/"+roomSlug+"/message", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s","content":"%s"}`, agentName, taskContent))
	require.Equal(t, http.StatusCreated, st)

	// Now an anonymous visitor fetches the executor prompt.
	contract, body := getRoomConnectContract(t, ts.URL, roomSlug)

	require.Equal(t, "1.0", contract.InstructionVersion, "body: %s", body)
	require.Equal(t, roomSlug, contract.RoomSlug)
	require.Equal(t, "https://solvr.dev/rooms/"+roomSlug, contract.RoomURL)
	require.False(t, contract.Private)
	require.NotEmpty(t, contract.ExecutorPrompt)
	require.Contains(t, contract.ExecutorPrompt, roomSlug, "prompt must contain the real slug")
	require.NotContains(t, contract.ExecutorPrompt, "ROOM_SLUG", "prompt must not contain the placeholder")
	require.Equal(t, agentName, contract.ExpectedPlanner, "expected planner = first message author")
	require.Contains(t, contract.Task, "distributed key-value store")
	require.Contains(t, contract.Task, "sharding strategy")
	require.Greater(t, contract.FirstMessageID, int64(0))
	require.Contains(t, contract.FirstMessageURL, roomSlug)

	// No credentials in the raw response body.
	lower := strings.ToLower(body)
	for _, secret := range []string{"solvr_sk_", "solvr_rt_", "solvr_rm_", "api_key", "room_token", "token_hash"} {
		require.NotContains(t, lower, secret, "leaked %q in room connect response", secret)
	}
}

// TestConnectEndpoint_RoomInstructionsEndpoint_PrivateRoomIs403 verifies that a private
// room returns 403 to an anonymous visitor (same read policy as room detail).
func TestConnectEndpoint_RoomInstructionsEndpoint_PrivateRoomIs403(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createClosedRoom(t, ts, jwt) // private room

	resp, err := http.Get(ts.URL + "/v1/rooms/" + slug + "/connect")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestConnectEndpoint_RoomInstructionsEndpoint_NonexistentRoomIs404 verifies a missing
// room returns 404.
func TestConnectEndpoint_RoomInstructionsEndpoint_NonexistentRoomIs404(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/v1/rooms/does-not-exist-12345/connect")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestConnectEndpoint_ExecutorPromptEndToEnd verifies the full executor journey:
// the planner creates a public room and posts the task, the executor fetches the
// room-specific prompt from /v1/rooms/{slug}/connect, then follows it to
// handshake, join, and post a plan — all without a human relaying messages.
func TestConnectEndpoint_ExecutorPromptEndToEnd(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_task19%'")
	})

	// --- PLANNER side: register, create room, handshake, join, post task + directive ---
	plannerName := fmt.Sprintf("task19ple2e%d", time.Now().UnixNano()%1000000000)
	_, plannerKey := registerTestAgent(t, ts, plannerName)

	createBody := fmt.Sprintf(`{"display_name":"E2E room %d"}`, time.Now().UnixNano()%1000000000)
	createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/rooms", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+plannerKey)
	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()
	createRaw, _ := io.ReadAll(createResp.Body)
	require.Equal(t, http.StatusCreated, createResp.StatusCode, string(createRaw))

	sharedToken, roomSlug := extractRoomTokenAndSlug(t, string(createRaw))
	_, plannerRoomToken := handshake(t, ts.URL, roomSlug, plannerKey, sharedToken)
	require.True(t, strings.HasPrefix(plannerRoomToken, "solvr_rt_"))

	doJSON(t, http.MethodPost, ts.URL+"/r/"+roomSlug+"/join", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s"}`, plannerName))
	taskContent := "Task: Build a todo list CLI. Directive: design the command structure."
	doJSON(t, http.MethodPost, ts.URL+"/r/"+roomSlug+"/message", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s","content":"%s"}`, plannerName, taskContent))

	// --- EXECUTOR side: GET the room-specific prompt ---
	contract, conBody := getRoomConnectContract(t, ts.URL, roomSlug)
	require.NotEmpty(t, conBody)
	require.Equal(t, roomSlug, contract.RoomSlug)
	require.Equal(t, plannerName, contract.ExpectedPlanner)
	require.Contains(t, contract.ExecutorPrompt, roomSlug)
	require.NotContains(t, contract.ExecutorPrompt, "ROOM_SLUG")

	// --- EXECUTOR side: follow the prompt — self-register, handshake, join, post plan ---
	execName := fmt.Sprintf("task19exe%d", time.Now().UnixNano()%1000000000)
	_, execKey := registerTestAgent(t, ts, execName)

	_, execRoomToken := handshake(t, ts.URL, roomSlug, execKey, "")
	require.True(t, strings.HasPrefix(execRoomToken, "solvr_rt_"), "executor must get its own per-agent token")
	require.NotEqual(t, plannerRoomToken, execRoomToken, "executor token must differ from planner token")

	doJSON(t, http.MethodPost, ts.URL+"/r/"+roomSlug+"/join", execRoomToken,
		fmt.Sprintf(`{"agent_name":"%s"}`, execName))

	execMsg := fmt.Sprintf(`{"agent_name":"%s","content":"PLAN: I will build the todo list CLI in three steps: 1) parse commands, 2) manage items, 3) persist state."}`, execName)
	st, out := doJSON(t, http.MethodPost, ts.URL+"/r/"+roomSlug+"/message", execRoomToken, execMsg)
	require.Equal(t, http.StatusCreated, st, "executor posts plan: %v", out)

	// --- VERIFY: both agents appear in the room, messages exchange both ways ---
	st, roomData := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+roomSlug, "", "")
	require.Equal(t, http.StatusOK, st, "public room must be readable logged out")
	roomMap, _ := roomData["data"].(map[string]any)
	messages, _ := roomMap["recent_messages"].([]any)
	require.GreaterOrEqual(t, len(messages), 2, "both planner and executor messages must be in the transcript")

	foundPlanner := false
	foundExecutor := false
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		author, _ := msg["agent_name"].(string)
		if author == plannerName {
			foundPlanner = true
		}
		if author == execName {
			foundExecutor = true
		}
	}
	require.True(t, foundPlanner, "planner message must be visible in the room")
	require.True(t, foundExecutor, "executor message must be visible in the room")

	// --- Verify the planner can read the executor's plan ---
	st, plannerView := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+roomSlug, plannerKey, "")
	require.Equal(t, http.StatusOK, st)
	plannerMsgs, _ := plannerView["data"].(map[string]any)["recent_messages"].([]any)
	require.True(t, messageContains(plannerMsgs, "PLAN:"),
		"planner can read the executor's plan")

	// --- Verify the executor can read the planner's directive (two-way exchange) ---
	st, execView := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+roomSlug, execKey, "")
	require.Equal(t, http.StatusOK, st)
	execMsgs, _ := execView["data"].(map[string]any)["recent_messages"].([]any)
	require.True(t, messageContains(execMsgs, "design the command structure"),
		"executor can read the planner's directive")
}

// TestConnectEndpoint_ExecutorPromptNamesOnlyRealRoutes verifies that every production
// URL named in the executor prompt is a route the API actually serves (walks the chi
// router to build the served set, same pattern as
// TestConnectEndpoint_PromptsOnlyNameRoutesThisAPIActuallyServes).
func TestConnectEndpoint_ExecutorPromptNamesOnlyRealRoutes(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_task19%'")
	})

	agentName := fmt.Sprintf("task19routecheck%d", time.Now().UnixNano()%1000000000)
	_, agentKey := registerTestAgent(t, ts, agentName)

	createBody := fmt.Sprintf(`{"display_name":"Route check room %d"}`, time.Now().UnixNano()%1000000000)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/rooms", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+agentKey)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(respBody))

	sharedToken, slug := extractRoomTokenAndSlug(t, string(respBody))
	_, roomToken := handshake(t, ts.URL, slug, agentKey, sharedToken)
	doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", roomToken, fmt.Sprintf(`{"agent_name":"%s"}`, agentName))
	st, _ := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", roomToken,
		fmt.Sprintf(`{"agent_name":"%s","content":"Task: route check directive."}`, agentName))
	require.Equal(t, http.StatusCreated, st)

	// Fetch the room connect endpoint and extract the executor prompt.
	contract, _ := getRoomConnectContract(t, ts.URL, slug)
	require.Contains(t, contract.ExecutorPrompt, slug)

	// Walk the real router to get all served routes.
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(context.Background(), registry, slog.Default(), 0)
	router := NewRouter(pool, hubMgr, registry)
	served := map[string]bool{}
	require.NoError(t, chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		served[method+" "+strings.TrimSuffix(route, "/")] = true
		return nil
	}))

	matches := promptEndpointRE.FindAllStringSubmatch(contract.ExecutorPrompt, -1)
	require.NotEmpty(t, matches, "executor prompt names no endpoint at all")
	for _, m := range matches {
		method, path := m[1], m[2]
		// The executor prompt names the REAL slug; chi reports the route template
		// with {slug}, so substitute before matching.
		path = strings.ReplaceAll(path, slug, "{slug}")
		require.True(t, served[method+" "+strings.TrimSuffix(path, "/")],
			"executor prompt tells the agent to call %s %s, which this API does not serve", method, path)
	}
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
			// The prompt's ROOM_SLUG placeholder maps to the router's {slug} parameter.
			path = strings.ReplaceAll(path, "ROOM_SLUG", "{slug}")
			key := method + " " + strings.TrimSuffix(path, "/")
			require.True(t, served[key],
				"prompt (query %q) tells the agent to call %s, which this API does not serve", query, key)
		}
	}
}
