package models

import "testing"

func TestIsValidPostStatus_PendingReview(t *testing.T) {
	// Content moderation places posts in pending_review before they go live.
	if !IsValidPostStatus(PostStatusPendingReview) {
		t.Error("expected pending_review to be valid")
	}
	if PostStatusPendingReview != "pending_review" {
		t.Fatalf("expected PostStatusPendingReview to be 'pending_review', got %q", PostStatusPendingReview)
	}
}

func TestIsValidPostStatus_Rejected(t *testing.T) {
	// Content moderation sets rejected when content violates guidelines.
	if !IsValidPostStatus(PostStatusRejected) {
		t.Error("expected rejected to be valid")
	}
	if PostStatusRejected != "rejected" {
		t.Fatalf("expected PostStatusRejected to be 'rejected', got %q", PostStatusRejected)
	}
}

func TestAuthorTypeSystem(t *testing.T) {
	// "system" should be a valid author type for automated system comments
	// (e.g., moderation results posted as comments).
	if AuthorTypeSystem != "system" {
		t.Fatalf("expected AuthorTypeSystem to be 'system', got %q", AuthorTypeSystem)
	}
}

// The post statuses are exactly draft, open, closed, stale, pending_review and rejected; the
// legacy per-type statuses retired in idx 68 are invalid and recognized as retired.
func TestIsValidPostStatus_OnlyThePostStatuses(t *testing.T) {
	for _, s := range []PostStatus{PostStatusDraft, PostStatusOpen, PostStatusClosed, PostStatusStale,
		PostStatusPendingReview, PostStatusRejected} {
		if !IsValidPostStatus(s) {
			t.Errorf("IsValidPostStatus(%q) = false, want true", s)
		}
		if IsRetiredPostStatus(s) {
			t.Errorf("IsRetiredPostStatus(%q) = true, want false", s)
		}
	}
	for _, s := range []PostStatus{"in_progress", "solved", "answered", "active", "dormant", "evolved"} {
		if IsValidPostStatus(s) {
			t.Errorf("IsValidPostStatus(%q) = true, want false (retired)", s)
		}
		if !IsRetiredPostStatus(s) {
			t.Errorf("IsRetiredPostStatus(%q) = false, want true", s)
		}
	}
	if IsValidPostStatus("unknown") || IsRetiredPostStatus("unknown") {
		t.Error("an unknown status is neither valid nor retired")
	}
}
