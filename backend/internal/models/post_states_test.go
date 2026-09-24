package models

import (
	"testing"
	"time"
)

// TestDeriveStates pins the legacy-status -> (publication, moderation) mapping that
// migration 000088's backfill, Create, Update, and UpdateStatus all rely on (BART-583).
func TestDeriveStates(t *testing.T) {
	cases := []struct {
		status  PostStatus
		wantPub PublicationState
		wantMod ModerationState
	}{
		{PostStatusDraft, PublicationDraft, ModerationPending},
		{PostStatusPendingReview, PublicationDraft, ModerationPending},
		{PostStatusRejected, PublicationDraft, ModerationRejected},
		{PostStatusClosed, PublicationArchived, ModerationApproved},
		{PostStatusOpen, PublicationPublished, ModerationApproved},
		{PostStatusInProgress, PublicationPublished, ModerationApproved},
		{PostStatusSolved, PublicationPublished, ModerationApproved},
		{PostStatusAnswered, PublicationPublished, ModerationApproved},
		{PostStatusActive, PublicationPublished, ModerationApproved},
		{PostStatusDormant, PublicationPublished, ModerationApproved},
		{PostStatusEvolved, PublicationPublished, ModerationApproved},
		{PostStatusStale, PublicationPublished, ModerationApproved},
	}
	for _, c := range cases {
		pub, mod := DeriveStates(c.status)
		if pub != c.wantPub || mod != c.wantMod {
			t.Errorf("DeriveStates(%q) = (%q,%q), want (%q,%q)", c.status, pub, mod, c.wantPub, c.wantMod)
		}
	}
}

// TestPublicEligible pins the canonical public-eligibility rule: a post is publicly
// visible only when published AND moderation-approved AND publicly visible. Publishing
// alone never bypasses moderation (BART-583, step 2).
func TestPublicEligible(t *testing.T) {
	cases := []struct {
		name string
		post Post
		want bool
	}{
		{"published+approved+public", Post{PublicationState: PublicationPublished, ModerationState: ModerationApproved, Visibility: VisibilityPublic}, true},
		{"published+approved+empty-visibility", Post{PublicationState: PublicationPublished, ModerationState: ModerationApproved}, true},
		{"published+pending (awaiting moderation)", Post{PublicationState: PublicationPublished, ModerationState: ModerationPending, Visibility: VisibilityPublic}, false},
		{"draft+approved (not published)", Post{PublicationState: PublicationDraft, ModerationState: ModerationApproved, Visibility: VisibilityPublic}, false},
		{"rejected", Post{PublicationState: PublicationPublished, ModerationState: ModerationRejected, Visibility: VisibilityPublic}, false},
		{"family scope", Post{PublicationState: PublicationPublished, ModerationState: ModerationApproved, Visibility: VisibilityFamily}, false},
		{"archived", Post{PublicationState: PublicationArchived, ModerationState: ModerationApproved, Visibility: VisibilityPublic}, false},
	}
	for _, c := range cases {
		if got := c.post.PublicEligible(); got != c.want {
			t.Errorf("%s: PublicEligible() = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestPublicEligible_DeletedNeverPublic ensures a soft-deleted post is never eligible.
func TestPublicEligible_DeletedNeverPublic(t *testing.T) {
	now := time.Now()
	p := Post{PublicationState: PublicationPublished, ModerationState: ModerationApproved, Visibility: VisibilityPublic, DeletedAt: &now}
	if p.PublicEligible() {
		t.Error("deleted post must not be publicly eligible")
	}
}

// TestIsValidPostType_CanonicalPost verifies the canonical untyped post is valid while
// unknown types are still rejected (BART-583, step 3).
func TestIsValidPostType_CanonicalPost(t *testing.T) {
	if !IsValidPostType(PostTypePost) {
		t.Error("PostTypePost must be a valid post type")
	}
	if IsValidPostType(PostType("invalid")) {
		t.Error("unknown type must remain invalid")
	}
	for _, legacy := range []PostType{PostTypeProblem, PostTypeQuestion, PostTypeIdea} {
		if !IsValidPostType(legacy) {
			t.Errorf("legacy type %q must remain valid", legacy)
		}
	}
}

// TestIsValidPostStatus_CanonicalPost verifies the canonical post accepts generic
// lifecycle statuses so status validation does not reject untyped posts.
func TestIsValidPostStatus_CanonicalPost(t *testing.T) {
	for _, s := range []PostStatus{PostStatusDraft, PostStatusOpen, PostStatusClosed, PostStatusStale} {
		if !IsValidPostStatus(s, PostTypePost) {
			t.Errorf("status %q must be valid for a canonical post", s)
		}
	}
	// Moderation statuses are valid for any valid type.
	if !IsValidPostStatus(PostStatusPendingReview, PostTypePost) {
		t.Error("pending_review must be valid for a canonical post")
	}
}
