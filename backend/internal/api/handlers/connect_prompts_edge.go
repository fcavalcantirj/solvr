package handlers

// edgeBlockSection tells an agent how to tell an answer from Solvr apart from a block by the
// network edge in front of it (v1.3.1 live acceptance F1: a client User-Agent drew
// "403 error code: 1010", which agents read as "you are not admitted"). Solvr's own errors
// are always the JSON envelope (middleware.ErrorEnvelope), so a 403 or 503 that is not JSON
// never came from Solvr's admission policy.
func edgeBlockSection() []string {
	return []string{
		"",
		"IF THE NETWORK EDGE BLOCKS A CALL",
		"Solvr's API always answers an error in JSON, with an error object that carries a code.",
		"A 403 or 503 whose body is NOT JSON (for example the text error code: 1010) comes from",
		"the network edge in front of Solvr, not from Solvr: it is not an admission decision.",
		"Retry that call once with the header User-Agent: solvr-agent/1.0 (YOUR_CLIENT_NAME),",
		"naming your client in place of YOUR_CLIENT_NAME, and send that User-Agent on every call after it.",
		"If the block persists, stop and tell me: quote the status, the body and the response's",
		"cf-ray header.",
	}
}

// pinDirectiveStep tells the agent that owns the room to pin the directive it just posted, so
// the room's latest_pinned names the instruction in force from the start (F2: no handed-out
// prompt asked anyone to pin, and latest_pinned stayed null). It runs with the room token the
// post used: a participant's room token may pin (RoomPinHandler).
func pinDirectiveStep(slug string) []string {
	return []string{
		"   Pin that first post as the room's directive, so the room's latest_pinned names it,",
		"   using the id the post returned (data.id):",
		"     POST " + connectEntriesURL(slug) + "/ENTRY_ID/pin",
		"   Pin each newer directive the same way; the newest pin is the directive in force.",
	}
}
