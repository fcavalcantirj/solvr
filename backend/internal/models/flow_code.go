package models

import "regexp"

// The flow code (SPEC.md Part 25.6).
//
// A flow is one visit to a surface that starts a connection. GET /v1/connect mints its id
// as a short public code, and that code rides on the skill link of the sentence the
// visitor copies (https://solvr.dev/skill.md?f=<code>), so the agent that creates the room
// can hand it back and the visit is tied to the room it produced.
//
// The code is not a secret and proves nothing: it only joins funnel steps. It is short
// and has no look-alike characters (no i, l, o, 0, 1) because it is shown inside a
// sentence people read and copy.
const (
	// FlowCodeAlphabet is every character a flow code may hold.
	FlowCodeAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

	// FlowCodeLength is the length of every flow code.
	FlowCodeLength = 8
)

// flowCodePattern is FlowCodeAlphabet and FlowCodeLength as one expression. In Go's
// syntax "$" is the end of the text, so a trailing newline is not accepted.
var flowCodePattern = regexp.MustCompile(`^[a-hjkmnp-z2-9]{8}$`)

// ValidFlowCode reports whether s is a well-formed flow code. It is the ONE format check:
// GET /v1/connect (the flow parameter), POST /v1/analytics/funnel (skill_fetched) and
// POST /v1/rooms (flow_id) all use it, so a value that is not exactly a code is never
// echoed into a sentence and never stored as one.
func ValidFlowCode(s string) bool {
	return flowCodePattern.MatchString(s)
}
