package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/services"
	"github.com/stretchr/testify/require"
)

// recordingModerator stands in for Groq behind handlers.ContentModerationServiceInterface, so
// router tests exercise moderation end to end (anti-abuse W2). Results are served in the order
// queued; with none queued it approves.
type recordingModerator struct {
	mu      sync.Mutex
	inputs  []handlers.ModerationInput
	results []*handlers.ModerationResult
}

func (m *recordingModerator) ModerateContent(_ context.Context, in handlers.ModerationInput) (*handlers.ModerationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputs = append(m.inputs, in)
	if len(m.results) == 0 {
		return approved(), nil
	}
	r := m.results[0]
	m.results = m.results[1:]
	return r, nil
}

// GetCalls reports how many moderation calls were made.
func (m *recordingModerator) GetCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.inputs)
}

// QueueResults sets the next verdicts, in order.
func (m *recordingModerator) QueueResults(results ...*handlers.ModerationResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, results...)
}

// LastInput returns the most recent moderation input.
func (m *recordingModerator) LastInput() handlers.ModerationInput {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.inputs) == 0 {
		return handlers.ModerationInput{}
	}
	return m.inputs[len(m.inputs)-1]
}

func approved() *handlers.ModerationResult {
	return &handlers.ModerationResult{Approved: true, LanguageDetected: "en", Confidence: 0.99}
}

func rejected(explanation string) *handlers.ModerationResult {
	return &handlers.ModerationResult{Approved: false, LanguageDetected: "en", Confidence: 0.99,
		RejectionReasons: []string{"MALICIOUS"}, Explanation: explanation}
}

// useRecordingModerator makes the next NewRouter wire the recording mock as the moderation
// service (GROQ_API_KEY set, the adapter swapped); call it before building the server.
func useRecordingModerator(t *testing.T) *recordingModerator {
	t.Helper()
	t.Setenv("GROQ_API_KEY", "test-groq-key-not-used")
	m := &recordingModerator{}
	prev := wrapContentModerator
	wrapContentModerator = func(*services.ContentModerationService) handlers.ContentModerationServiceInterface { return m }
	t.Cleanup(func() { wrapContentModerator = prev })
	return m
}

// Asynchronous moderation settles within waitTimeout in these tests.
const (
	waitTimeout = 5 * time.Second
	waitTick    = 50 * time.Millisecond
)

// waitForValue polls query until it returns want, failing after 5s (moderation is async).
func waitForValue(t *testing.T, pool *db.Pool, want, query string, args ...any) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	var got string
	for time.Now().Before(deadline) {
		if err := pool.QueryRow(context.Background(), query, args...).Scan(&got); err == nil && got == want {
			return
		}
		time.Sleep(waitTick)
	}
	require.Equal(t, want, got, "timed out waiting for: %s", query)
}
