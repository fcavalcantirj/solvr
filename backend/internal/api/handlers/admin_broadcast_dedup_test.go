package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestBroadcastEmail_Deduplication_BlocksDuplicate tests that a broadcast
// with the same subject as a recent completed broadcast is blocked.
func TestBroadcastEmail_Deduplication_BlocksDuplicate(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "prev-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>","dry_run":false}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	errObj, _ := resp["error"].(map[string]interface{})
	if errObj["code"] != "DUPLICATE_BROADCAST" {
		t.Errorf("expected DUPLICATE_BROADCAST error code, got %v", resp["error"])
	}

	// No emails should be sent
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_Deduplication_ForceOverride tests that force=true bypasses dedup.
func TestBroadcastEmail_Deduplication_ForceOverride(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "prev-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>","dry_run":false,"force":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK with force=true, got %d: %s", rr.Code, rr.Body.String())
	}

	// Email SHOULD be sent with force=true
	if len(sender.calls) != 1 {
		t.Errorf("expected 1 email send with force=true, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_Deduplication_SingleRecipientAlsoBlocked tests that the "to" field
// does NOT bypass dedup — a retry to a single recipient should also be blocked.
func TestBroadcastEmail_Deduplication_SingleRecipientAlsoBlocked(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "prev-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>","to":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// Single-recipient sends should ALSO be blocked by dedup
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for single-recipient dedup, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_Deduplication_RepoErrorFailsClosed tests that if HasRecentBroadcast
// returns an error, the broadcast is BLOCKED (fail-closed), not silently allowed.
func TestBroadcastEmail_Deduplication_RepoErrorFailsClosed(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcastErr: fmt.Errorf("database connection lost"),
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// Should fail-closed: return 500, NOT send emails
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on dedup check error, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends on dedup error, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_Deduplication_SendingStatusBlocks tests that a broadcast still
// in "sending" status (not yet completed) also blocks retries.
func TestBroadcastEmail_Deduplication_SendingStatusBlocks(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "in-flight-broadcast",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       150, // still sending, only 150 of 315 done
			Status:          "sending",
			StartedAt:       time.Now().Add(-45 * time.Second),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// In-flight broadcast should block retry
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for in-flight broadcast, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_DryRun_SkipsDedup tests that dry-run mode does NOT trigger dedup,
// even when a recent broadcast with the same subject exists.
func TestBroadcastEmail_DryRun_SkipsDedup(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "prev-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// Dry-run should return 200 with preview, NOT 409
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK for dry-run (should skip dedup), got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if _, ok := resp["would_send"]; !ok {
		t.Error("expected dry-run response with 'would_send' field")
	}
}

// TestBroadcastEmail_Deduplication_FullyFailedBroadcastAlsoBlocks tests that a broadcast
// where ALL emails failed (sent_count=0) still blocks retries.
func TestBroadcastEmail_Deduplication_FullyFailedBroadcastAlsoBlocks(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "failed-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       0,
			FailedCount:     315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-5 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// Even fully-failed broadcasts should block (use force=true to override)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for fully-failed broadcast dedup, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends, got %d", len(sender.calls))
	}
}
