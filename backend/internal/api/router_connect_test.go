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

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// GET /v1/connect through the real router, called the way a logged-out visitor
// calls it: no Authorization header, no cookie, no account.
//
// The index panel and the /connect page both read this endpoint, so whatever is
// proven here is true of both surfaces.

type connectContract struct {
	InstructionVersion string `json:"instruction_version"`
	Heading            string `json:"heading"`
	IntentField        struct {
		Label    string `json:"label"`
		MaxChars int    `json:"max_chars"`
	} `json:"intent_field"`
	Presets []struct {
		Value    string `json:"value"`
		Label    string `json:"label"`
		Selected bool   `json:"selected"`
		Next     string `json:"next"`
		Prompt   struct {
			Text string `json:"text"`
		} `json:"prompt"`
	} `json:"presets"`
	Selected struct {
		Intent     string `json:"intent"`
		Preset     string `json:"preset"`
		Visibility string `json:"visibility"`
		FlowID     string `json:"flow_id"`
	} `json:"selected"`
	Prompt struct {
		Text      string `json:"text"`
		WordCount int    `json:"word_count"`
		Segments  []struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
		} `json:"segments"`
	} `json:"prompt"`
	Next    string `json:"next"`
	Example struct {
		Kind  string `json:"kind"`
		URL   string `json:"url"`
		Label string `json:"label"`
	} `json:"example"`
}

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

	require.Equal(t, handlers.ConnectInstructionVersion, contract.InstructionVersion, "body: %s", body)
	require.Equal(t, "Connect your agents", contract.Heading)
	require.NotEmpty(t, contract.IntentField.Label)
	require.Greater(t, contract.IntentField.MaxChars, 0)
	require.Len(t, contract.Presets, 3)
	require.Equal(t, "plan-and-build", contract.Selected.Preset)
	require.Equal(t, "public", contract.Selected.Visibility)
	require.True(t, strings.HasPrefix(contract.Prompt.Text, "Learn Solvr from https://solvr.dev/skill.md. Create a public Solvr room"))
	require.Contains(t, contract.Prompt.Text, "join it as the PLANNER, and answer me with a prompt for the EXECUTOR")
	require.Less(t, contract.Prompt.WordCount, 120)
	require.NotEmpty(t, contract.Prompt.Segments)
	require.NotEmpty(t, contract.Next)
	require.NotContains(t, contract.Prompt.Text, contract.Selected.FlowID)

	// No credential, identifier or private detail may ride along with a public
	// contract.
	lower := strings.ToLower(body)
	for _, forbidden := range []string{"solvr_rt_2", "api_key\":", "room_token\":", "owner_id"} {
		require.NotContains(t, lower, strings.ToLower(forbidden), "public contract leaked %q", forbidden)
	}
}

func TestConnectEndpoint_CarriesTheTypedIntentAndTheChosenVisibilityIntoTheSentence(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	contract, body := getConnectContract(t, ts.URL, "intent=Port+the+billing+job+to+the+new+queue&visibility=private")

	require.Equal(t, "Port the billing job to the new queue", contract.Selected.Intent, "body: %s", body)
	require.Contains(t, contract.Prompt.Text, "Create a private Solvr room to Port the billing job to the new queue, join it")
	require.Equal(t, "private", contract.Selected.Visibility)
	require.Contains(t, contract.Prompt.Text, "It's private, so the EXECUTOR gives me its agent id for you to admit.")
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

func TestConnectEndpoint_BuildAndReviewPresetServesBuilderReviewerContract(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	contract, body := getConnectContract(t, ts.URL, "preset=build-and-review&visibility=public")

	require.Equal(t, "build-and-review", contract.Selected.Preset, "body: %s", body)
	require.Len(t, contract.Presets, 3, "three presets must be advertised")

	// The sentence names a builder and a reviewer, not a planner and an executor.
	text := contract.Prompt.Text
	require.Contains(t, text, "join it as the BUILDER, and answer me with a prompt for the REVIEWER")
	require.Contains(t, text, "review and test each change you post, and approve or reject it.")
	lower := strings.ToLower(text)
	require.NotContains(t, lower, "planner")
	require.NotContains(t, lower, "executor")

	// No endpoint, no shell/template placeholder and no localhost: the skill teaches the calls.
	for _, banned := range []string{"api.solvr.dev", "$", "${", "http://localhost"} {
		require.NotContains(t, text, banned)
	}
}

func TestConnectEndpoint_BuildAndReviewRejectsUnknownPreset(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/v1/connect?preset=builder")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body %s", string(body))
}

func TestConnectEndpoint_LinksTheRealPublicExampleRoomItIsPointedAt(t *testing.T) {
	slug := fmt.Sprintf("test-connect-example-%d", time.Now().UnixNano()%1000000)
	t.Setenv("HOMEPAGE_EXAMPLE_ROOM_SLUG", slug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	room, err := db.NewRoomRepository(pool).Create(t.Context(), models.CreateRoomParams{
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
	contract, _ := getConnectContract(t, ts.URL, "intent=Build+a+tic-tac-toe+AI")
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
	_, hasToken := createResult["token"]
	require.False(t, hasToken, "room creation must not hand out a shared room token")
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

	// Step 4: HANDSHAKE — take its own per-agent room token (as the room's owner member).
	status, plannerRoomToken := handshake(t, ts.URL, slug, apiKey, "")
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
}

// roomConnectContract is the response envelope from GET /v1/rooms/{slug}/connect.
type roomConnectContract struct {
	InstructionVersion string `json:"instruction_version"`
	RoomSlug           string `json:"room_slug"`
	RoomURL            string `json:"room_url"`
	Private            bool   `json:"private"`
	Role               string `json:"role"`
	Task               string `json:"task"`
	Prompt             struct {
		Text string `json:"text"`
	} `json:"prompt"`
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

// extractRoomSlug reads the nested "data.slug" from a room creation JSON response
// ({"data": {...}}; no shared room token is returned any more, 000098).
func extractRoomSlug(t *testing.T, raw string) string {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &result))
	data, _ := result["data"].(map[string]any)
	slug, _ := data["slug"].(string)
	require.NotEmpty(t, slug)
	return slug
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
	roomSlug := extractRoomSlug(t, string(createRaw))
	_, plannerRoomToken := handshake(t, ts.URL, roomSlug, agentKey, "")
	require.True(t, strings.HasPrefix(plannerRoomToken, "solvr_rt_"))

	doJSON(t, http.MethodPost, ts.URL+"/r/"+roomSlug+"/join", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s"}`, agentName))
	taskContent := "Task: Build a distributed key-value store. Directive: design the sharding strategy."
	st, _ := doJSON(t, http.MethodPost, ts.URL+"/r/"+roomSlug+"/message", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s","content":"%s"}`, agentName, taskContent))
	require.Equal(t, http.StatusCreated, st)

	// Now an anonymous visitor fetches the executor prompt.
	contract, body := getRoomConnectContract(t, ts.URL, roomSlug)

	require.Equal(t, handlers.ConnectInstructionVersion, contract.InstructionVersion, "body: %s", body)
	require.Equal(t, roomSlug, contract.RoomSlug)
	require.Equal(t, "https://solvr.dev/rooms/"+roomSlug, contract.RoomURL)
	require.False(t, contract.Private)
	require.Equal(t, "executor", contract.Role)
	require.Contains(t, contract.Prompt.Text, "https://solvr.dev/rooms/"+roomSlug+" as the EXECUTOR", "the sentence names the real room")
	require.NotContains(t, contract.Prompt.Text, "ROOM_SLUG", "the sentence must not contain the placeholder")
	require.Contains(t, contract.Task, "distributed key-value store")
	require.Contains(t, contract.Task, "sharding strategy")

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

	roomSlug := extractRoomSlug(t, string(createRaw))
	_, plannerRoomToken := handshake(t, ts.URL, roomSlug, plannerKey, "")
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
	require.Contains(t, contract.Prompt.Text, "https://solvr.dev/rooms/"+roomSlug)
	require.NotContains(t, contract.Prompt.Text, "ROOM_SLUG")

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

// TestConnectEndpoint_MultipleAgentsCanJoinViaAddAgentPrompt verifies that N agents
// can join the same room by role (GET /v1/rooms/{slug}/connect?role=). Each agent establishes its
// own identity and presence without overwriting a fixed executor slot. This is the
// behavioral requirement of task 15, step 3: agents must be able to reuse the generic
// join prompt without fixed slots.
func TestConnectEndpoint_MultipleAgentsCanJoinViaAddAgentPrompt(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_n_agent_%'") //nolint:errcheck
	})

	// SETUP: Planner creates a room.
	plannerName := fmt.Sprintf("planner_nagent_%d", time.Now().UnixNano()%1000000000)
	regReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"planner for n-agent test"}`, plannerName)))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, _ := http.DefaultClient.Do(regReq)
	regRaw, _ := io.ReadAll(regResp.Body)
	regResp.Body.Close()
	var regResult map[string]any
	json.Unmarshal(regRaw, &regResult)
	plannerKey, _ := regResult["api_key"].(string)
	agentData, _ := regResult["agent"].(map[string]any)
	plannerID, _ := agentData["id"].(string)

	// Create room.
	slug := fmt.Sprintf("test-n-agent-%d", time.Now().UnixNano()%1000000000)
	createBody := fmt.Sprintf(`{"display_name":"N-Agent Test Room","slug":"%s"}`, slug)
	createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/rooms", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+plannerKey)
	createResp, _ := http.DefaultClient.Do(createReq)
	createRaw, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "create room: %s", string(createRaw))

	// Planner joins.
	_, plannerRoomToken := handshake(t, ts.URL, slug, plannerKey, "")
	status, _ := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s"}`, plannerName))
	require.Equal(t, http.StatusOK, status)

	// Now register and join N executors, each by its role sentence.
	// The skill tells each agent to:
	// 1. Register (if needed)
	// 2. POST /v1/rooms/{slug}/handshake to get its own per-agent token
	// 3. POST /r/{slug}/join to establish presence
	// 4. POST messages to the room
	const numExecutors = 3
	executors := make(map[string]string) // agentID -> agentKey
	executorTokens := make(map[string]string) // agentID -> perAgentToken

	for i := 1; i <= numExecutors; i++ {
		// Step 1: Each executor self-registers.
		execName := fmt.Sprintf("executor_%d_nagent_%d", i, time.Now().UnixNano()%1000000000)
		regReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/agents/register",
			strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"executor %d for n-agent test"}`, execName, i)))
		regReq.Header.Set("Content-Type", "application/json")
		regResp, _ := http.DefaultClient.Do(regReq)
		regRaw, _ := io.ReadAll(regResp.Body)
		regResp.Body.Close()
		var execRegResult map[string]any
		json.Unmarshal(regRaw, &execRegResult)
		execKey, _ := execRegResult["api_key"].(string)
		execData, _ := execRegResult["agent"].(map[string]any)
		execID, _ := execData["id"].(string)
		executors[execID] = execKey
		executorTokens[execID] = ""

		// Step 2: Each executor handshakes with the room (no shared token needed for public room).
		status, execPerAgentToken := handshake(t, ts.URL, slug, execKey, "")
		require.Equal(t, http.StatusCreated, status, "executor %d handshake failed", i)
		require.NotEmpty(t, execPerAgentToken, "executor %d must receive per-agent token", i)
		executorTokens[execID] = execPerAgentToken

		// Step 3: Each executor joins presence.
		status, joinOut := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", execPerAgentToken,
			fmt.Sprintf(`{"agent_name":"%s"}`, execName))
		require.Equal(t, http.StatusOK, status, "executor %d join failed: %v", i, joinOut)

		// Step 4: Each executor posts a message.
		status, msgOut := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", execPerAgentToken,
			fmt.Sprintf(`{"agent_name":"%s","content":"Executor %d is ready"}`, execName, i))
		require.Equal(t, http.StatusCreated, status, "executor %d post failed: %v", i, msgOut)
		msgData, _ := msgOut["data"].(map[string]any)
		// The server stamps the authoritative author_id based on the authenticated token.
		require.Equal(t, execID, msgData["author_id"], "executor %d authorship must be stamped by server", i)
	}

	// VERIFICATION 1: All executors are members of the room.
	status, membersOut := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug+"/members", plannerKey, "")
	require.Equal(t, http.StatusOK, status)
	members, _ := membersOut["data"].([]any)
	memberIDs := make(map[string]bool)
	for _, m := range members {
		entry, ok := m.(map[string]any)
		if ok {
			agentID, _ := entry["agent_id"].(string)
			memberIDs[agentID] = true
		}
	}
	require.True(t, memberIDs[plannerID], "planner must be a member")
	for execID := range executors {
		require.True(t, memberIDs[execID], "executor %s must be a member", execID)
	}
	require.Equal(t, 1+numExecutors, len(memberIDs), "room must have exactly planner + %d executors", numExecutors)

	// VERIFICATION 2: All agents' messages appear in the room with correct authorship.
	status, roomOut := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug, "", "")
	require.Equal(t, http.StatusOK, status)
	roomData, _ := roomOut["data"].(map[string]any)
	recentMsgs, _ := roomData["recent_messages"].([]any)
	authorIDs := make(map[string]int) // count messages per author_id
	for _, msg := range recentMsgs {
		msgEntry, ok := msg.(map[string]any)
		if ok {
			authorID, _ := msgEntry["author_id"].(string)
			authorIDs[authorID]++
		}
	}
	// Each executor should have exactly 1 message (the one they posted).
	for execID := range executors {
		require.Equal(t, 1, authorIDs[execID], "executor %s must have exactly 1 message in room", execID)
	}

	// VERIFICATION 3: No overwriting of slots — each agent maintains distinct identity.
	// If there were a fixed "executor slot" that was being overwritten, we would see
	// fewer than numExecutors distinct author_ids.
	distinctAuthors := 0
	for _, count := range authorIDs {
		if count > 0 {
			distinctAuthors++
		}
	}
	require.GreaterOrEqual(t, distinctAuthors, numExecutors, "must have at least %d distinct authors (no fixed slot overwriting)", numExecutors)
}

// TestConnectEndpoint_PrivateRoomRequiresOwnerAdmissionForEachAgent verifies that in
// a private room, each new agent identity must be explicitly admitted by the owner.
// Removing one agent does not affect others. This is task 15, step 4.
func TestConnectEndpoint_PrivateRoomRequiresOwnerAdmissionForEachAgent(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_priv_%'") //nolint:errcheck
	})

	// SETUP: Create a private room.
	plannerName := fmt.Sprintf("owner_priv_%d", time.Now().UnixNano()%1000000000)
	regReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"private room owner"}`, plannerName)))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, _ := http.DefaultClient.Do(regReq)
	regRaw, _ := io.ReadAll(regResp.Body)
	regResp.Body.Close()
	var regResult map[string]any
	json.Unmarshal(regRaw, &regResult)
	ownerKey, _ := regResult["api_key"].(string)

	// Create private room.
	slug := fmt.Sprintf("test-priv-room-%d", time.Now().UnixNano()%1000000000)
	createBody := fmt.Sprintf(`{"display_name":"Private Room","slug":"%s","is_private":true}`, slug)
	createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/rooms", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+ownerKey)
	createResp, _ := http.DefaultClient.Do(createReq)
	createRaw, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "create room: %s", string(createRaw))

	// Owner joins.
	_, ownerRoomToken := handshake(t, ts.URL, slug, ownerKey, "")
	doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", ownerRoomToken, `{"agent_name":"owner"}`)

	// TEST: Register two agents.
	agent1Name := fmt.Sprintf("agent1_priv_%d", time.Now().UnixNano()%1000000000)
	regReq, _ = http.NewRequest(http.MethodPost, ts.URL+"/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"agent 1"}`, agent1Name)))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, _ = http.DefaultClient.Do(regReq)
	regRaw, _ = io.ReadAll(regResp.Body)
	regResp.Body.Close()
	var agent1RegResult map[string]any
	json.Unmarshal(regRaw, &agent1RegResult)
	agent1Key, _ := agent1RegResult["api_key"].(string)
	agent1Data, _ := agent1RegResult["agent"].(map[string]any)
	agent1ID, _ := agent1Data["id"].(string)

	agent2Name := fmt.Sprintf("agent2_priv_%d", time.Now().UnixNano()%1000000000)
	regReq, _ = http.NewRequest(http.MethodPost, ts.URL+"/v1/agents/register",
		strings.NewReader(fmt.Sprintf(`{"name":"%s","description":"agent 2"}`, agent2Name)))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, _ = http.DefaultClient.Do(regReq)
	regRaw, _ = io.ReadAll(regResp.Body)
	regResp.Body.Close()
	var agent2RegResult map[string]any
	json.Unmarshal(regRaw, &agent2RegResult)
	agent2Key, _ := agent2RegResult["api_key"].(string)
	agent2Data, _ := agent2RegResult["agent"].(map[string]any)
	agent2ID, _ := agent2Data["id"].(string)

	// VERIFICATION 1: Unadmitted agents cannot handshake or join.
	status, _ := handshake(t, ts.URL, slug, agent1Key, "")
	require.Equal(t, http.StatusForbidden, status, "unadmitted agent must be denied")

	// VERIFICATION 2: Owner admits agent1 only (not agent2).
	status, _ = doJSON(t, http.MethodPost, ts.URL+"/v1/rooms/"+slug+"/members", ownerKey,
		fmt.Sprintf(`{"agent_id":"%s"}`, agent1ID))
	require.Equal(t, http.StatusCreated, status)

	// Agent1 can now handshake and join.
	status, agent1Token := handshake(t, ts.URL, slug, agent1Key, "")
	require.Equal(t, http.StatusCreated, status, "admitted agent must handshake successfully")
	status, _ = doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", agent1Token, `{"agent_name":"agent1"}`)
	require.Equal(t, http.StatusOK, status, "admitted agent must join successfully")

	// Agent2 still cannot handshake (not yet admitted).
	status, _ = handshake(t, ts.URL, slug, agent2Key, "")
	require.Equal(t, http.StatusForbidden, status, "unadmitted agent2 must still be denied")

	// VERIFICATION 3: Owner admits agent2.
	status, _ = doJSON(t, http.MethodPost, ts.URL+"/v1/rooms/"+slug+"/members", ownerKey,
		fmt.Sprintf(`{"agent_id":"%s"}`, agent2ID))
	require.Equal(t, http.StatusCreated, status)

	// Agent2 can now handshake and join.
	status, agent2Token := handshake(t, ts.URL, slug, agent2Key, "")
	require.Equal(t, http.StatusCreated, status)
	status, _ = doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", agent2Token, `{"agent_name":"agent2"}`)
	require.Equal(t, http.StatusOK, status)

	// VERIFICATION 4: Owner revokes agent1. Agent1 loses access, agent2 unaffected.
	status, _ = doJSON(t, http.MethodDelete, ts.URL+"/v1/rooms/"+slug+"/members/"+agent1ID, ownerKey, "")
	require.Equal(t, http.StatusNoContent, status)

	// Agent1 can no longer access the room.
	status, _ = doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug, agent1Key, "")
	require.Equal(t, http.StatusForbidden, status, "revoked agent loses access")

	// Agent2 still has full access.
	status, _ = doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug, agent2Key, "")
	require.Equal(t, http.StatusOK, status, "other agents remain unaffected")
	
	// Agent2 can still post via its per-agent token.
	status, msgOut := doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", agent2Token, `{"agent_name":"agent2","content":"still here"}`)
	require.Equal(t, http.StatusCreated, status, "unaffected agent can still post")
	msgData, _ := msgOut["data"].(map[string]any)
	require.Equal(t, agent2ID, msgData["author_id"])
}
