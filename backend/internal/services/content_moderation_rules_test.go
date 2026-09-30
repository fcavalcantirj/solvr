package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Anti-abuse W2, T-M12: the moderator is told the purge rules (automated reports and templated
// series, repeats) in the static, cacheable system prompt, and sees the author's recent titles
// in the user message.
func TestModerateContent_AntiAbuseRulesAndAuthorTitles(t *testing.T) {
	var captured groqChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(groqModerationResponse(false, "english", []string{"QUALITY"}, 0.97, "templated series"))) //nolint:errcheck
	}))
	defer server.Close()

	svc := NewContentModerationService("test-api-key", WithGroqBaseURL(server.URL))
	_, err := svc.ModerateContent(context.Background(), ModerationInput{
		Title:       "Quantum Monitoring 52-Day Persistence Verification",
		Description: "Day 52 of continuous operation verification for the eternal monitoring architecture.",
		AuthorRecentTitles: []string{
			"Quantum Monitoring 51-Day Persistence Verification",
			"Quantum Monitoring 50-Day Persistence Verification",
		},
	})
	if err != nil {
		t.Fatalf("ModerateContent: %v", err)
	}
	if len(captured.Messages) < 2 {
		t.Fatalf("messages = %d, want system + user", len(captured.Messages))
	}
	system, user := captured.Messages[0].Content, captured.Messages[1].Content
	if system != contentModerationSystemPrompt {
		t.Fatal("the system prompt must stay the static constant (prompt caching)")
	}
	for _, rule := range []string{"6. AUTOMATED REPORTS", "heartbeat or watchdog", "templated day or milestone series", "7. REPEATS"} {
		if !strings.Contains(system, rule) {
			t.Errorf("system prompt lacks %q", rule)
		}
	}
	want := "Author's recent titles:\n- Quantum Monitoring 51-Day Persistence Verification\n- Quantum Monitoring 50-Day Persistence Verification"
	if !strings.Contains(user, want) {
		t.Errorf("user message lacks the author's recent titles:\n%s", user)
	}

	// No history, no titles block.
	_, _ = svc.ModerateContent(context.Background(), ModerationInput{Title: "Fresh author post", Description: "Body"})
	if strings.Contains(captured.Messages[1].Content, "Author's recent titles") {
		t.Errorf("titles block without titles:\n%s", captured.Messages[1].Content)
	}
}
