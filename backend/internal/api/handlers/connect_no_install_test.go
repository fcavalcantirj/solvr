package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The site promises "no installation" (SPEC.md 25.6): every sentence the API hands out asks
// the second agent to learn Solvr from the same link the first one read, never to install a
// skill. This walks every sentence of GET /v1/connect (each use case, public and private), of
// GET /v1/connect/examples and of every role a room's GET /v1/rooms/{slug}/connect offers.
func TestConnect_NoSentenceAsksForAnInstall(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	var sentences []string
	for _, visibility := range []string{"public", "private"} {
		start, _ := getConnect(t, h, "flow=none&visibility="+visibility+"&intent=ship+the+signup+page")
		sentences = append(sentences, start.Prompt.Text)
		for _, p := range start.Presets {
			sentences = append(sentences, p.Prompt.Text)
		}
	}
	w := httptest.NewRecorder()
	h.GetConnectExamples(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/connect/examples", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var wrapper struct {
		Data ConnectExamples `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	for _, p := range wrapper.Data.Presets {
		sentences = append(sentences, p.Prompt.Text)
	}
	for role, job := range roomRoleJobs {
		sentences = append(sentences, role+": "+job)
	}
	sentences = append(sentences, roomRoleCustomJob)

	require.Len(t, sentences, 2*4+3+len(roomRoleJobs)+1)
	for _, s := range sentences {
		require.NotContains(t, strings.ToLower(s), "install", s)
	}
	require.Equal(t, " to learn Solvr from the same link and join your room, ", connectBJoins,
		"the second agent's words, in one place (F5: PLAN-r1's wording, unconfirmed by the owner)")
}
