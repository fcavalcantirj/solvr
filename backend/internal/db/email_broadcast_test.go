package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func setupEmailBroadcastTest(t *testing.T) (*Pool, *EmailBroadcastRepository) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Clean up test data from previous runs
	_, _ = pool.Exec(ctx, "DELETE FROM email_broadcast_logs WHERE subject LIKE 'test_%'")

	repo := NewEmailBroadcastRepository(pool)
	return pool, repo
}

func TestEmailBroadcastRepository_CreateLog(t *testing.T) {
	pool, repo := setupEmailBroadcastTest(t)
	defer pool.Close()

	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_broadcast_logs WHERE subject LIKE 'test_%'")
	}()

	broadcast := &models.EmailBroadcast{
		Subject:         "test_broadcast_subject",
		BodyHTML:        "<p>Hello</p>",
		TotalRecipients: 5,
		Status:          "sending",
	}

	result, err := repo.CreateLog(ctx, broadcast)
	if err != nil {
		t.Fatalf("CreateLog() error = %v", err)
	}

	if result.ID == "" {
		t.Error("expected non-empty ID")
	}
	if result.Subject != "test_broadcast_subject" {
		t.Errorf("expected Subject 'test_broadcast_subject', got '%s'", result.Subject)
	}
	if result.TotalRecipients != 5 {
		t.Errorf("expected TotalRecipients 5, got %d", result.TotalRecipients)
	}
	if result.SentCount != 0 {
		t.Errorf("expected SentCount 0, got %d", result.SentCount)
	}
	if result.FailedCount != 0 {
		t.Errorf("expected FailedCount 0, got %d", result.FailedCount)
	}
	if result.Status != "sending" {
		t.Errorf("expected Status 'sending', got '%s'", result.Status)
	}
	if result.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
	if result.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestEmailBroadcastRepository_UpdateStatusAndCounts(t *testing.T) {
	pool, repo := setupEmailBroadcastTest(t)
	defer pool.Close()

	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_broadcast_logs WHERE subject LIKE 'test_%'")
	}()

	broadcast := &models.EmailBroadcast{
		Subject:         "test_update_subject",
		BodyHTML:        "<p>Test</p>",
		TotalRecipients: 5,
		Status:          "sending",
	}

	created, err := repo.CreateLog(ctx, broadcast)
	if err != nil {
		t.Fatalf("CreateLog() error = %v", err)
	}

	now := time.Now()
	err = repo.UpdateStatusAndCounts(ctx, created.ID, "completed", 4, 1, &now)
	if err != nil {
		t.Fatalf("UpdateStatusAndCounts() error = %v", err)
	}

	// Verify by raw SQL query
	var status string
	var sentCount, failedCount int
	var completedAt *time.Time
	err = pool.QueryRow(ctx,
		"SELECT status, sent_count, failed_count, completed_at FROM email_broadcast_logs WHERE id = $1",
		created.ID,
	).Scan(&status, &sentCount, &failedCount, &completedAt)
	if err != nil {
		t.Fatalf("verification query failed: %v", err)
	}

	if status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", status)
	}
	if sentCount != 4 {
		t.Errorf("expected sent_count 4, got %d", sentCount)
	}
	if failedCount != 1 {
		t.Errorf("expected failed_count 1, got %d", failedCount)
	}
	if completedAt == nil {
		t.Error("expected completed_at to be non-NULL")
	}
}

func TestEmailBroadcastRepository_List(t *testing.T) {
	pool, repo := setupEmailBroadcastTest(t)
	defer pool.Close()

	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM email_broadcast_logs WHERE subject LIKE 'test_%'")
	}()

	now := time.Now()

	// Insert first broadcast (older)
	_, err := pool.Exec(ctx, `
		INSERT INTO email_broadcast_logs (subject, body_html, total_recipients, status, started_at)
		VALUES ($1, $2, $3, $4, $5)
	`, "test_list_first", "<p>First</p>", 3, "completed", now.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("insert first broadcast: %v", err)
	}

	// Insert second broadcast (newer)
	_, err = pool.Exec(ctx, `
		INSERT INTO email_broadcast_logs (subject, body_html, total_recipients, status, started_at)
		VALUES ($1, $2, $3, $4, $5)
	`, "test_list_second", "<p>Second</p>", 2, "sending", now)
	if err != nil {
		t.Fatalf("insert second broadcast: %v", err)
	}

	results, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// Find our test entries and verify DESC order
	var firstSubject string
	for _, r := range results {
		if r.Subject == "test_list_first" || r.Subject == "test_list_second" {
			firstSubject = r.Subject
			break
		}
	}

	if firstSubject != "test_list_second" {
		t.Errorf("expected first test result to be 'test_list_second' (DESC order), got '%s'", firstSubject)
	}
}

func TestEmailBroadcastRepository_List_Empty(t *testing.T) {
	pool, repo := setupEmailBroadcastTest(t)
	defer pool.Close()

	ctx := context.Background()

	// Ensure no test rows exist
	_, _ = pool.Exec(ctx, "DELETE FROM email_broadcast_logs WHERE subject LIKE 'test_%'")

	// Note: this test only verifies behavior for the test-prefixed rows cleaned above.
	// In a shared DB there may be other rows, so we just verify no error and non-nil slice.
	results, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if results == nil {
		t.Error("expected non-nil slice, got nil")
	}
}
