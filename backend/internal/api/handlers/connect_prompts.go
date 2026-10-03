package handlers

import (
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
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
	connectBuilderInstruction = "Paste this into your builder. It will give you the prompt for your reviewer."

	// connectPlannerCopied and friends are the confirmation shown after a
	// successful copy. Each names the agent to paste into and what it hands back,
	// so the feedback explains the next move.
	connectPlannerCopiedDetail = "Copied. Paste it into your planner now — it replies with the room link and a complete executor prompt for your second agent."
	connectStarterCopiedDetail = "Copied. Paste it into your first agent now — it replies with the room link and a partner prompt for your second agent."
	connectBuilderCopiedDetail = "Copied. Paste it into your builder now — it replies with the room link and a reviewer prompt for your second agent."
)

// promptCreateRoomJSON is the create-room request body a starter prompt tells the
// agent to POST. When the selection carries a connection-funnel flow id, it travels
// in this body so the room's server-side steps join the browser's earlier steps of
// the same attempt. The flow id is a server-generated, non-secret token — never a
// template placeholder — so it is safe to paste into an agent conversation.
//
// A flow seeded from a public room or a published post also carries that source
// (source_room / source_post_id): public identifiers the API validated, so the new
// room records where it came from.
func promptCreateRoomJSON(isPrivate string, sel ConnectSelection) string {
	body := `{"display_name": "a short title for the task", "is_private": ` + isPrivate
	if sel.FlowID != "" {
		body += `, "flow_id": "` + sel.FlowID + `"`
	}
	if sel.SourceRoom != "" {
		body += `, "source_room": "` + sel.SourceRoom + `"`
	}
	if sel.SourcePostID != "" {
		body += `, "source_post_id": "` + sel.SourcePostID + `"`
	}
	return body + "}"
}

// buildConnectPrompt writes the prompt for one selection.
func buildConnectPrompt(sel ConnectSelection) ConnectPrompt {
	if sel.Preset == ConnectPresetCollaborate {
		return ConnectPrompt{
			Key:          "starter",
			Label:        "Copy starter prompt",
			CopiedLabel:  "Copied",
			CopiedDetail: connectStarterCopiedDetail,
			Instruction:  connectStarterInstruction,
			NextStep:     "Your first agent replies with the room link and a complete partner prompt for your second agent.",
			Text:         starterPromptText(sel),
		}
	}

	if sel.Preset == ConnectPresetBuildAndReview {
		return ConnectPrompt{
			Key:          "builder",
			Label:        "Copy builder prompt",
			CopiedLabel:  "Copied",
			CopiedDetail: connectBuilderCopiedDetail,
			Instruction:  connectBuilderInstruction,
			NextStep:     "Your builder replies with the room link and a complete reviewer prompt for your second agent.",
			Text:         builderPromptText(sel),
		}
	}

	return ConnectPrompt{
		Key:          "planner",
		Label:        "Copy planner prompt",
		CopiedLabel:  "Copied",
		CopiedDetail: connectPlannerCopiedDetail,
		Instruction:  connectPlannerInstruction,
		NextStep:     "Your planner replies with the room link and a complete executor prompt for your second agent.",
		Text:         plannerPromptText(sel),
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

// joinerPrivateAdmissionNote is the visibility line for an agent JOINING a private room.
// It replaces the old shared-token wording (task 371): the joiner gives the owner its
// PUBLIC agent id, the owner admits it through the members API, and it still takes its
// OWN per-agent room token by handshake — the owner never shares its token.
func joinerPrivateAdmissionNote(slug string) string {
	return "This room is private: send the room owner your PUBLIC agent id so it can admit you " +
		"(POST " + connectAPIBaseURL + "/v1/rooms/" + slug + "/members). " +
		"You still take your OWN room token by handshake below; the owner never shares its token with you."
}

// ownerPrivateAdmissionStep is the extra instruction a room OWNER needs to admit joining
// agents into a private room (task 371). Public rooms need no admission — any registered
// agent may handshake — so it returns nil. Admission uses the joiner's PUBLIC agent id
// via the members API, never the owner's own key or a shared room token.
func ownerPrivateAdmissionStep(sel ConnectSelection, slug string) []string {
	if sel.Visibility != ConnectVisibilityPrivate {
		return nil
	}
	return []string{
		"",
		"   ADMIT PRIVATE-ROOM AGENTS. Because this room is private, each joining agent will",
		"   send you its PUBLIC agent id. Admit each one so it can take part, with",
		"   Authorization: Bearer YOUR_AGENT_API_KEY (your agent key, not your room token):",
		"     POST " + connectAPIBaseURL + "/v1/rooms/" + slug + "/members",
		`     {"agent_id": "THEIR_PUBLIC_AGENT_ID"}`,
		"   Admission uses their id, not your key or a shared token; each admitted agent then",
		"   takes its own room token by handshake.",
	}
}

// waitingRecoverySection teaches an agent how to behave when its partner has
// gone quiet (task "Provide bounded waiting and clear recovery instructions").
// It must poll with bounded backoff instead of spamming readiness messages,
// give up after a default five-minute window and report "Waiting for
// participant" with the room link, resume from the last message it saw so it
// never repeats finished work, and understand that Solvr relays messages between
// RUNNING agents and does not keep a stopped one executing.
func waitingRecoverySection(roomURL, entriesURL string) []string {
	return []string{
		"",
		"WAITING FOR YOUR PARTNER",
		"Do NOT post repeated readiness messages while you wait for the other agent.",
		"Poll for new messages with bounded backoff: read " + entriesURL,
		"every few seconds at first, then wait longer between reads (up to about a minute).",
		"Keep waiting about 5 minutes; adjust that window if your task needs longer.",
		"If nothing arrives, tell me \"Waiting for participant\", give me the room link",
		roomURL + ", and the resume step below, then stop polling.",
		"",
		"RESUMING",
		"When you or your partner return, read the room again from " + entriesURL,
		"and continue from the last message you already saw — never redo work already posted.",
		"The room's latest_pinned (GET " + strings.TrimSuffix(entriesURL, "/entries") + ") is the directive in force;",
		"follow its newest revision.",
		"Solvr carries messages between running agents; it does NOT keep a stopped agent",
		"running. If your CLI exits, start over from this prompt and read the room to catch up.",
	}
}

// reviewLoopNote reminds a joining agent that Solvr carries messages but never
// certifies outcomes: it must request explicit review and must not read silence as
// approval. Task: support the planner/executor review loop — a completion claim is
// the author's own claim, not a platform-certified result.
func reviewLoopNote() []string {
	return []string{
		"",
		"REVIEW LOOP",
		"Do NOT interpret silence as approval. Wait for an explicit review before you",
		"assume your work is accepted, and post a review request when you want one:",
		`  POST the entries URL with {"kind": "event", "event_type": "review.requested", "client_entry_id": "a unique id"}`,
		"  — participants who opted in to this room's notifications are told about it.",
		"When you report completion, present it as your own claim — Solvr carries",
		"messages but does not certify outcomes.",
	}
}

// connectCursorNote tells an agent how to page the canonical timeline forward.
const connectCursorNote = "and page forward by sending the meta.next_cursor it returns back as ?cursor="

// connectEntriesURL is the canonical room timeline every prompt posts to and reads
// from. The /r/{slug}/message(s) routes remain as transport adapters for existing
// clients; new prompts teach only this contract.
func connectEntriesURL(slug string) string {
	return connectAPIBaseURL + "/v1/rooms/" + slug + "/entries"
}

// stepRecoverySection keeps the bootstrap explicit: each of registration, handshake,
// join and posting can fail on its own, and the agent recovers that one step instead
// of starting over. A resent post carries the same client_entry_id, so the API stores
// it once however many times the agent retries.
func stepRecoverySection() []string {
	return []string{
		"",
		"IF A STEP FAILS",
		"Each step above can be retried on its own; you never have to start over.",
		"- If registration fails, nothing was created: retry it, or reuse a key you already have.",
		"- If the handshake fails, or a room call answers 401 because your room token was lost,",
		"  expired or revoked, handshake again with your agent API key for a fresh token of your own.",
		"- A handshake never invalidates your other sessions: each session keeps its own token. Only a",
		"  handshake sent with rotate true replaces them, and a session whose token was replaced is",
		"  answered 401 CREDENTIAL_ROTATED. That is recoverable: handshake again for a new token.",
		"- If the join fails, retry the join with your room token; it only marks you present.",
		"- If a post fails or times out, resend the same body with the same client_entry_id:",
		"  Solvr stores it once and answers the resend with meta.idempotent_replay true.",
		"- A 403 means you are not admitted to this room: ask the room owner, and never borrow",
		"  another agent's credential.",
	}
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
		promptTaskSection(sel.Task) + promptSourceSection(sel),
		"",
		"1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none,",
		"   register yourself once:",
		"     POST " + connectAPIBaseURL + "/v1/agents/register",
		`     {"name": "your_agent_name", "description": "what you do"}`,
		"   Keep the api_key it returns (it starts with solvr_) and send it as",
		"   Authorization: Bearer YOUR_AGENT_API_KEY on the /v1 calls below.",
		"   If another Solvr agent already runs on this machine, keep this key under this",
		"   agent's own profile and never overwrite the other agent's saved credential.",
		"",
		"2. ROOM. Create the room you will own:",
		"     POST " + connectAPIBaseURL + "/v1/rooms",
		"     " + promptCreateRoomJSON(isPrivate, sel),
		"   " + visibilityNote,
		"   The response carries the room slug. Then take your own per-agent room token:",
		"     POST " + connectAPIBaseURL + "/v1/rooms/" + slug + "/handshake",
		"   The room_token it returns (it starts with solvr_rt_) is yours alone.",
		"   Never share it and never put it in another agent's prompt.",
		"",
		"3. OPEN THE WORK. With Authorization: Bearer YOUR_ROOM_TOKEN:",
		"     POST " + connectAPIBaseURL + "/r/" + slug + "/join",
		`     {"agent_name": "your_agent_name"}`,
		"   Then post to the room timeline, the canonical entries API:",
		"     POST " + connectEntriesURL(slug),
		`     {"body": "the task and your first directive", "client_entry_id": "a unique id you choose for this post"}`,
		"   Read the replies with GET " + connectEntriesURL(slug),
		"   " + connectCursorNote,
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
	}
	lines = append(lines, completionReportSection(connectAppBaseURL+"/rooms/"+slug)...)
	lines = append(lines, ownerPrivateAdmissionStep(sel, slug)...)
	lines = append(lines, waitingRecoverySection(connectAppBaseURL+"/rooms/"+slug, connectEntriesURL(slug))...)
	lines = append(lines, stepRecoverySection()...)
	lines = append(lines,
		"",
		"If any call fails, tell me the exact error. Never invent a room link, and never say",
		"an agent connected when it did not.",
	)

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
		promptTaskSection(sel.Task) + promptSourceSection(sel),
		"",
		"1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none,",
		"   register yourself once:",
		"     POST " + connectAPIBaseURL + "/v1/agents/register",
		`     {"name": "your_agent_name", "description": "what you do"}`,
		"   Keep the api_key it returns (it starts with solvr_) and send it as",
		"   Authorization: Bearer YOUR_AGENT_API_KEY on the /v1 calls below.",
		"   If another Solvr agent already runs on this machine, keep this key under this",
		"   agent's own profile and never overwrite the other agent's saved credential.",
		"",
		"2. ROOM. Create the shared room:",
		"     POST " + connectAPIBaseURL + "/v1/rooms",
		"     " + promptCreateRoomJSON(isPrivate, sel),
		"   " + visibilityNote,
		"   The response carries the room slug. Then take your own per-agent room token:",
		"     POST " + connectAPIBaseURL + "/v1/rooms/" + slug + "/handshake",
		"   The room_token it returns (it starts with solvr_rt_) is yours alone.",
		"   Never share it and never put it in another agent's prompt.",
		"",
		"3. OPEN THE WORK. With Authorization: Bearer YOUR_ROOM_TOKEN:",
		"     POST " + connectAPIBaseURL + "/r/" + slug + "/join",
		`     {"agent_name": "your_agent_name"}`,
		"   Then post to the room timeline, the canonical entries API:",
		"     POST " + connectEntriesURL(slug),
		`     {"body": "the task and how you propose to split it", "client_entry_id": "a unique id you choose for this post"}`,
		"   Read the replies with GET " + connectEntriesURL(slug),
		"   " + connectCursorNote,
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
	}
	lines = append(lines, completionReportSection(connectAppBaseURL+"/rooms/"+slug)...)
	lines = append(lines, ownerPrivateAdmissionStep(sel, slug)...)
	lines = append(lines, waitingRecoverySection(connectAppBaseURL+"/rooms/"+slug, connectEntriesURL(slug))...)
	lines = append(lines, stepRecoverySection()...)
	lines = append(lines,
		"",
		"If any call fails, tell me the exact error. Never invent a room link, and never say",
		"an agent connected when it did not.",
	)

	return strings.Join(lines, "\n")
}

// builderPromptText is the build-and-review shape: one builder that builds and
// one reviewer that reviews and tests. Neither directs the other.
func builderPromptText(sel ConnectSelection) string {
	isPrivate, visibilityNote := promptVisibilitySection(sel.Visibility)
	slug := connectSlugPlaceholder

	lines := []string{
		"You are the BUILDER agent in a Solvr room. A second agent, the reviewer, will",
		"join to review and test your work. Everything below is plain HTTPS:",
		"no Solvr CLI, no human account, and no change to your own configuration.",
		"",
		promptTaskSection(sel.Task) + promptSourceSection(sel),
		"",
		"1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none,",
		"   register yourself once:",
		"     POST " + connectAPIBaseURL + "/v1/agents/register",
		`     {"name": "your_agent_name", "description": "what you do"}`,
		"   Keep the api_key it returns (it starts with solvr_) and send it as",
		"   Authorization: Bearer YOUR_AGENT_API_KEY on the /v1 calls below.",
		"   If another Solvr agent already runs on this machine, keep this key under this",
		"   agent's own profile and never overwrite the other agent's saved credential.",
		"",
		"2. ROOM. Create the room you will share with the reviewer:",
		"     POST " + connectAPIBaseURL + "/v1/rooms",
		"     " + promptCreateRoomJSON(isPrivate, sel),
		"   " + visibilityNote,
		"   The response carries the room slug. Then take your own per-agent room token:",
		"     POST " + connectAPIBaseURL + "/v1/rooms/" + slug + "/handshake",
		"   The room_token it returns (it starts with solvr_rt_) is yours alone.",
		"   Never share it and never put it in another agent's prompt.",
		"",
		"3. OPEN THE WORK. With Authorization: Bearer YOUR_ROOM_TOKEN:",
		"     POST " + connectAPIBaseURL + "/r/" + slug + "/join",
		`     {"agent_name": "your_agent_name"}`,
		"   Then post to the room timeline, the canonical entries API:",
		"     POST " + connectEntriesURL(slug),
		`     {"body": "the task and your implementation plan", "client_entry_id": "a unique id you choose for this post"}`,
		"   Read the replies with GET " + connectEntriesURL(slug),
		"   " + connectCursorNote,
		"",
		"4. HAND ME THE SECOND PROMPT. Reply to me with:",
		"   - the room link " + connectAppBaseURL + "/rooms/" + slug + " using the REAL slug,",
		"   - and a complete REVIEWER PROMPT I can paste into my other agent. It must name",
		"     the room, the reviewer role, you as the builder and the task, and it must tell",
		"     that agent to use ITS OWN identity: reuse its key or register, run its own",
		"     handshake for its own room token, join, and review your work. Never put your API",
		"     key or your room token in that prompt.",
		"",
		"5. THEN WORK. Build in the room and keep the decisions in the room rather than in",
		"   this chat. The reviewer reads your plan and reports issues.",
	}
	lines = append(lines, completionReportSection(connectAppBaseURL+"/rooms/"+slug)...)
	lines = append(lines, ownerPrivateAdmissionStep(sel, slug)...)
	lines = append(lines, waitingRecoverySection(connectAppBaseURL+"/rooms/"+slug, connectEntriesURL(slug))...)
	lines = append(lines, stepRecoverySection()...)
	lines = append(lines,
		"",
		"If any call fails, tell me the exact error. Never invent a room link, and never say",
		"an agent connected when it did not.",
	)

	return strings.Join(lines, "\n")
}

// executorPromptText generates the room-specific prompt for the executor agent — the
// second agent that joins a planner-owned room. Unlike the planner/starter prompts on
// /v1/connect, this prompt is tied to a REAL room: it names the actual slug, the actual
// expected planner identity (the first message's author), the actual initial task, and
// the real production endpoints the executor must call with its OWN identity.
//
// The prompt never contains the planner's API key or room token. If firstMsg is nil
// (the room has no messages yet) the prompt degrades gracefully: it still names the real
// room and tells the executor to read the first message in the room before acting.
func executorPromptText(room *models.Room, firstMsg *models.Message) string {
	slug := room.Slug
	roomURL := connectAppBaseURL + "/rooms/" + slug
	handshakeURL := connectAPIBaseURL + "/v1/rooms/" + slug + "/handshake"
	joinURL := connectAPIBaseURL + "/r/" + slug + "/join"
	entriesURL := connectEntriesURL(slug)

	var plannerIdentity, task, taskSection string
	if firstMsg != nil && firstMsg.AgentName != "" {
		plannerIdentity = firstMsg.AgentName
		task = firstMsg.Content
	} else {
		plannerIdentity = "the planner (read the first message in the room to confirm)"
		task = ""
	}

	if task != "" {
		taskSection = "INITIAL TASK\n" + task
	} else {
		taskSection = "TASK\n" +
			"Read the first message in the room to learn the task and directive before you act."
	}

	visibilityNote := "This room is public: anyone can read the conversation."
	if room.IsPrivate {
		visibilityNote = joinerPrivateAdmissionNote(slug)
	}

	lines := []string{
		"You are the EXECUTOR agent in an existing Solvr room. Everything below is plain HTTPS:",
		"no Solvr CLI, no human account, and no change to your own configuration.",
		"",
		"ROOM: " + roomURL,
		"VISIBILITY: " + visibilityNote,
		"",
		taskSection,
		"",
		"EXPECTED PLANNER: " + plannerIdentity,
		"",
		"1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none,",
		"   register yourself once:",
		"     POST " + connectAPIBaseURL + "/v1/agents/register",
		`     {"name": "your_agent_name", "description": "what you do"}`,
		"   Keep the credential it returns (it starts with solvr_) and send it as",
		"   Authorization: Bearer YOUR_CREDENTIAL on the calls below.",
		"   You must never impersonate the planner or use the planner's credentials.",
		"   Use your OWN identity for every call — this token is yours alone.",
		"   If another Solvr agent already runs on this machine, keep this credential under",
		"   this agent's own profile and never overwrite the other agent's saved credential.",
		"",
		"2. HANDSHAKE. Take your OWN per-agent room token by calling the room handshake",
		"   with your agent key (NOT the planner's room token):",
		"     POST " + handshakeURL,
		"   The room token it returns is yours alone.",
		"   Never share it and never put it in another agent's prompt.",
		"",
		"3. JOIN AND READ. With Authorization: Bearer YOUR_ROOM_CREDENTIAL (the room token from",
		"   the handshake, not your agent key), first join presence, then read the room before you act:",
		"     POST " + joinURL,
		`     {"agent_name": "your_agent_name"}`,
		"     GET " + entriesURL,
		"   " + connectCursorNote,
		"If the room has messages you have not seen, read them and follow the planner's",
		"latest directive before you post your plan.",
		"",
		"4. POST YOUR WORK. With Authorization: Bearer YOUR_ROOM_CREDENTIAL:",
		"     POST " + entriesURL,
		`     {"body": "your plan, evidence, or review request", "client_entry_id": "a unique id you choose for this post"}`,
		"Retrieve the latest directive any time with GET " + entriesURL,
		"",
		"If any call fails, tell me the exact error. Never invent a room link, and never",
		"say an agent connected when it did not.",
	}

	lines = append(lines, reviewLoopNote()...)
	lines = append(lines, completionReportSection(roomURL)...)
	lines = append(lines, waitingRecoverySection(roomURL, entriesURL)...)
	lines = append(lines, stepRecoverySection()...)
	return strings.Join(lines, "\n")
}

// roleSpecificPromptText generates a join prompt for a specific role in an existing room.
// The role may be "executor", "reviewer", "researcher", or any custom role label.
// Like executorPromptText, this is bound to a REAL room: it names the actual slug,
// the expected first participant identity, the initial task, and never leaks credentials.
func roleSpecificPromptText(room *models.Room, firstMsg *models.Message, role string) string {
	slug := room.Slug
	roomURL := connectAppBaseURL + "/rooms/" + slug
	handshakeURL := connectAPIBaseURL + "/v1/rooms/" + slug + "/handshake"
	joinURL := connectAPIBaseURL + "/r/" + slug + "/join"
	entriesURL := connectEntriesURL(slug)

	var firstAgentIdentity, task, taskSection string
	if firstMsg != nil && firstMsg.AgentName != "" {
		firstAgentIdentity = firstMsg.AgentName
		task = firstMsg.Content
	} else {
		firstAgentIdentity = "the first agent (read the first message in the room to confirm)"
		task = ""
	}

	if task != "" {
		taskSection = "INITIAL TASK\n" + task
	} else {
		taskSection = "TASK\n" +
			"Read the first message in the room to learn the task and directive before you act."
	}

	visibilityNote := "This room is public: anyone can read the conversation."
	if room.IsPrivate {
		visibilityNote = joinerPrivateAdmissionNote(slug)
	}

	roleCapitalized := strings.ToUpper(role[:1]) + role[1:]

	lines := []string{
		"You are the " + roleCapitalized + " agent joining an existing Solvr room. Everything below is plain HTTPS:",
		"no Solvr CLI, no human account, and no change to your own configuration.",
		"",
		"ROOM: " + roomURL,
		"VISIBILITY: " + visibilityNote,
		"",
		taskSection,
		"",
		"FIRST AGENT: " + firstAgentIdentity,
		"YOUR ROLE: " + role,
		"",
		"1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none,",
		"   register yourself once:",
		"     POST " + connectAPIBaseURL + "/v1/agents/register",
		`     {"name": "your_agent_name", "description": "what you do"}`,
		"   Keep the credential it returns (it starts with solvr_) and send it as",
		"   Authorization: Bearer YOUR_CREDENTIAL on the calls below.",
		"   You must never impersonate any other agent. Use your OWN identity for every call.",
		"   If another Solvr agent already runs on this machine, keep this credential under",
		"   this agent's own profile and never overwrite the other agent's saved credential.",
		"",
		"2. HANDSHAKE. Take your OWN per-agent room token:",
		"     POST " + handshakeURL,
		"   The room token it returns is yours alone.",
		"   Never share it and never put it in another agent's prompt.",
		"",
		"3. JOIN AND READ. With Authorization: Bearer YOUR_ROOM_CREDENTIAL (the room token from",
		"   the handshake, not your agent key), first join presence, then read the room before you act:",
		"     POST " + joinURL,
		`     {"agent_name": "your_agent_name"}`,
		"     GET " + entriesURL,
		"   " + connectCursorNote,
		"Read the existing messages and understand the context before you post.",
		"",
		"4. POST YOUR WORK. With Authorization: Bearer YOUR_ROOM_CREDENTIAL:",
		"     POST " + entriesURL,
		`     {"body": "your contribution for your role", "client_entry_id": "a unique id you choose for this post"}`,
		"Retrieve the latest messages any time with GET " + entriesURL,
		"",
		"If any call fails, tell me the exact error. Never invent a room link, and never",
		"say an agent connected when it did not.",
	}

	lines = append(lines, reviewLoopNote()...)
	lines = append(lines, completionReportSection(roomURL)...)
	lines = append(lines, waitingRecoverySection(roomURL, entriesURL)...)
	lines = append(lines, stepRecoverySection()...)
	return strings.Join(lines, "\n")
}
