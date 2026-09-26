package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// idx 75 step 1: every connection prompt tells the agent what a handshake does to its other
// sessions and how to recover when a session's token was replaced, so following the prompt
// twice cannot silently break the first session and a rotated session knows what to do.
func TestConnectPrompts_ExplainSessionsAndRotation(t *testing.T) {
	for _, p := range allConnectionPrompts() {
		t.Run(p.name, func(t *testing.T) {
			lower := strings.ToLower(p.text)
			require.Contains(t, p.text, "CREDENTIAL_ROTATED", "the recoverable code is named")
			require.Contains(t, lower, "never invalidates your other sessions", "a plain handshake is safe to repeat")
			require.Contains(t, p.text, "rotate true", "rotation is the explicit, named exception")
			require.NotContains(t, p.text, "$")
			require.NotContains(t, p.text, "`")
		})
	}
}
