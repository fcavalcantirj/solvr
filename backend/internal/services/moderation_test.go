// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// MockFlagCreator implements FlagCreator interface for testing.
type MockFlagCreator struct {
	CreatedFlags []*models.Flag
	CreateError  error
}

func (m *MockFlagCreator) CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error) {
	if m.CreateError != nil {
		return nil, m.CreateError
	}
	flag.ID = uuid.New()
	flag.CreatedAt = time.Now()
	m.CreatedFlags = append(m.CreatedFlags, flag)
	return flag, nil
}

// MockRateLimitChecker implements RateLimitChecker interface for testing.
type MockRateLimitChecker struct {
	RateLimitHits map[string]int // userKey -> hit count
	Threshold     int
}

func (m *MockRateLimitChecker) GetRecentPostCount(ctx context.Context, userType, userID string, window time.Duration) (int, error) {
	key := userType + ":" + userID
	return m.RateLimitHits[key], nil
}

// --- Spam Detection Tests ---

func TestDetectSpam_NoSpamInNormalContent(t *testing.T) {
	detector := NewSpamDetector()
	content := ModerationContent{
		Title:       "How to handle async errors in Go?",
		Description: "I'm trying to implement proper error handling in my Go application that uses goroutines. What's the best practice for propagating errors from goroutines back to the main function?",
	}

	result := detector.CheckSpam(content)

	if result.IsSpam {
		t.Errorf("Expected no spam detection for normal content, got spam with reasons: %v", result.Reasons)
	}
}

func TestDetectSpam_ExcessiveLinks(t *testing.T) {
	detector := NewSpamDetector()
	content := ModerationContent{
		Title:       "Great resources",
		Description: "Check out http://spam1.com and http://spam2.com and http://spam3.com and http://spam4.com and http://spam5.com and http://spam6.com for more info",
	}

	result := detector.CheckSpam(content)

	if !result.IsSpam {
		t.Error("Expected spam detection for excessive links")
	}
	if !containsReason(result.Reasons, "excessive_links") {
		t.Errorf("Expected 'excessive_links' reason, got: %v", result.Reasons)
	}
}

func TestDetectSpam_ExactLinkThreshold(t *testing.T) {
	detector := NewSpamDetector()
	// 5 links should be OK (threshold is 5)
	content := ModerationContent{
		Title:       "Resources",
		Description: "http://link1.com http://link2.com http://link3.com http://link4.com http://link5.com",
	}

	result := detector.CheckSpam(content)

	if result.IsSpam {
		t.Error("5 links should not trigger spam detection (threshold is 5)")
	}
}

func TestDetectSpam_RepeatedText(t *testing.T) {
	detector := NewSpamDetector()
	// Repeated phrase more than 3 times
	content := ModerationContent{
		Title:       "Check this out",
		Description: "buy now buy now buy now buy now buy now",
	}

	result := detector.CheckSpam(content)

	if !result.IsSpam {
		t.Error("Expected spam detection for repeated text")
	}
	if !containsReason(result.Reasons, "repeated_text") {
		t.Errorf("Expected 'repeated_text' reason, got: %v", result.Reasons)
	}
}

func TestDetectSpam_AllCaps(t *testing.T) {
	detector := NewSpamDetector()
	// More than 50% caps in title
	content := ModerationContent{
		Title:       "BUY NOW BEST DEAL EVER",
		Description: "This is a normal description with proper capitalization.",
	}

	result := detector.CheckSpam(content)

	if !result.IsSpam {
		t.Error("Expected spam detection for all caps title")
	}
	if !containsReason(result.Reasons, "excessive_caps") {
		t.Errorf("Expected 'excessive_caps' reason, got: %v", result.Reasons)
	}
}

func TestDetectSpam_ForbiddenWords(t *testing.T) {
	detector := NewSpamDetector()
	content := ModerationContent{
		Title:       "Free bitcoin giveaway",
		Description: "Get free crypto now! This is a limited time offer.",
	}

	result := detector.CheckSpam(content)

	if !result.IsSpam {
		t.Error("Expected spam detection for forbidden words")
	}
	if !containsReason(result.Reasons, "forbidden_words") {
		t.Errorf("Expected 'forbidden_words' reason, got: %v", result.Reasons)
	}
}

func TestDetectSpam_MultipleReasons(t *testing.T) {
	detector := NewSpamDetector()
	content := ModerationContent{
		Title:       "FREE BITCOIN GET IT NOW",
		Description: "http://spam1.com http://spam2.com http://spam3.com http://spam4.com http://spam5.com http://spam6.com buy now buy now buy now buy now",
	}

	result := detector.CheckSpam(content)

	if !result.IsSpam {
		t.Error("Expected spam detection")
	}
	if len(result.Reasons) < 2 {
		t.Errorf("Expected multiple reasons, got: %v", result.Reasons)
	}
}

func TestDetectSpam_ShortContent(t *testing.T) {
	detector := NewSpamDetector()
	content := ModerationContent{
		Title:       "Hi",
		Description: "Hello",
	}

	result := detector.CheckSpam(content)

	if !result.IsSpam {
		t.Error("Expected spam detection for too short content")
	}
	if !containsReason(result.Reasons, "too_short") {
		t.Errorf("Expected 'too_short' reason, got: %v", result.Reasons)
	}
}

func TestDetectSpam_CustomConfig(t *testing.T) {
	config := SpamDetectorConfig{
		MaxLinks:       10, // More permissive
		MinTitleLength: 5,
		MinDescLength:  10,
	}
	detector := NewSpamDetectorWithConfig(config)
	content := ModerationContent{
		Title:       "Links",
		Description: "http://a.com http://b.com http://c.com http://d.com http://e.com http://f.com normal text here",
	}

	result := detector.CheckSpam(content)

	if result.IsSpam {
		t.Errorf("Custom config (maxLinks=10) should allow 6 links, got spam: %v", result.Reasons)
	}
}

// --- Duplicate Detection Tests ---

func TestModerationService_CheckDuplicate_NoDuplicate(t *testing.T) {
	dupService := NewDuplicateDetectionService(NewInMemoryDuplicateStore())
	modService := NewModerationService(nil, dupService, nil, nil)

	content := ModerationContent{
		Title:       "Unique title here",
		Description: "This is a unique description that has not been posted before.",
	}

	result, err := modService.CheckDuplicate(context.Background(), content)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.IsDuplicate {
		t.Error("Expected no duplicate for new content")
	}
}

func TestModerationService_CheckDuplicate_Found(t *testing.T) {
	dupStore := NewInMemoryDuplicateStore()
	dupService := NewDuplicateDetectionService(dupStore)
	modService := NewModerationService(nil, dupService, nil, nil)

	// Register existing content
	postID := uuid.New()
	title := "How to test Go code"
	desc := "I want to learn about testing in Go. What's the best approach?"
	_ = dupService.RegisterContent(context.Background(), postID, title, desc, time.Now())

	content := ModerationContent{
		Title:       title,
		Description: desc,
	}

	result, err := modService.CheckDuplicate(context.Background(), content)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.IsDuplicate {
		t.Error("Expected duplicate detection")
	}
	if result.OriginalPostID != postID {
		t.Errorf("Expected original post ID %s, got %s", postID, result.OriginalPostID)
	}
}

// --- Rate Abuse Detection Tests ---

func TestModerationService_CheckRateAbuse_NoAbuse(t *testing.T) {
	mockChecker := &MockRateLimitChecker{
		RateLimitHits: map[string]int{"human:user123": 2},
		Threshold:     10,
	}
	modService := NewModerationService(nil, nil, mockChecker, nil)

	result, err := modService.CheckRateAbuse(context.Background(), "human", "user123")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.IsAbusive {
		t.Error("Expected no abuse for low post count")
	}
}

func TestModerationService_CheckRateAbuse_Detected(t *testing.T) {
	mockChecker := &MockRateLimitChecker{
		RateLimitHits: map[string]int{"human:user123": 15},
		Threshold:     10,
	}
	modService := NewModerationService(nil, nil, mockChecker, nil)

	result, err := modService.CheckRateAbuse(context.Background(), "human", "user123")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.IsAbusive {
		t.Error("Expected abuse detection for high post count")
	}
	if result.PostCount != 15 {
		t.Errorf("Expected post count 15, got %d", result.PostCount)
	}
}

func TestModerationService_CheckRateAbuse_CustomThreshold(t *testing.T) {
	mockChecker := &MockRateLimitChecker{
		RateLimitHits: map[string]int{"agent:bot1": 5},
	}
	config := RateAbuseConfig{
		Threshold: 3,  // Custom lower threshold
		Window:    time.Hour,
	}
	modService := NewModerationServiceWithRateConfig(nil, nil, mockChecker, nil, config)

	result, err := modService.CheckRateAbuse(context.Background(), "agent", "bot1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.IsAbusive {
		t.Error("Expected abuse detection with custom threshold")
	}
}

// --- Link Spam Detection Tests ---

func TestDetectLinkSpam_NoLinks(t *testing.T) {
	detector := NewSpamDetector()
	content := ModerationContent{
		Title:       "Simple question",
		Description: "This is a question without any links in it.",
	}

	result := detector.CheckLinkSpam(content)

	if result.HasExcessiveLinks {
		t.Error("Expected no link spam for content without links")
	}
	if result.LinkCount != 0 {
		t.Errorf("Expected 0 links, got %d", result.LinkCount)
	}
}

func TestDetectLinkSpam_NormalLinks(t *testing.T) {
	detector := NewSpamDetector()
	content := ModerationContent{
		Title:       "Reference links",
		Description: "Check https://golang.org and https://go.dev for documentation.",
	}

	result := detector.CheckLinkSpam(content)

	if result.HasExcessiveLinks {
		t.Error("Expected no link spam for normal link count")
	}
	if result.LinkCount != 2 {
		t.Errorf("Expected 2 links, got %d", result.LinkCount)
	}
}

func TestDetectLinkSpam_Excessive(t *testing.T) {
	detector := NewSpamDetector()
	content := ModerationContent{
		Title:       "Links",
		Description: "http://a.com http://b.com http://c.com https://d.com https://e.com https://f.com",
	}

	result := detector.CheckLinkSpam(content)

	if !result.HasExcessiveLinks {
		t.Error("Expected link spam detection for 6 links")
	}
	if result.LinkCount != 6 {
		t.Errorf("Expected 6 links, got %d", result.LinkCount)
	}
}

// --- Auto-Flag Creation Tests ---

func TestModerationService_AutoFlag_Spam(t *testing.T) {
	mockFlagCreator := &MockFlagCreator{}
	modService := NewModerationService(mockFlagCreator, nil, nil, nil)

	ctx := context.Background()
	postID := uuid.New()
	content := ModerationContent{
		Title:       "BUY FREE BITCOIN NOW",
		Description: "http://a.com http://b.com http://c.com http://d.com http://e.com http://f.com get rich quick",
	}

	err := modService.AutoFlagIfNeeded(ctx, postID, "post", content)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(mockFlagCreator.CreatedFlags) == 0 {
		t.Error("Expected flag to be created for spam content")
	}

	flag := mockFlagCreator.CreatedFlags[0]
	if flag.TargetID != postID {
		t.Errorf("Expected target ID %s, got %s", postID, flag.TargetID)
	}
	if flag.Reason != "spam" {
		t.Errorf("Expected reason 'spam', got '%s'", flag.Reason)
	}
	if flag.ReporterType != "system" {
		t.Errorf("Expected reporter type 'system', got '%s'", flag.ReporterType)
	}
}

func TestModerationService_AutoFlag_NoFlagForCleanContent(t *testing.T) {
	mockFlagCreator := &MockFlagCreator{}
	modService := NewModerationService(mockFlagCreator, nil, nil, nil)

	ctx := context.Background()
	postID := uuid.New()
	content := ModerationContent{
		Title:       "How to handle errors in Go?",
		Description: "I'm building a web application and want to implement proper error handling. What are the best practices for error handling in Go?",
	}

	err := modService.AutoFlagIfNeeded(ctx, postID, "post", content)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(mockFlagCreator.CreatedFlags) != 0 {
		t.Errorf("Expected no flag for clean content, got %d flags", len(mockFlagCreator.CreatedFlags))
	}
}

func TestModerationService_AutoFlag_LinkSpam(t *testing.T) {
	mockFlagCreator := &MockFlagCreator{}
	modService := NewModerationService(mockFlagCreator, nil, nil, nil)

	ctx := context.Background()
	postID := uuid.New()
	content := ModerationContent{
		Title:       "Check these resources out",
		Description: "http://spam1.com http://spam2.com http://spam3.com http://spam4.com http://spam5.com http://spam6.com http://spam7.com http://spam8.com",
	}

	err := modService.AutoFlagIfNeeded(ctx, postID, "post", content)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(mockFlagCreator.CreatedFlags) == 0 {
		t.Error("Expected flag to be created for link spam")
	}
}

func TestModerationService_AutoFlag_Duplicate(t *testing.T) {
	mockFlagCreator := &MockFlagCreator{}
	dupStore := NewInMemoryDuplicateStore()
	dupService := NewDuplicateDetectionService(dupStore)
	modService := NewModerationService(mockFlagCreator, dupService, nil, nil)

	ctx := context.Background()

	// Register existing content
	existingID := uuid.New()
	title := "How to test Go code"
	desc := "I want to learn about testing in Go applications. What is the best approach?"
	_ = dupService.RegisterContent(ctx, existingID, title, desc, time.Now())

	// Try to post duplicate
	newPostID := uuid.New()
	content := ModerationContent{
		Title:       title,
		Description: desc,
	}

	err := modService.AutoFlagIfNeeded(ctx, newPostID, "post", content)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(mockFlagCreator.CreatedFlags) == 0 {
		t.Error("Expected flag to be created for duplicate content")
	}

	flag := mockFlagCreator.CreatedFlags[0]
	if flag.Reason != "duplicate" {
		t.Errorf("Expected reason 'duplicate', got '%s'", flag.Reason)
	}
}

// --- SpamDetectorConfig Tests ---

func TestDefaultSpamDetectorConfig(t *testing.T) {
	config := DefaultSpamDetectorConfig()

	if config.MaxLinks != 5 {
		t.Errorf("Expected MaxLinks=5, got %d", config.MaxLinks)
	}
	if config.MinTitleLength != 10 {
		t.Errorf("Expected MinTitleLength=10, got %d", config.MinTitleLength)
	}
	if config.MinDescLength != 50 {
		t.Errorf("Expected MinDescLength=50, got %d", config.MinDescLength)
	}
	if config.MaxCapsPercentage != 50 {
		t.Errorf("Expected MaxCapsPercentage=50, got %d", config.MaxCapsPercentage)
	}
	if len(config.ForbiddenWords) == 0 {
		t.Error("Expected ForbiddenWords to be populated")
	}
}

func TestDefaultRateAbuseConfig(t *testing.T) {
	config := DefaultRateAbuseConfig()

	if config.Threshold != 10 {
		t.Errorf("Expected Threshold=10, got %d", config.Threshold)
	}
	if config.Window != time.Hour {
		t.Errorf("Expected Window=1h, got %v", config.Window)
	}
}

// --- Helper Functions ---

func containsReason(reasons []string, target string) bool {
	for _, r := range reasons {
		if r == target {
			return true
		}
	}
	return false
}

// --- Integration Test: Full Moderation Flow ---

func TestModerationService_FullModerationFlow(t *testing.T) {
	// Setup
	mockFlagCreator := &MockFlagCreator{}
	dupService := NewDuplicateDetectionService(NewInMemoryDuplicateStore())
	mockRateChecker := &MockRateLimitChecker{
		RateLimitHits: map[string]int{},
	}
	modService := NewModerationService(mockFlagCreator, dupService, mockRateChecker, nil)

	ctx := context.Background()

	// Test 1: Clean content - no flags
	cleanContent := ModerationContent{
		Title:       "Understanding Go interfaces",
		Description: "I'm trying to understand how interfaces work in Go. Can someone explain the empty interface and type assertions?",
	}
	err := modService.AutoFlagIfNeeded(ctx, uuid.New(), "post", cleanContent)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(mockFlagCreator.CreatedFlags) != 0 {
		t.Error("Expected no flags for clean content")
	}

	// Test 2: Spam content - flag created
	spamContent := ModerationContent{
		Title:       "FREE MONEY NOW",
		Description: "http://spam.com http://spam2.com http://spam3.com http://spam4.com http://spam5.com http://spam6.com GET RICH",
	}
	err = modService.AutoFlagIfNeeded(ctx, uuid.New(), "post", spamContent)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(mockFlagCreator.CreatedFlags) != 1 {
		t.Errorf("Expected 1 flag for spam content, got %d", len(mockFlagCreator.CreatedFlags))
	}

	// Test 3: Register clean content, then check duplicate
	postID := uuid.New()
	_ = dupService.RegisterContent(ctx, postID, cleanContent.Title, cleanContent.Description, time.Now())

	duplicateResult, err := modService.CheckDuplicate(ctx, cleanContent)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !duplicateResult.IsDuplicate {
		t.Error("Expected duplicate detection for registered content")
	}
}

func TestCountLinks(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"no links here", 0},
		{"check http://example.com", 1},
		{"check https://example.com", 1},
		{"http://a.com and https://b.com", 2},
		{"http://a.com http://b.com http://c.com", 3},
		{"text http://a.com more text https://b.com end", 2},
		{"malformed htt://notaurl.com", 0},
		{"ftp://other.com is not http", 0},
	}

	for _, tc := range tests {
		count := countLinks(tc.text)
		if count != tc.expected {
			t.Errorf("countLinks(%q) = %d, expected %d", tc.text, count, tc.expected)
		}
	}
}

func TestCountCapsPercentage(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"hello world", 0},
		{"HELLO WORLD", 100},
		{"Hello World", 20}, // 2 caps out of 10 letters
		{"HeLLo", 60},       // 3 caps out of 5 letters
		{"", 0},
		{"123", 0}, // no letters
	}

	for _, tc := range tests {
		percentage := countCapsPercentage(tc.text)
		if percentage != tc.expected {
			t.Errorf("countCapsPercentage(%q) = %d, expected %d", tc.text, percentage, tc.expected)
		}
	}
}

func TestHasRepeatedText(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"normal text here", false},
		{"buy now buy now buy now buy now", true}, // 4 repeats
		{"good good good", false},                  // only 3 repeats
		{"word word word word word", true},        // 5 repeats
		{"", false},
	}

	for _, tc := range tests {
		result := hasRepeatedText(tc.text, 4)
		if result != tc.expected {
			t.Errorf("hasRepeatedText(%q) = %v, expected %v", tc.text, result, tc.expected)
		}
	}
}

func TestContainsForbiddenWords(t *testing.T) {
	forbidden := []string{"bitcoin", "crypto", "free money"}

	tests := []struct {
		text     string
		expected bool
	}{
		{"normal text here", false},
		{"get bitcoin now", true},
		{"BITCOIN is great", true}, // case insensitive
		{"crypto currency", true},
		{"free money for all", true},
		{"freedom is good", false}, // "free" alone not in list
	}

	for _, tc := range tests {
		result := containsForbiddenWords(tc.text, forbidden)
		if result != tc.expected {
			t.Errorf("containsForbiddenWords(%q) = %v, expected %v", tc.text, result, tc.expected)
		}
	}
}

// countLinks is a helper to count HTTP/HTTPS links in text.
func countLinks(text string) int {
	count := 0
	// Simple counting of http:// and https:// occurrences
	count += strings.Count(text, "http://")
	count += strings.Count(text, "https://")
	return count
}

// countCapsPercentage returns the percentage of uppercase letters.
func countCapsPercentage(text string) int {
	if len(text) == 0 {
		return 0
	}
	caps := 0
	letters := 0
	for _, r := range text {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			letters++
			if r >= 'A' && r <= 'Z' {
				caps++
			}
		}
	}
	if letters == 0 {
		return 0
	}
	return (caps * 100) / letters
}

// hasRepeatedText checks for phrases repeated more than threshold times.
func hasRepeatedText(text string, threshold int) bool {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 2 {
		return false
	}

	// Check 2-word phrases
	phrases := make(map[string]int)
	for i := 0; i < len(words)-1; i++ {
		phrase := words[i] + " " + words[i+1]
		phrases[phrase]++
		if phrases[phrase] >= threshold {
			return true
		}
	}

	// Check single word repetition
	wordCount := make(map[string]int)
	for _, w := range words {
		wordCount[w]++
		if wordCount[w] >= threshold {
			return true
		}
	}

	return false
}

// containsForbiddenWords checks if text contains any forbidden words/phrases.
func containsForbiddenWords(text string, forbidden []string) bool {
	lower := strings.ToLower(text)
	for _, word := range forbidden {
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}
