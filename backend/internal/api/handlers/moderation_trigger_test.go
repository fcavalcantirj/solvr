package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestModerationTrigger_Approved(t *testing.T) {
	statusUpdater := NewMockPostStatusUpdater()
	commentCreator := &MockCommentCreator{}
	notifService := &MockNotificationServiceForModeration{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         true,
		LanguageDetected: "English",
		Explanation:      "OK",
	})

	trigger := NewModerationTrigger(modService, statusUpdater, newTestLogger())
	trigger.SetCommentRepo(commentCreator)
	trigger.SetNotificationService(notifService)
	trigger.SetRetryDelays([]time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond})

	trigger.TriggerAsync(testPostID, "Translated Title", "Translated Description", []string{"go"}, "idea", "human", "user-123")
	time.Sleep(100 * time.Millisecond)

	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, status)
	}

	comments := commentCreator.GetComments()
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if !strings.Contains(comments[0].Content, "approved") {
		t.Errorf("expected approval comment, got %q", comments[0].Content)
	}

	notifs := notifService.GetNotifications()
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	if !notifs[0].Approved {
		t.Error("expected approved=true in notification")
	}
}

func TestModerationTrigger_Rejected(t *testing.T) {
	statusUpdater := NewMockPostStatusUpdater()
	commentCreator := &MockCommentCreator{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:    false,
		Explanation: "Content is spam",
	})

	trigger := NewModerationTrigger(modService, statusUpdater, newTestLogger())
	trigger.SetCommentRepo(commentCreator)
	trigger.SetRetryDelays([]time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond})

	trigger.TriggerAsync(testPostID, "Translated Title", "Translated Description", nil, "idea", "human", "user-1")
	time.Sleep(100 * time.Millisecond)

	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusRejected {
		t.Errorf("expected status %q, got %q", models.PostStatusRejected, status)
	}

	comments := commentCreator.GetComments()
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if !strings.Contains(comments[0].Content, "rejected") {
		t.Errorf("expected rejection comment, got %q", comments[0].Content)
	}
}

func TestModerationTrigger_RetryOnError(t *testing.T) {
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.QueueResults(
		[]*ModerationResult{nil, {Approved: true, Explanation: "OK"}},
		[]error{fmt.Errorf("temporary error"), nil},
	)

	trigger := NewModerationTrigger(modService, statusUpdater, newTestLogger())
	trigger.SetRetryDelays([]time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond})

	trigger.TriggerAsync(testPostID, "Title", "Description", nil, "idea", "human", "user-1")
	time.Sleep(100 * time.Millisecond)

	if calls := modService.GetCalls(); calls != 2 {
		t.Errorf("expected 2 moderation calls, got %d", calls)
	}

	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called after retry")
	}
	if status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, status)
	}
}

func TestModerationTrigger_AllRetriesFail_CreatesFlag(t *testing.T) {
	statusUpdater := NewMockPostStatusUpdater()
	flagCreator := &MockFlagCreator{}
	modService := NewMockContentModerationService()
	modService.QueueResults(
		[]*ModerationResult{nil, nil, nil},
		[]error{fmt.Errorf("err 1"), fmt.Errorf("err 2"), fmt.Errorf("err 3")},
	)

	trigger := NewModerationTrigger(modService, statusUpdater, newTestLogger())
	trigger.SetFlagCreator(flagCreator)
	trigger.SetRetryDelays([]time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond})

	trigger.TriggerAsync(testPostID, "Title", "Description", nil, "idea", "human", "user-1")
	time.Sleep(100 * time.Millisecond)

	if calls := modService.GetCalls(); calls != 3 {
		t.Errorf("expected 3 moderation calls, got %d", calls)
	}

	_, ok := statusUpdater.GetStatus(testPostID)
	if ok {
		t.Error("expected UpdateStatus NOT to be called when all retries fail")
	}

	flags := flagCreator.GetFlags()
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}
	if flags[0].Reason != "moderation_failed" {
		t.Errorf("expected flag reason 'moderation_failed', got %q", flags[0].Reason)
	}
	if flags[0].ReporterID != "translation-moderation" {
		t.Errorf("expected reporter 'translation-moderation', got %q", flags[0].ReporterID)
	}
}

func TestModerationTrigger_RateLimitRetry(t *testing.T) {
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.QueueResults(
		[]*ModerationResult{nil, {Approved: true, Explanation: "OK"}},
		[]error{&testRateLimitError{retryAfter: 1 * time.Millisecond, message: "rate limited"}, nil},
	)

	trigger := NewModerationTrigger(modService, statusUpdater, newTestLogger())
	trigger.SetRetryDelays([]time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond})

	trigger.TriggerAsync(testPostID, "Title", "Description", nil, "idea", "human", "user-1")
	time.Sleep(100 * time.Millisecond)

	// Rate limit retries do NOT count as attempts
	if calls := modService.GetCalls(); calls != 2 {
		t.Errorf("expected 2 moderation calls, got %d", calls)
	}

	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusOpen {
		t.Errorf("expected status %q, got %q", models.PostStatusOpen, status)
	}
}

func TestModerationTrigger_NoLanguageOnlyRejection(t *testing.T) {
	// Unlike PostsHandler.moderatePostAsync, ModerationTrigger should NOT
	// detect language-only rejections. Post-translation should only approve/reject.
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE"},
		Explanation:      "Content is in Portuguese",
	})

	trigger := NewModerationTrigger(modService, statusUpdater, newTestLogger())
	trigger.SetRetryDelays([]time.Duration{1 * time.Millisecond})

	trigger.TriggerAsync(testPostID, "Title", "Description", nil, "idea", "human", "user-1")
	time.Sleep(100 * time.Millisecond)

	// Should be rejected, NOT set to draft for re-translation
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateStatus to be called")
	}
	if status != models.PostStatusRejected {
		t.Errorf("expected status %q (not draft), got %q", models.PostStatusRejected, status)
	}

	// Should NOT have called UpdateOriginalLanguage
	_, langSet := statusUpdater.GetLanguage(testPostID)
	if langSet {
		t.Error("expected UpdateOriginalLanguage NOT to be called — post-translation should not trigger re-translation")
	}
}

func TestModerationTrigger_PanicRecovery(t *testing.T) {
	statusUpdater := NewMockPostStatusUpdater()
	modService := &panicModerationService{}

	trigger := NewModerationTrigger(modService, statusUpdater, newTestLogger())
	trigger.SetRetryDelays([]time.Duration{1 * time.Millisecond})

	// Should NOT panic the test — goroutine recovers
	trigger.TriggerAsync(testPostID, "Title", "Description", nil, "idea", "human", "user-1")
	time.Sleep(100 * time.Millisecond)

	// Status should NOT be updated (panic prevented completion)
	_, ok := statusUpdater.GetStatus(testPostID)
	if ok {
		t.Error("expected UpdateStatus NOT to be called after panic")
	}
}

// panicModerationService panics on ModerateContent to test recovery.
type panicModerationService struct{}

func (p *panicModerationService) ModerateContent(_ context.Context, _ ModerationInput) (*ModerationResult, error) {
	panic("simulated groq panic")
}

func TestModerationTrigger_Timeout60s(t *testing.T) {
	trigger := NewModerationTrigger(nil, nil, newTestLogger())
	if trigger.timeout != 60*time.Second {
		t.Errorf("expected default timeout 60s, got %v", trigger.timeout)
	}
}
