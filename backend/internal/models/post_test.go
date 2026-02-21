package models

import "testing"

func TestIsValidPostStatus_PendingReview(t *testing.T) {
	// "pending_review" should be valid for ALL post types (problem, question, idea).
	// Content moderation places posts in pending_review before they go live.
	types := []PostType{PostTypeProblem, PostTypeQuestion, PostTypeIdea}
	for _, pt := range types {
		if !IsValidPostStatus(PostStatusPendingReview, pt) {
			t.Errorf("expected pending_review to be valid for %s", pt)
		}
	}

	// Verify the constant has the correct string value.
	if PostStatusPendingReview != "pending_review" {
		t.Fatalf("expected PostStatusPendingReview to be 'pending_review', got %q", PostStatusPendingReview)
	}
}

func TestIsValidPostStatus_Rejected(t *testing.T) {
	// "rejected" should be valid for ALL post types (problem, question, idea).
	// Content moderation sets rejected when content violates guidelines.
	types := []PostType{PostTypeProblem, PostTypeQuestion, PostTypeIdea}
	for _, pt := range types {
		if !IsValidPostStatus(PostStatusRejected, pt) {
			t.Errorf("expected rejected to be valid for %s", pt)
		}
	}

	// Verify the constant has the correct string value.
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

func TestIsValidPostStatus_ExistingStatusesStillValid(t *testing.T) {
	// Regression: ensure existing statuses remain valid after adding new ones.
	tests := []struct {
		status   PostStatus
		postType PostType
		want     bool
	}{
		{PostStatusDraft, PostTypeProblem, true},
		{PostStatusOpen, PostTypeProblem, true},
		{PostStatusInProgress, PostTypeProblem, true},
		{PostStatusSolved, PostTypeProblem, true},
		{PostStatusClosed, PostTypeProblem, true},
		{PostStatusStale, PostTypeProblem, true},

		{PostStatusDraft, PostTypeQuestion, true},
		{PostStatusOpen, PostTypeQuestion, true},
		{PostStatusAnswered, PostTypeQuestion, true},
		{PostStatusClosed, PostTypeQuestion, true},
		{PostStatusStale, PostTypeQuestion, true},

		{PostStatusDraft, PostTypeIdea, true},
		{PostStatusOpen, PostTypeIdea, true},
		{PostStatusActive, PostTypeIdea, true},
		{PostStatusDormant, PostTypeIdea, true},
		{PostStatusEvolved, PostTypeIdea, true},

		// Invalid cross-type statuses should still be invalid.
		{PostStatusSolved, PostTypeQuestion, false},
		{PostStatusAnswered, PostTypeProblem, false},
		{PostStatusActive, PostTypeProblem, false},
		{PostStatusEvolved, PostTypeQuestion, false},
	}
	for _, tt := range tests {
		got := IsValidPostStatus(tt.status, tt.postType)
		if got != tt.want {
			t.Errorf("IsValidPostStatus(%q, %q) = %v, want %v", tt.status, tt.postType, got, tt.want)
		}
	}
}
