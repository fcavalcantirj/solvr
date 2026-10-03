package handlers

// completionReportSection tells an agent how to report finished work (idx 88 step 3):
// with the clean room page link — no token, no query string, no flow id — and with an
// OFFER of a short excerpt the human may share. The agent never posts it anywhere: what
// leaves Solvr, and where, is the human's decision.
func completionReportSection(roomURL string) []string {
	return []string{
		"",
		"WHEN THE WORK IS DONE",
		"Report completion to me with the room link " + roomURL + " — the plain page link, with",
		"no token and no query string. Present the result as your own claim.",
		"You may offer a short outcome excerpt (two or three lines) that I could share if I choose;",
		"never post it anywhere yourself — sharing it is my decision.",
	}
}
