// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// The retired FeedHandler/FeedRepository query stack (idx 72 step 3) was replaced by the
// LegacyFeedAdapter over the canonical GET /v1/posts list; its live behavior is guarded by
// internal/api/router_legacy_feed_test.go. models.FeedItem stays alive (the adapter renders
// canonical rows in that shape), so its shape/serialization guards are kept here.

func TestFeedItem_IncludesRequiredFields(t *testing.T) {
	now := time.Now()
	item := models.FeedItem{
		ID:          "post-123",
		Type:        "problem",
		Title:       "Test Problem Title",
		Snippet:     "This is a snippet of the problem description...",
		Tags:        []string{"go", "postgresql"},
		Status:      "open",
		Author:      models.FeedAuthor{Type: "human", ID: "user-1", DisplayName: "John Doe"},
		VoteScore:   10,
		AnswerCount: 3,
		CreatedAt:   now,
	}

	if item.ID != "post-123" {
		t.Errorf("expected ID post-123, got %s", item.ID)
	}
	if item.Type != "problem" {
		t.Errorf("expected Type problem, got %s", item.Type)
	}
	if item.Title != "Test Problem Title" {
		t.Errorf("expected Title 'Test Problem Title', got %s", item.Title)
	}
	if item.VoteScore != 10 {
		t.Errorf("expected VoteScore 10, got %d", item.VoteScore)
	}
	if item.Author.DisplayName != "John Doe" {
		t.Errorf("expected author DisplayName 'John Doe', got %s", item.Author.DisplayName)
	}
}

func TestFeedItem_JSONSerialization(t *testing.T) {
	item := models.FeedItem{
		ID:          "post-123",
		Type:        "question",
		Title:       "How do I use Go?",
		Snippet:     "I need help with Go...",
		Tags:        []string{"go", "beginner"},
		Status:      "open",
		Author:      models.FeedAuthor{Type: "agent", ID: "claude", DisplayName: "Claude"},
		VoteScore:   5,
		AnswerCount: 0,
		CreatedAt:   time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
	}

	jsonBytes, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("failed to marshal FeedItem: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(jsonBytes)
	expectedFields := []string{
		`"id":"post-123"`,
		`"type":"question"`,
		`"title":"How do I use Go?"`,
		`"vote_score":5`,
		`"answer_count":0`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("expected JSON to contain %s, got %s", field, jsonStr)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
