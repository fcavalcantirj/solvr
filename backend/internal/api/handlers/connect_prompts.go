package handlers

import (
	"strings"
)

// The prompts a visitor copies.
//
// They are written HERE, not in the browser, because they are the product's
// contract with an agent: the endpoints they name, the identity they demand,
// the visibility they create and the honesty they require are all decisions,
// and decisions live in the API. The panel on the index and the /connect page
// copy the same characters.
//
// Every URL is absolute and points at production, because the agent reading
// the prompt is not running in the browser that copied it. Nothing in a prompt
// may look like a shell or template substitution: the text is pasted into an
// agent conversation, and an agent must never expand it.

const (
	// connectAPIBaseURL and connectAppBaseURL are the production origins an
	// agent and a reader respectively need.
	connectAPIBaseURL = "https://api.solvr.dev"
	connectAppBaseURL = "https://solvr.dev"

	// connectSlugPlaceholder is how a prompt refers to the slug the agent does
	// not have yet. It is deliberately unmistakable for a finished URL.
	connectSlugPlaceholder = "ROOM_SLUG"

	// connectPlannerInstruction and connectStarterInstruction name the agent
	// that must receive the copied prompt.
	connectPlannerInstruction = "Paste this into your planner. It will give you the prompt for your executor."
	connectStarterInstruction = "Paste this into your first agent. It will give you the prompt for its partner."
)

// buildConnectPrompt writes the prompt for one selection.
func buildConnectPrompt(sel ConnectSelection) ConnectPrompt {
	if sel.Preset == ConnectPresetCollaborate {
		return ConnectPrompt{
			Key:         "starter",
			Label:       "Copy starter prompt",
			CopiedLabel: "Copied",
			Instruction: connectStarterInstruction,
			NextStep:    "Your first agent replies with the room link and a complete partner prompt for your second agent.",
			Text:        starterPromptText(sel),
		}
	}

	return ConnectPrompt{
		Key:         "planner",
		Label:       "Copy planner prompt",
		CopiedLabel: "Copied",
		Instruction: connectPlannerInstruction,
		NextStep:    "Your planner replies with the room link and a complete executor prompt for your second agent.",
		Text:        plannerPromptText(sel),
	}
}

// promptTaskSection is the task as the visitor typed it, or the instruction to
// ask for it. An empty field never blocks the copy: the agent asks.
func promptTaskSection(task string) string {
	if task == "" {
		return "TASK\n" +
			"Ask me what we are working on before you create the room, and use my answer as the room's task."
	}
	return "TASK\n" + task
}

// promptVisibilitySection states what the chosen visibility means for the room
// the agent is about to create.
func promptVisibilitySection(visibility string) (isPrivate string, note string) {
	if visibility == ConnectVisibilityPrivate {
		return "true", "This room is private: only agents you admit can take part, and reading it in a browser requires authorized access."
	}
	return "false", "This room is public: anyone can read it, and it can appear in public lists and search engines."
}

// plannerPromptText is the default prompt: one agent opens and owns the room,
// then hands over a complete prompt for the second one.
func plannerPromptText(sel ConnectSelection) string {
	isPrivate, visibilityNote := promptVisibilitySection(sel.Visibility)
	slug := connectSlugPlaceholder

	lines := []string{
		"You are the PLANNER agent in a Solvr room. Everything below is plain HTTPS:",
		"no Solvr CLI, no human account, and no change to your own configuration.",
		"",
		promptTaskSection(sel.Task),
		"",
		"1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none,",
		"   register yourself once:",
		"     POST " + connectAPIBaseURL + "/v1/agents/register",
		`     {"name": "your_agent_name", "description": "what you do"}`,
		"   Keep the api_key it returns (it starts with solvr_) and send it as",
		"   Authorization: Bearer YOUR_AGENT_API_KEY on the /v1 calls below.",
		"",
		"2. ROOM. Create the room you will own:",
		"     POST " + connectAPIBaseURL + "/v1/rooms",
		`     {"display_name": "a short title for the task", "is_private": ` + isPrivate + "}",
		"   " + visibilityNote,
		"   The response carries the room slug. Then take your own per-agent room token:",
		"     POST " + connectAPIBaseURL + "/v1/rooms/" + slug + "/handshake",
		"   The room_token it returns (it starts with solvr_rt_) is yours alone.",
		"   Never share it and never put it in another agent's prompt.",
		"",
		"3. OPEN THE WORK. With Authorization: Bearer YOUR_ROOM_TOKEN:",
		"     POST " + connectAPIBaseURL + "/r/" + slug + "/join",
		`     {"agent_name": "your_agent_name"}`,
		"     POST " + connectAPIBaseURL + "/r/" + slug + "/message",
		`     {"agent_name": "your_agent_name", "content": "the task and your first directive"}`,
		"   Read the replies with GET " + connectAPIBaseURL + "/r/" + slug + "/messages",
		"",
		"4. HAND ME THE SECOND PROMPT. Reply to me with:",
		"   - the room link " + connectAppBaseURL + "/rooms/" + slug + " using the REAL slug,",
		"   - and a complete EXECUTOR PROMPT I can paste into my second agent. It must name",
		"     the room, the executor role, you as the planner and the task, and it must tell",
		"     that agent to use ITS OWN identity: reuse its key or register, run its own",
		"     handshake for its own room token, join, and announce itself in the room.",
		"     Never put your API key or your room token in that prompt.",
		"",
		"5. THEN WORK. Direct the executor in the room, review what it reports, and keep the",
		"   decisions in the room rather than in this chat.",
		"",
		"If any call fails, tell me the exact error. Never invent a room link, and never say",
		"an agent connected when it did not.",
	}

	return strings.Join(lines, "\n")
}

// starterPromptText is the peer shape: two agents share one room and neither
// directs the other.
func starterPromptText(sel ConnectSelection) string {
	isPrivate, visibilityNote := promptVisibilitySection(sel.Visibility)
	slug := connectSlugPlaceholder

	lines := []string{
		"You are the FIRST agent in a shared Solvr room. A second agent will join you as an",
		"equal partner. Everything below is plain HTTPS: no Solvr CLI and no human account.",
		"",
		promptTaskSection(sel.Task),
		"",
		"1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none,",
		"   register yourself once:",
		"     POST " + connectAPIBaseURL + "/v1/agents/register",
		`     {"name": "your_agent_name", "description": "what you do"}`,
		"   Keep the api_key it returns (it starts with solvr_) and send it as",
		"   Authorization: Bearer YOUR_AGENT_API_KEY on the /v1 calls below.",
		"",
		"2. ROOM. Create the shared room:",
		"     POST " + connectAPIBaseURL + "/v1/rooms",
		`     {"display_name": "a short title for the task", "is_private": ` + isPrivate + "}",
		"   " + visibilityNote,
		"   The response carries the room slug. Then take your own per-agent room token:",
		"     POST " + connectAPIBaseURL + "/v1/rooms/" + slug + "/handshake",
		"   The room_token it returns (it starts with solvr_rt_) is yours alone.",
		"   Never share it and never put it in another agent's prompt.",
		"",
		"3. OPEN THE WORK. With Authorization: Bearer YOUR_ROOM_TOKEN:",
		"     POST " + connectAPIBaseURL + "/r/" + slug + "/join",
		`     {"agent_name": "your_agent_name"}`,
		"     POST " + connectAPIBaseURL + "/r/" + slug + "/message",
		`     {"agent_name": "your_agent_name", "content": "the task and how you propose to split it"}`,
		"   Read the replies with GET " + connectAPIBaseURL + "/r/" + slug + "/messages",
		"",
		"4. HAND ME THE SECOND PROMPT. Reply to me with:",
		"   - the room link " + connectAppBaseURL + "/rooms/" + slug + " using the REAL slug,",
		"   - and a complete PARTNER PROMPT I can paste into my other agent. It must name the",
		"     room and the task, say the two of you are peers, and tell that agent to use ITS",
		"     OWN identity: reuse its key or register, run its own handshake for its own room",
		"     token, join, and announce itself in the room.",
		"     Never put your API key or your room token in that prompt.",
		"",
		"5. THEN WORK. Agree the split in the room, do your half, and keep the decisions in",
		"   the room rather than in this chat.",
		"",
		"If any call fails, tell me the exact error. Never invent a room link, and never say",
		"an agent connected when it did not.",
	}

	return strings.Join(lines, "\n")
}
