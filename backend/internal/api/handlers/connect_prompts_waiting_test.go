package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task "Provide bounded waiting and clear recovery instructions when another
// agent stops responding": every copied prompt must teach the receiving agent
// how to wait for a partner that has gone quiet. It must poll with bounded
// backoff instead of spamming readiness messages, give up after a default
// five-minute window and report "Waiting for participant" with the room link and
// a resume instruction, resume from the last message it saw so it never repeats
// finished work, and understand that Solvr relays messages between RUNNING agents
// and does not keep a stopped agent executing.

// waitingRecoveryPhrases are the substrings that encode the bounded-waiting and
// recovery contract. Every generated prompt must contain all of them.
var waitingRecoveryPhrases = []string{
	"bounded backoff",                       // step 2: poll, do not spam
	"Do NOT post repeated",                  // step 2: no repeated readiness messages
	"5 minutes",                             // step 3: default wait window
	"adjust",                                // step 3: adjustable window
	"Waiting for participant",               // step 3: what to report after the window
	"read the room",                         // step 4/6: resume by reading, not redoing
	"carries messages between running agents", // step 6
	"does NOT keep a stopped agent",         // step 6
}

func assertWaitingRecovery(t *testing.T, label, text string) {
	t.Helper()
	for _, phrase := range waitingRecoveryPhrases {
		if !strings.Contains(text, phrase) {
			t.Errorf("%s prompt is missing bounded-waiting phrase %q\n--- prompt ---\n%s", label, phrase, text)
		}
	}
	// The room link must accompany the "Waiting for participant" report so the
	// human can resume the stopped agent (step 3).
	if !strings.Contains(text, connectAppBaseURL+"/rooms/") {
		t.Errorf("%s prompt must include the room link with the waiting report", label)
	}
}

func TestConnectPrompts_IncludeBoundedWaitingRecovery(t *testing.T) {
	planSel := ConnectSelection{Task: "build a thing", Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPublic}
	collabSel := ConnectSelection{Task: "build a thing", Preset: ConnectPresetCollaborate, Visibility: ConnectVisibilityPublic}
	reviewSel := ConnectSelection{Task: "build a thing", Preset: ConnectPresetBuildAndReview, Visibility: ConnectVisibilityPublic}

	assertWaitingRecovery(t, "planner", plannerPromptText(planSel))
	assertWaitingRecovery(t, "starter", starterPromptText(collabSel))
	assertWaitingRecovery(t, "builder", builderPromptText(reviewSel))

	room := &models.Room{Slug: "tic-tac-toe-1", IsPrivate: false}
	firstMsg := &models.Message{AgentName: "planner-agent", Content: "Task: build tic-tac-toe."}
	assertWaitingRecovery(t, "executor", executorPromptText(room, firstMsg))
	assertWaitingRecovery(t, "role-specific", roleSpecificPromptText(room, firstMsg, "reviewer"))
}
