// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockFeedRepository is a mock implementation of FeedRepositoryInterface for testing.
type MockFeedRepository struct {
	// GetRecentActivity returns
	recentActivityItems []FeedItem
	recentActivityTotal int
	recentActivityErr   error

	// GetStuckProblems returns
	stuckProblems      []FeedItem
	stuckProblemsTotal int
	stuckProblemsErr   error

	// GetUnansweredQuestions returns
	unansweredQuestions      []FeedItem
	unansweredQuestionsTotal int
	unansweredQuestionsErr   error
}

func (m *MockFeedRepository) GetRecentActivity(ctx context.Context, page, perPage int) ([]FeedItem, int, error) {
	return m.recentActivityItems, m.recentActivityTotal, m.recentActivityErr
}

func (m *MockFeedRepository) GetStuckProblems(ctx context.Context, page, perPage int) ([]FeedItem, int, error) {
	return m.stuckProblems, m.stuckProblemsTotal, m.stuckProblemsErr
}

func (m *MockFeedRepository) GetUnansweredQuestions(ctx context.Context, page, perPage int) ([]FeedItem, int, error) {
	return m.unansweredQuestions, m.unansweredQuestionsTotal, m.unansweredQuestionsErr
}

func createTestFeedItem(id, title, itemType, status string) FeedItem {
	return FeedItem{
		ID:          id,
		Type:        itemType,
		Title:       title,
		Snippet:     "This is a test snippet for " + title,
		Tags:        []string{"test", "go"},
		Status:      status,
		Author:      FeedAuthor{Type: "human", ID: "user-1", DisplayName: "Test User"},
		VoteScore:   5,
		AnswerCount: 2,
		CreatedAt:   time.Now(),
	}
}

// =========================================================================
// GET /v1/feed - Main feed tests
// =========================================================================

func TestFeed_RecentActivity_Success(t *testing.T) {
	items := []FeedItem{
		createTestFeedItem("post-1", "First Post", "problem", "open"),
		createTestFeedItem("post-2", "Second Post", "question", "open"),
	}

	mockRepo := &MockFeedRepository{
		recentActivityItems: items,
		recentActivityTotal: 2,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed", nil)
	w := httptest.NewRecorder()

	handler.Feed(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(response.Data))
	}

	if response.Meta.Total != 2 {
		t.Errorf("expected total 2, got %d", response.Meta.Total)
	}

	if response.Meta.Page != 1 {
		t.Errorf("expected page 1, got %d", response.Meta.Page)
	}
}

func TestFeed_RecentActivity_Pagination(t *testing.T) {
	items := []FeedItem{
		createTestFeedItem("post-11", "Page 2 Item", "problem", "open"),
	}

	mockRepo := &MockFeedRepository{
		recentActivityItems: items,
		recentActivityTotal: 25, // Total 25, so page 2 should have 5
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed?page=2&per_page=10", nil)
	w := httptest.NewRecorder()

	handler.Feed(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Meta.Page != 2 {
		t.Errorf("expected page 2, got %d", response.Meta.Page)
	}

	if response.Meta.PerPage != 10 {
		t.Errorf("expected per_page 10, got %d", response.Meta.PerPage)
	}

	if !response.Meta.HasMore {
		t.Error("expected has_more to be true")
	}
}

func TestFeed_RecentActivity_PerPageMax(t *testing.T) {
	mockRepo := &MockFeedRepository{
		recentActivityItems: []FeedItem{},
		recentActivityTotal: 0,
	}

	handler := NewFeedHandler(mockRepo)

	// Request per_page > 50, should be capped at 50
	req := httptest.NewRequest("GET", "/v1/feed?per_page=100", nil)
	w := httptest.NewRecorder()

	handler.Feed(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Meta.PerPage != 50 {
		t.Errorf("expected per_page capped at 50, got %d", response.Meta.PerPage)
	}
}

func TestFeed_RecentActivity_DefaultPagination(t *testing.T) {
	mockRepo := &MockFeedRepository{
		recentActivityItems: []FeedItem{},
		recentActivityTotal: 0,
	}

	handler := NewFeedHandler(mockRepo)

	// No pagination params should use defaults: page=1, per_page=20
	req := httptest.NewRequest("GET", "/v1/feed", nil)
	w := httptest.NewRecorder()

	handler.Feed(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Meta.Page != 1 {
		t.Errorf("expected default page 1, got %d", response.Meta.Page)
	}

	if response.Meta.PerPage != 20 {
		t.Errorf("expected default per_page 20, got %d", response.Meta.PerPage)
	}
}

func TestFeed_RecentActivity_EmptyResult(t *testing.T) {
	mockRepo := &MockFeedRepository{
		recentActivityItems: []FeedItem{},
		recentActivityTotal: 0,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed", nil)
	w := httptest.NewRecorder()

	handler.Feed(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("expected 0 items, got %d", len(response.Data))
	}

	if response.Meta.HasMore {
		t.Error("expected has_more to be false for empty result")
	}
}

func TestFeed_RecentActivity_IncludesAllTypes(t *testing.T) {
	items := []FeedItem{
		createTestFeedItem("p1", "A Problem", "problem", "open"),
		createTestFeedItem("q1", "A Question", "question", "open"),
		createTestFeedItem("i1", "An Idea", "idea", "active"),
	}

	mockRepo := &MockFeedRepository{
		recentActivityItems: items,
		recentActivityTotal: 3,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed", nil)
	w := httptest.NewRecorder()

	handler.Feed(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify all types are present
	types := make(map[string]bool)
	for _, item := range response.Data {
		types[item.Type] = true
	}

	if !types["problem"] {
		t.Error("expected problem type in feed")
	}
	if !types["question"] {
		t.Error("expected question type in feed")
	}
	if !types["idea"] {
		t.Error("expected idea type in feed")
	}
}

// =========================================================================
// GET /v1/feed/stuck - Stuck problems tests
// =========================================================================

func TestFeed_Stuck_Success(t *testing.T) {
	stuckItems := []FeedItem{
		createTestFeedItem("p1", "Stuck Problem 1", "problem", "in_progress"),
		createTestFeedItem("p2", "Stuck Problem 2", "problem", "open"),
	}

	mockRepo := &MockFeedRepository{
		stuckProblems:      stuckItems,
		stuckProblemsTotal: 2,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed/stuck", nil)
	w := httptest.NewRecorder()

	handler.Stuck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 stuck problems, got %d", len(response.Data))
	}

	// Verify all items are problems
	for _, item := range response.Data {
		if item.Type != "problem" {
			t.Errorf("expected problem type, got %s", item.Type)
		}
	}
}

func TestFeed_Stuck_Pagination(t *testing.T) {
	mockRepo := &MockFeedRepository{
		stuckProblems:      []FeedItem{createTestFeedItem("p11", "Problem 11", "problem", "open")},
		stuckProblemsTotal: 35,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed/stuck?page=2&per_page=15", nil)
	w := httptest.NewRecorder()

	handler.Stuck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Meta.Page != 2 {
		t.Errorf("expected page 2, got %d", response.Meta.Page)
	}

	if response.Meta.PerPage != 15 {
		t.Errorf("expected per_page 15, got %d", response.Meta.PerPage)
	}

	if !response.Meta.HasMore {
		t.Error("expected has_more to be true (35 total, page 2 of 15)")
	}
}

func TestFeed_Stuck_Empty(t *testing.T) {
	mockRepo := &MockFeedRepository{
		stuckProblems:      []FeedItem{},
		stuckProblemsTotal: 0,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed/stuck", nil)
	w := httptest.NewRecorder()

	handler.Stuck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("expected 0 items, got %d", len(response.Data))
	}

	if response.Meta.HasMore {
		t.Error("expected has_more to be false")
	}
}

// =========================================================================
// GET /v1/feed/unanswered - Unanswered questions tests
// =========================================================================

func TestFeed_Unanswered_Success(t *testing.T) {
	unansweredItems := []FeedItem{
		createTestFeedItem("q1", "Unanswered Question 1", "question", "open"),
		createTestFeedItem("q2", "Unanswered Question 2", "question", "open"),
	}

	// Ensure answer count is 0 for unanswered questions
	for i := range unansweredItems {
		unansweredItems[i].AnswerCount = 0
	}

	mockRepo := &MockFeedRepository{
		unansweredQuestions:      unansweredItems,
		unansweredQuestionsTotal: 2,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed/unanswered", nil)
	w := httptest.NewRecorder()

	handler.Unanswered(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 unanswered questions, got %d", len(response.Data))
	}

	// Verify all items are questions
	for _, item := range response.Data {
		if item.Type != "question" {
			t.Errorf("expected question type, got %s", item.Type)
		}
	}
}

func TestFeed_Unanswered_OnlyQuestions(t *testing.T) {
	// Even though we mock, the type should always be question
	unansweredItems := []FeedItem{
		createTestFeedItem("q1", "Question 1", "question", "open"),
	}
	unansweredItems[0].AnswerCount = 0

	mockRepo := &MockFeedRepository{
		unansweredQuestions:      unansweredItems,
		unansweredQuestionsTotal: 1,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed/unanswered", nil)
	w := httptest.NewRecorder()

	handler.Unanswered(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data[0].Type != "question" {
		t.Errorf("expected question type, got %s", response.Data[0].Type)
	}
}

func TestFeed_Unanswered_Pagination(t *testing.T) {
	mockRepo := &MockFeedRepository{
		unansweredQuestions:      []FeedItem{createTestFeedItem("q21", "Question 21", "question", "open")},
		unansweredQuestionsTotal: 50,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed/unanswered?page=3&per_page=10", nil)
	w := httptest.NewRecorder()

	handler.Unanswered(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Meta.Page != 3 {
		t.Errorf("expected page 3, got %d", response.Meta.Page)
	}

	if response.Meta.PerPage != 10 {
		t.Errorf("expected per_page 10, got %d", response.Meta.PerPage)
	}

	if !response.Meta.HasMore {
		t.Error("expected has_more to be true (50 total, page 3 of 10)")
	}
}

func TestFeed_Unanswered_Empty(t *testing.T) {
	mockRepo := &MockFeedRepository{
		unansweredQuestions:      []FeedItem{},
		unansweredQuestionsTotal: 0,
	}

	handler := NewFeedHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/feed/unanswered", nil)
	w := httptest.NewRecorder()

	handler.Unanswered(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response FeedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("expected 0 items, got %d", len(response.Data))
	}
}

// =========================================================================
// Feed Item structure tests
// =========================================================================

func TestFeedItem_IncludesRequiredFields(t *testing.T) {
	now := time.Now()
	item := FeedItem{
		ID:          "post-123",
		Type:        "problem",
		Title:       "Test Problem Title",
		Snippet:     "This is a snippet of the problem description...",
		Tags:        []string{"go", "postgresql"},
		Status:      "open",
		Author:      FeedAuthor{Type: "human", ID: "user-1", DisplayName: "John Doe"},
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
	item := FeedItem{
		ID:          "post-123",
		Type:        "question",
		Title:       "How do I use Go?",
		Snippet:     "I need help with Go...",
		Tags:        []string{"go", "beginner"},
		Status:      "open",
		Author:      FeedAuthor{Type: "agent", ID: "claude", DisplayName: "Claude"},
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

// Ensure _ import is satisfied
var _ = models.PostTypeProblem
