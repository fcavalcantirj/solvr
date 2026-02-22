package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// ============================================================================
// Mock implementations
// ============================================================================

type mockTranslationLister struct {
	posts []*models.Post
	err   error
}

func (m *mockTranslationLister) ListPostsNeedingTranslation(ctx context.Context, limit int) ([]*models.Post, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.posts, nil
}

type mockTranslationUpdater struct {
	applied    []applyCall
	increments []string
	applyErr   error
	incrErr    error
}

type applyCall struct {
	postID      string
	title       string
	description string
}

func (m *mockTranslationUpdater) ApplyTranslation(ctx context.Context, postID, title, description string) error {
	if m.applyErr != nil {
		return m.applyErr
	}
	m.applied = append(m.applied, applyCall{postID, title, description})
	return nil
}

func (m *mockTranslationUpdater) IncrementTranslationAttempts(ctx context.Context, postID string) error {
	if m.incrErr != nil {
		return m.incrErr
	}
	m.increments = append(m.increments, postID)
	return nil
}

type mockPostTranslator struct {
	results  map[string]*services.TranslationResult
	err      error
	rateLimitErr *services.TranslationRateLimitError
	calls    []services.TranslationInput
}

func (m *mockPostTranslator) TranslateContent(ctx context.Context, input services.TranslationInput) (*services.TranslationResult, error) {
	m.calls = append(m.calls, input)
	if m.rateLimitErr != nil {
		return nil, m.rateLimitErr
	}
	if m.err != nil {
		return nil, m.err
	}
	if m.results != nil {
		if r, ok := m.results[input.Title]; ok {
			return r, nil
		}
	}
	return &services.TranslationResult{
		Title:       "Translated: " + input.Title,
		Description: "Translated: " + input.Description,
	}, nil
}

type mockModerationTrigger struct {
	triggered []moderationTriggerCall
}

type moderationTriggerCall struct {
	postID      string
	title       string
	description string
}

func (m *mockModerationTrigger) TriggerAsync(postID, title, description string, tags []string, postType, authorType, authorID string) {
	m.triggered = append(m.triggered, moderationTriggerCall{postID, title, description})
}

// ============================================================================
// Tests
// ============================================================================

func TestTranslationJob_RunOnce_NoCandidates(t *testing.T) {
	lister := &mockTranslationLister{posts: []*models.Post{}}
	updater := &mockTranslationUpdater{}
	translator := &mockPostTranslator{}
	trigger := &mockModerationTrigger{}

	job := NewTranslationJob(lister, updater, translator, trigger, 5, 0)
	translated, failed := job.RunOnce(context.Background())

	if translated != 0 {
		t.Errorf("RunOnce() translated = %d, want 0", translated)
	}
	if failed != 0 {
		t.Errorf("RunOnce() failed = %d, want 0", failed)
	}
	if len(updater.applied) != 0 {
		t.Errorf("expected no ApplyTranslation calls, got %d", len(updater.applied))
	}
}

func TestTranslationJob_RunOnce_Success(t *testing.T) {
	posts := []*models.Post{
		{
			ID:               "post-1",
			Type:             models.PostTypeProblem,
			Title:            "Como usar goroutines",
			Description:      "Estou tentando entender goroutines",
			Tags:             []string{"go"},
			PostedByType:     models.AuthorTypeHuman,
			PostedByID:       "user-1",
			OriginalLanguage: "Portuguese",
		},
		{
			ID:               "post-2",
			Type:             models.PostTypeQuestion,
			Title:            "Cómo usar channels",
			Description:      "Intento entender channels",
			Tags:             []string{"go"},
			PostedByType:     models.AuthorTypeHuman,
			PostedByID:       "user-2",
			OriginalLanguage: "Spanish",
		},
	}
	lister := &mockTranslationLister{posts: posts}
	updater := &mockTranslationUpdater{}
	translator := &mockPostTranslator{}
	trigger := &mockModerationTrigger{}

	job := NewTranslationJob(lister, updater, translator, trigger, 5, 0)
	translated, failed := job.RunOnce(context.Background())

	if translated != 2 {
		t.Errorf("RunOnce() translated = %d, want 2", translated)
	}
	if failed != 0 {
		t.Errorf("RunOnce() failed = %d, want 0", failed)
	}
	if len(updater.applied) != 2 {
		t.Errorf("expected 2 ApplyTranslation calls, got %d", len(updater.applied))
	}
	if len(trigger.triggered) != 2 {
		t.Errorf("expected 2 moderation triggers, got %d", len(trigger.triggered))
	}
	// Verify translated content was passed to ApplyTranslation
	if updater.applied[0].postID != "post-1" {
		t.Errorf("expected postID 'post-1', got %q", updater.applied[0].postID)
	}
	// Translated title should contain something (mock returns "Translated: <title>")
	if updater.applied[0].title == "" {
		t.Error("expected non-empty translated title")
	}
}

func TestTranslationJob_RunOnce_RateLimitStopsBatch(t *testing.T) {
	posts := []*models.Post{
		{ID: "post-1", Title: "Post 1", OriginalLanguage: "Portuguese"},
		{ID: "post-2", Title: "Post 2", OriginalLanguage: "Spanish"},
		{ID: "post-3", Title: "Post 3", OriginalLanguage: "French"},
	}
	lister := &mockTranslationLister{posts: posts}
	updater := &mockTranslationUpdater{}
	trigger := &mockModerationTrigger{}

	callCount := 0
	translator := &mockPostTranslator{}
	// Override to fail with rate limit on second call
	customTranslator := &rateLimitOnSecondCall{
		rateLimitErr: &services.TranslationRateLimitError{
			RetryAfter: 1 * time.Millisecond,
			Message:    "rate limited",
		},
	}

	job := NewTranslationJob(lister, updater, customTranslator, trigger, 5, 0)
	_ = callCount
	_ = translator
	translated, failed := job.RunOnce(context.Background())

	// Only first post should be translated; second triggers rate limit → stop
	if translated != 1 {
		t.Errorf("RunOnce() translated = %d, want 1", translated)
	}
	// Rate limited post should have its attempts incremented
	if len(updater.increments) != 1 {
		t.Errorf("expected 1 IncrementTranslationAttempts call, got %d", len(updater.increments))
	}
	if updater.increments[0] != "post-2" {
		t.Errorf("expected increment for 'post-2', got %q", updater.increments[0])
	}
	_ = failed
}

// rateLimitOnSecondCall is a translator that succeeds on first call but rate-limits on second.
type rateLimitOnSecondCall struct {
	calls        int
	rateLimitErr *services.TranslationRateLimitError
}

func (m *rateLimitOnSecondCall) TranslateContent(ctx context.Context, input services.TranslationInput) (*services.TranslationResult, error) {
	m.calls++
	if m.calls >= 2 {
		return nil, m.rateLimitErr
	}
	return &services.TranslationResult{
		Title:       "Translated: " + input.Title,
		Description: "Translated: " + input.Description,
	}, nil
}

func TestTranslationJob_RunOnce_ErrorIncrementsAttempts(t *testing.T) {
	posts := []*models.Post{
		{ID: "post-1", Title: "Post 1", OriginalLanguage: "Portuguese"},
		{ID: "post-2", Title: "Post 2", OriginalLanguage: "Spanish"},
	}
	lister := &mockTranslationLister{posts: posts}
	updater := &mockTranslationUpdater{}
	translator := &mockPostTranslator{err: errors.New("translation service unavailable")}
	trigger := &mockModerationTrigger{}

	job := NewTranslationJob(lister, updater, translator, trigger, 5, 0)
	translated, failed := job.RunOnce(context.Background())

	if translated != 0 {
		t.Errorf("RunOnce() translated = %d, want 0", translated)
	}
	if failed != 2 {
		t.Errorf("RunOnce() failed = %d, want 2", failed)
	}
	// Both posts should have their attempts incremented
	if len(updater.increments) != 2 {
		t.Errorf("expected 2 IncrementTranslationAttempts calls, got %d", len(updater.increments))
	}
	// No moderation should have been triggered
	if len(trigger.triggered) != 0 {
		t.Errorf("expected 0 moderation triggers, got %d", len(trigger.triggered))
	}
}

func TestTranslationJob_RunOnce_ListError(t *testing.T) {
	lister := &mockTranslationLister{err: errors.New("database unavailable")}
	updater := &mockTranslationUpdater{}
	translator := &mockPostTranslator{}
	trigger := &mockModerationTrigger{}

	job := NewTranslationJob(lister, updater, translator, trigger, 5, 0)
	translated, failed := job.RunOnce(context.Background())

	if translated != 0 {
		t.Errorf("RunOnce() translated = %d, want 0", translated)
	}
	if failed != 0 {
		t.Errorf("RunOnce() failed = %d, want 0", failed)
	}
}

func TestTranslationJob_RunScheduled_StopsOnCancel(t *testing.T) {
	lister := &mockTranslationLister{posts: []*models.Post{}}
	updater := &mockTranslationUpdater{}
	translator := &mockPostTranslator{}
	trigger := &mockModerationTrigger{}

	job := NewTranslationJob(lister, updater, translator, trigger, 5, 0)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		job.RunScheduled(ctx, 10*time.Millisecond)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("job did not stop within timeout")
	}
}

func TestTranslationJob_DefaultInterval_TwiceDaily(t *testing.T) {
	expected := 12 * time.Hour
	if DefaultTranslationInterval != expected {
		t.Errorf("DefaultTranslationInterval = %v, want %v (twice daily)", DefaultTranslationInterval, expected)
	}
}

func TestTranslationJob_DefaultBatchSize(t *testing.T) {
	if DefaultTranslationBatchSize <= 0 {
		t.Errorf("DefaultTranslationBatchSize = %d, should be > 0", DefaultTranslationBatchSize)
	}
}

func TestTranslationJob_RunOnce_PassesLanguageToTranslator(t *testing.T) {
	posts := []*models.Post{
		{
			ID:               "post-1",
			Title:            "Título em Português",
			Description:      "Descrição",
			OriginalLanguage: "Portuguese",
		},
	}
	lister := &mockTranslationLister{posts: posts}
	updater := &mockTranslationUpdater{}
	translator := &mockPostTranslator{}
	trigger := &mockModerationTrigger{}

	job := NewTranslationJob(lister, updater, translator, trigger, 5, 0)
	job.RunOnce(context.Background())

	if len(translator.calls) != 1 {
		t.Fatalf("expected 1 translator call, got %d", len(translator.calls))
	}
	call := translator.calls[0]
	if call.Language != "Portuguese" {
		t.Errorf("expected Language 'Portuguese', got %q", call.Language)
	}
	if call.Title != "Título em Português" {
		t.Errorf("expected Title 'Título em Português', got %q", call.Title)
	}
}
