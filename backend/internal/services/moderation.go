// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// FlagCreator interface for creating flags.
type FlagCreator interface {
	CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error)
}

// RateLimitChecker interface for checking posting rate.
type RateLimitChecker interface {
	GetRecentPostCount(ctx context.Context, userType, userID string, window time.Duration) (int, error)
}

// ContentDuplicateFinder reads the canonical posts and replies tables for the earlier live
// post or reply that new content repeats (db.ContentDuplicateRepository).
type ContentDuplicateFinder interface {
	FindPost(ctx context.Context, title, description string, since time.Time, excludeID string) (*models.ContentDuplicate, error)
	FindReply(ctx context.Context, postID, body string, since time.Time, excludeID string) (*models.ContentDuplicate, error)
}

// ModerationContent represents content to be moderated.
type ModerationContent struct {
	Title       string
	Description string // for a reply target: the reply body
	Tags        []string
	AuthorType  string // human, agent
	AuthorID    string
	PostID      string // for a reply target: the post the reply belongs to
}

// SpamCheckResult contains the result of spam detection.
type SpamCheckResult struct {
	IsSpam  bool
	Reasons []string
	Score   float64 // 0.0 to 1.0 confidence
}

// DuplicateCheckResult contains the result of duplicate detection.
type DuplicateCheckResult struct {
	IsDuplicate    bool
	OriginalPostID uuid.UUID
	Similarity     float64
	// OriginalTargetType and OriginalTargetID name the repeated post or reply; set by
	// CheckContentDuplicate only.
	OriginalTargetType string
	OriginalTargetID   string
}

// RateAbuseResult contains the result of rate abuse detection.
type RateAbuseResult struct {
	IsAbusive bool
	PostCount int
	Window    time.Duration
	Threshold int
}

// LinkSpamResult contains the result of link spam detection.
type LinkSpamResult struct {
	HasExcessiveLinks bool
	LinkCount         int
	Threshold         int
}

// SpamDetectorConfig configures spam detection thresholds.
type SpamDetectorConfig struct {
	MaxLinks          int
	MinTitleLength    int
	MinDescLength     int
	MaxCapsPercentage int
	ForbiddenWords    []string
	RepeatThreshold   int
}

// DefaultSpamDetectorConfig returns the default spam detector configuration.
func DefaultSpamDetectorConfig() SpamDetectorConfig {
	return SpamDetectorConfig{
		MaxLinks:          5,
		MinTitleLength:    10,
		MinDescLength:     50,
		MaxCapsPercentage: 50,
		ForbiddenWords: []string{
			"bitcoin", "crypto", "cryptocurrency",
			"free money", "get rich quick", "earn money fast",
			"click here now", "limited time offer",
			"buy now", "cheap pills", "pharmacy",
		},
		RepeatThreshold: 4,
	}
}

// RateAbuseConfig configures rate abuse detection.
type RateAbuseConfig struct {
	Threshold int           // Max posts allowed in window
	Window    time.Duration // Time window to check
}

// DefaultRateAbuseConfig returns the default rate abuse configuration.
func DefaultRateAbuseConfig() RateAbuseConfig {
	return RateAbuseConfig{
		Threshold: 10,
		Window:    time.Hour,
	}
}

// SpamDetector detects spam content.
type SpamDetector struct {
	config SpamDetectorConfig
}

// NewSpamDetector creates a new spam detector with default config.
func NewSpamDetector() *SpamDetector {
	return &SpamDetector{
		config: DefaultSpamDetectorConfig(),
	}
}

// NewSpamDetectorWithConfig creates a spam detector with custom config.
func NewSpamDetectorWithConfig(config SpamDetectorConfig) *SpamDetector {
	return &SpamDetector{
		config: config,
	}
}

// CheckSpam analyzes content for spam patterns.
func (d *SpamDetector) CheckSpam(content ModerationContent) SpamCheckResult {
	result := SpamCheckResult{
		IsSpam:  false,
		Reasons: []string{},
		Score:   0.0,
	}

	combined := content.Title + " " + content.Description

	// Check for excessive links
	linkCount := countLinksInText(combined)
	if linkCount > d.config.MaxLinks {
		result.Reasons = append(result.Reasons, "excessive_links")
		result.Score += 0.3
	}

	// Check for repeated text
	if hasRepeatedTextPattern(combined, d.config.RepeatThreshold) {
		result.Reasons = append(result.Reasons, "repeated_text")
		result.Score += 0.25
	}

	// Check for excessive caps in title
	capsPercentage := countCapsPercentageInText(content.Title)
	if capsPercentage > d.config.MaxCapsPercentage {
		result.Reasons = append(result.Reasons, "excessive_caps")
		result.Score += 0.2
	}

	// Check for forbidden words
	if containsForbiddenWordsInText(combined, d.config.ForbiddenWords) {
		result.Reasons = append(result.Reasons, "forbidden_words")
		result.Score += 0.3
	}

	// Check for minimum length
	if len(content.Title) < d.config.MinTitleLength || len(content.Description) < d.config.MinDescLength {
		result.Reasons = append(result.Reasons, "too_short")
		result.Score += 0.15
	}

	// Mark as spam if score exceeds threshold or any critical reason
	if result.Score >= 0.3 || len(result.Reasons) > 0 {
		result.IsSpam = true
	}

	return result
}

// CheckLinkSpam specifically checks for link spam.
func (d *SpamDetector) CheckLinkSpam(content ModerationContent) LinkSpamResult {
	combined := content.Title + " " + content.Description
	linkCount := countLinksInText(combined)

	return LinkSpamResult{
		HasExcessiveLinks: linkCount > d.config.MaxLinks,
		LinkCount:         linkCount,
		Threshold:         d.config.MaxLinks,
	}
}

// DefaultDuplicateMaxAge is how far back CheckContentDuplicate looks (24 hours).
const DefaultDuplicateMaxAge = 24 * time.Hour

// ModerationService provides content moderation functionality.
type ModerationService struct {
	flagCreator     FlagCreator
	duplicateFinder ContentDuplicateFinder
	rateChecker     RateLimitChecker
	spamDetector    *SpamDetector
	rateConfig      RateAbuseConfig
}

// NewModerationService creates a new moderation service.
func NewModerationService(
	flagCreator FlagCreator,
	rateChecker RateLimitChecker,
	spamDetector *SpamDetector,
) *ModerationService {
	if spamDetector == nil {
		spamDetector = NewSpamDetector()
	}
	return &ModerationService{
		flagCreator:  flagCreator,
		rateChecker:  rateChecker,
		spamDetector: spamDetector,
		rateConfig:   DefaultRateAbuseConfig(),
	}
}

// NewModerationServiceWithRateConfig creates a moderation service with custom rate config.
func NewModerationServiceWithRateConfig(
	flagCreator FlagCreator,
	rateChecker RateLimitChecker,
	spamDetector *SpamDetector,
	rateConfig RateAbuseConfig,
) *ModerationService {
	if spamDetector == nil {
		spamDetector = NewSpamDetector()
	}
	return &ModerationService{
		flagCreator:  flagCreator,
		rateChecker:  rateChecker,
		spamDetector: spamDetector,
		rateConfig:   rateConfig,
	}
}

// SetDuplicateFinder makes duplicate detection read the canonical posts and replies
// tables. Once set, AutoFlagIfNeeded uses it instead of the hash store.
func (s *ModerationService) SetDuplicateFinder(finder ContentDuplicateFinder) {
	s.duplicateFinder = finder
}

// CheckContentDuplicate reports whether the post or reply targetID repeats an earlier live
// post (same title and description) or an earlier live reply on the same post (same body)
// created within DefaultDuplicateMaxAge. Legacy contribution target types are not looked up.
func (s *ModerationService) CheckContentDuplicate(ctx context.Context, targetID uuid.UUID, targetType string, content ModerationContent) (*DuplicateCheckResult, error) {
	if s.duplicateFinder == nil {
		return &DuplicateCheckResult{IsDuplicate: false}, nil
	}

	since := time.Now().Add(-DefaultDuplicateMaxAge)
	var match *models.ContentDuplicate
	var err error
	switch targetType {
	case "post":
		match, err = s.duplicateFinder.FindPost(ctx, content.Title, content.Description, since, targetID.String())
	case "reply":
		match, err = s.duplicateFinder.FindReply(ctx, content.PostID, content.Description, since, targetID.String())
	default:
		return &DuplicateCheckResult{IsDuplicate: false}, nil
	}
	if err != nil {
		return nil, err
	}
	if match == nil {
		return &DuplicateCheckResult{IsDuplicate: false}, nil
	}

	postID, err := uuid.Parse(match.PostID)
	if err != nil {
		return nil, fmt.Errorf("duplicate %s %s: invalid post id %q: %w", match.TargetType, match.TargetID, match.PostID, err)
	}
	return &DuplicateCheckResult{
		IsDuplicate:        true,
		OriginalPostID:     postID,
		Similarity:         1.0, // exact match
		OriginalTargetType: match.TargetType,
		OriginalTargetID:   match.TargetID,
	}, nil
}

// CheckRateAbuse checks if a user is posting too frequently.
func (s *ModerationService) CheckRateAbuse(ctx context.Context, userType, userID string) (*RateAbuseResult, error) {
	if s.rateChecker == nil {
		return &RateAbuseResult{IsAbusive: false}, nil
	}

	postCount, err := s.rateChecker.GetRecentPostCount(ctx, userType, userID, s.rateConfig.Window)
	if err != nil {
		return nil, err
	}

	return &RateAbuseResult{
		IsAbusive: postCount > s.rateConfig.Threshold,
		PostCount: postCount,
		Window:    s.rateConfig.Window,
		Threshold: s.rateConfig.Threshold,
	}, nil
}

// AutoFlagIfNeeded checks content and creates a flag if moderation rules are violated.
// A reply has no title, so the post-shaped spam rules skip it and only the duplicate
// check runs.
func (s *ModerationService) AutoFlagIfNeeded(ctx context.Context, targetID uuid.UUID, targetType string, content ModerationContent) error {
	// Check for spam
	if targetType != "reply" {
		spamResult := s.spamDetector.CheckSpam(content)
		if spamResult.IsSpam {
			return s.createSystemFlag(ctx, targetID, targetType, "spam", strings.Join(spamResult.Reasons, ", "))
		}
	}

	// Check for duplicate
	if s.duplicateFinder != nil {
		dupResult, err := s.CheckContentDuplicate(ctx, targetID, targetType, content)
		if err != nil {
			return err
		}
		if dupResult.IsDuplicate {
			details := "duplicate of " + dupResult.OriginalTargetType + " " + dupResult.OriginalTargetID
			if dupResult.OriginalTargetType == "reply" {
				details += " on post " + dupResult.OriginalPostID.String()
			}
			return s.createSystemFlag(ctx, targetID, targetType, "duplicate", details)
		}
	}

	return nil
}

// AutoFlagForRateAbuse creates a flag for rate abuse if detected.
func (s *ModerationService) AutoFlagForRateAbuse(ctx context.Context, targetID uuid.UUID, targetType string, userType, userID string) error {
	abuseResult, err := s.CheckRateAbuse(ctx, userType, userID)
	if err != nil {
		return err
	}

	if abuseResult.IsAbusive {
		details := "posted " + string(rune(abuseResult.PostCount+'0')) + " times in " + abuseResult.Window.String()
		return s.createSystemFlag(ctx, targetID, targetType, "low_quality", details)
	}

	return nil
}

// createSystemFlag creates a flag from the system auto-moderation.
func (s *ModerationService) createSystemFlag(ctx context.Context, targetID uuid.UUID, targetType, reason, details string) error {
	if s.flagCreator == nil {
		return nil
	}

	flag := &models.Flag{
		TargetType:   targetType,
		TargetID:     targetID,
		ReporterType: "system",
		ReporterID:   "auto-moderation",
		Reason:       reason,
		Details:      details,
		Status:       "pending",
	}

	_, err := s.flagCreator.CreateFlag(ctx, flag)
	return err
}

// --- Helper functions ---

// countLinksInText counts HTTP/HTTPS links in text.
func countLinksInText(text string) int {
	count := 0
	count += strings.Count(text, "http://")
	count += strings.Count(text, "https://")
	return count
}

// countCapsPercentageInText returns the percentage of uppercase letters.
func countCapsPercentageInText(text string) int {
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

// hasRepeatedTextPattern checks for phrases repeated more than threshold times.
func hasRepeatedTextPattern(text string, threshold int) bool {
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

// containsForbiddenWordsInText checks if text contains any forbidden words/phrases.
func containsForbiddenWordsInText(text string, forbidden []string) bool {
	lower := strings.ToLower(text)
	for _, word := range forbidden {
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}
