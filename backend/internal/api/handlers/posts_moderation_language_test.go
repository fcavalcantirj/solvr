package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// isLanguageOnlyRejection Tests
// ============================================================================

func TestIsLanguageOnlyRejection_TrueForSingleLanguageReason(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE"},
	}
	if !isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=true for single LANGUAGE reason")
	}
}

// TestIsLanguageOnlyRejection_RealGroqPhrasing ensures the real Groq output
// "Non-English language" is treated as a language-only rejection.
func TestIsLanguageOnlyRejection_RealGroqPhrasing(t *testing.T) {
	// Real Groq moderation returns this exact phrase (confirmed via live API test)
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"Non-English language"},
	}
	if !isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=true for real Groq reason 'Non-English language'")
	}
}

func TestIsLanguageOnlyRejection_CaseInsensitive(t *testing.T) {
	cases := []string{"LANGUAGE", "language", "Language"}
	for _, reason := range cases {
		result := &ModerationResult{
			Approved:         false,
			LanguageDetected: "Spanish",
			RejectionReasons: []string{reason},
		}
		if !isLanguageOnlyRejection(result) {
			t.Errorf("expected isLanguageOnlyRejection=true for reason %q", reason)
		}
	}
}

func TestIsLanguageOnlyRejection_FalseWhenApproved(t *testing.T) {
	result := &ModerationResult{
		Approved:         true,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE"},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false when approved")
	}
}

func TestIsLanguageOnlyRejection_FalseWhenNoLanguage(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "",
		RejectionReasons: []string{"LANGUAGE"},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false when no language detected")
	}
}

func TestIsLanguageOnlyRejection_FalseWhenEnglish(t *testing.T) {
	for _, lang := range []string{"en", "EN", "english", "English", "ENGLISH"} {
		result := &ModerationResult{
			Approved:         false,
			LanguageDetected: lang,
			RejectionReasons: []string{"LANGUAGE"},
		}
		if isLanguageOnlyRejection(result) {
			t.Errorf("expected isLanguageOnlyRejection=false for English language %q", lang)
		}
	}
}

func TestIsLanguageOnlyRejection_FalseWhenMultipleReasons(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE", "SPAM"},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false when multiple rejection reasons")
	}
}

func TestIsLanguageOnlyRejection_FalseWhenNonLanguageReason(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"SPAM"},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false for non-LANGUAGE reason")
	}
}

func TestIsLanguageOnlyRejection_FalseWhenNoReasons(t *testing.T) {
	result := &ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{},
	}
	if isLanguageOnlyRejection(result) {
		t.Error("expected isLanguageOnlyRejection=false when no rejection reasons")
	}
}

// ============================================================================
// Language-only rejection → draft + translation comment
// ============================================================================

func TestModeratePostAsync_LanguageOnlyRejection_SetsDraft(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	commentCreator := &MockCommentCreator{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE"},
		Explanation:      "Content is in Portuguese, not English",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetCommentRepo(commentCreator)

	handler.moderatePostAsync(testPostID, "Título de teste", "Descrição de teste", []string{"go"}, "question", "human", "user-123")

	// Status should be draft (not rejected)
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected UpdateOriginalLanguage to set status")
	}
	if status != models.PostStatusDraft {
		t.Errorf("expected status %q, got %q", models.PostStatusDraft, status)
	}

	// Language should be recorded
	lang, langOK := statusUpdater.GetLanguage(testPostID)
	if !langOK {
		t.Fatal("expected original language to be recorded")
	}
	if lang != "Portuguese" {
		t.Errorf("expected language 'Portuguese', got %q", lang)
	}

	// Comment should mention translation
	comments := commentCreator.GetComments()
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	c := comments[0]
	if !strings.Contains(c.Content, "Portuguese") {
		t.Errorf("expected comment to mention 'Portuguese', got %q", c.Content)
	}
	if !strings.Contains(strings.ToLower(c.Content), "translat") {
		t.Errorf("expected comment to mention translation, got %q", c.Content)
	}
}

// mockTranslationTrigger records calls to TranslateAndModerateAsync.
type mockTranslationTrigger struct {
	calls []mockTranslationCall
}

type mockTranslationCall struct {
	postID   string
	language string
}

func (m *mockTranslationTrigger) TranslateAndModerateAsync(postID, title, description string, tags []string, language, postType, authorType, authorID string) {
	m.calls = append(m.calls, mockTranslationCall{postID: postID, language: language})
}

func TestModeratePostAsync_LanguageOnlyRejection_TriggersInlineTranslation(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	commentCreator := &MockCommentCreator{}
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		LanguageDetected: "Chinese",
		RejectionReasons: []string{"LANGUAGE"},
		Explanation:      "Content is in Chinese",
	})

	trigger := &mockTranslationTrigger{}

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetCommentRepo(commentCreator)
	handler.SetTranslationTrigger(trigger)

	handler.moderatePostAsync(testPostID, "中文标题", "中文描述", []string{"go"}, "problem", "agent", "agent-1")

	// Translation trigger should have been called exactly once
	if len(trigger.calls) != 1 {
		t.Fatalf("expected 1 translation trigger call, got %d", len(trigger.calls))
	}
	if trigger.calls[0].postID != testPostID {
		t.Errorf("expected postID %q, got %q", testPostID, trigger.calls[0].postID)
	}
	if trigger.calls[0].language != "Chinese" {
		t.Errorf("expected language 'Chinese', got %q", trigger.calls[0].language)
	}
}

func TestModeratePostAsync_NoTranslationTrigger_StillWorks(t *testing.T) {
	// When no translation trigger is set, language-only rejection still sets draft
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		LanguageDetected: "Spanish",
		RejectionReasons: []string{"LANGUAGE"},
		Explanation:      "Content is in Spanish",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	// No translation trigger set

	handler.moderatePostAsync(testPostID, "Título", "Descripción", []string{}, "question", "human", "user-1")

	// Status should still be draft
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected status to be set")
	}
	if status != models.PostStatusDraft {
		t.Errorf("expected status %q, got %q", models.PostStatusDraft, status)
	}
}

func TestModeratePostAsync_ApprovedPost_NoTranslationTrigger(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved: true,
	})

	trigger := &mockTranslationTrigger{}

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetTranslationTrigger(trigger)

	handler.moderatePostAsync(testPostID, "English title", "English desc", []string{}, "question", "human", "user-1")

	// Approved → no translation trigger
	if len(trigger.calls) != 0 {
		t.Errorf("expected 0 translation trigger calls for approved post, got %d", len(trigger.calls))
	}
}

func TestModeratePostAsync_MultipleReasonsNotDraft(t *testing.T) {
	repo := NewMockPostsRepository()
	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{
		Approved:         false,
		LanguageDetected: "Portuguese",
		RejectionReasons: []string{"LANGUAGE", "SPAM"},
		Explanation:      "Content is in Portuguese and is spam",
	})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)

	handler.moderatePostAsync(testPostID, "Título de teste", "Descrição de teste", []string{"go"}, "question", "human", "user-123")

	// Multiple reasons → regular rejection, not draft
	status, ok := statusUpdater.GetStatus(testPostID)
	if !ok {
		t.Fatal("expected status to be set")
	}
	if status != models.PostStatusRejected {
		t.Errorf("expected status %q for multi-reason rejection, got %q", models.PostStatusRejected, status)
	}
}
