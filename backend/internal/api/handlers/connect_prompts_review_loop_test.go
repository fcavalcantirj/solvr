package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task "Support the planner/executor review loop": step 5 requires that an agent
// does NOT interpret silence as approval and that an unverified completion claim is
// the author's own claim, not a platform-certified outcome. The join prompts for the
// executor and any additional role must encode both.
var reviewLoopPhrases = []string{
	"Do NOT interpret silence as approval", // step 5: silence is not approval
	"review request",                       // step 5: ask for explicit review
	"your own claim",                       // step 5: completion is the author's claim
	"does not certify outcomes",            // step 5: Solvr does not certify
}

func assertReviewLoop(t *testing.T, label, text string) {
	t.Helper()
	for _, phrase := range reviewLoopPhrases {
		if !strings.Contains(text, phrase) {
			t.Errorf("%s prompt is missing review-loop phrase %q\n--- prompt ---\n%s", label, phrase, text)
		}
	}
}

func TestConnectPrompts_IncludeReviewLoopGuidance(t *testing.T) {
	room := &models.Room{Slug: "tic-tac-toe-1", IsPrivate: false}
	firstMsg := &models.Message{AgentName: "planner-agent", Content: "Task: build tic-tac-toe."}

	assertReviewLoop(t, "executor", executorPromptText(room, firstMsg))
	assertReviewLoop(t, "role-specific", roleSpecificPromptText(room, firstMsg, "reviewer"))
}
