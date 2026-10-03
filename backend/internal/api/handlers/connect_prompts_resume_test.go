package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The RESUMING step names the directive in force (idx 92 step 1).
func TestConnectPrompts_ResumingNamesTheDirectiveInForce(t *testing.T) {
	for name, text := range allPromptTexts() {
		t.Run(name, func(t *testing.T) {
			idx := strings.Index(text, "RESUMING")
			require.GreaterOrEqual(t, idx, 0)
			require.Contains(t, text[idx:], "latest_pinned")
		})
	}
}

// A review request is a typed event subscribers are told about (idx 92 step 3).
func TestConnectPrompts_TheReviewLoopTeachesTheReviewRequestedEvent(t *testing.T) {
	for name, text := range allPromptTexts() {
		if !strings.Contains(text, "REVIEW LOOP") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			require.Contains(t, text, `"event_type": "review.requested"`)
		})
	}
}
