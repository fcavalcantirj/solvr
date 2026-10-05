package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// The flow code of a new room (SPEC.md Part 25.7).
//
// The sentence a visitor copies carries the visit's flow code on its skill link, and the
// skill tells the agent to send it as flow_id when it creates the room. That is how a
// website visit is tied to the room it produced. The value is analytics only, so the
// rule has two halves that never bend:
//
//   - A room is NEVER refused or delayed because of flow_id. Any JSON decodes, the lookup
//     is one bounded query, and whatever cannot be kept is dropped without a word: the
//     room is created exactly as without it.
//   - room_created keeps the code only when it is well formed AND known, that is, an
//     earlier funnel step other than room_created already carries it. A code nobody
//     issued, or one typed by hand, attributes nothing.

// flowLookupTimeout bounds the one lookup that decides whether a flow code is known. A
// lookup that takes longer is treated like one that failed: the code is dropped.
const flowLookupTimeout = 500 * time.Millisecond

// roomFlowID is the flow_id of a create-room body. Whatever JSON a caller put there is
// accepted, because a room is never refused because of it: a string is taken as it is,
// and anything else (a bare number that happens to spell a code, null, an object) is kept
// as its raw text, which is then either a flow code or dropped like any other value.
type roomFlowID string

// UnmarshalJSON never fails.
func (f *roomFlowID) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		*f = roomFlowID(s)
		return nil
	}
	*f = roomFlowID(bytes.TrimSpace(raw))
	return nil
}

// attributableFlowCode decides which flow code a new room's room_created step carries:
// sent itself when it is a well-formed flow code that known reports an earlier step
// carries, otherwise "". A malformed value is never looked up. When the lookup fails the
// code is dropped and the error is returned so the caller can log it.
func attributableFlowCode(sent string, known func(code string) (bool, error)) (string, error) {
	if !models.ValidFlowCode(sent) {
		return "", nil
	}
	ok, err := known(sent)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return sent, nil
}

// roomCreatedFlow is attributableFlowCode against the funnel store, bounded in time.
func (h *RoomHandler) roomCreatedFlow(ctx context.Context, sent roomFlowID) string {
	flow, err := attributableFlowCode(string(sent), func(code string) (bool, error) {
		lookupCtx, cancel := context.WithTimeout(ctx, flowLookupTimeout)
		defer cancel()
		return h.funnel.FlowKnown(lookupCtx, code)
	})
	if err != nil {
		slog.Warn("flow code lookup failed; room_created is recorded without a flow id", "error", err)
	}
	return flow
}
