package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Task idx 76 step 3 (feature:duplicate-detection): with a ContentDuplicateFinder the
// moderation service detects duplicates on the canonical targets — a post against earlier
// posts, a reply against earlier replies on the same post — and flags the new one.

type duplicateLookup struct {
	postID, title, text, excludeID string
	since                          time.Time
}

type fakeDuplicateFinder struct {
	post, reply           *models.ContentDuplicate
	err                   error
	postCalls, replyCalls []duplicateLookup
}

func (f *fakeDuplicateFinder) FindPost(_ context.Context, title, description string, since time.Time, excludeID string) (*models.ContentDuplicate, error) {
	f.postCalls = append(f.postCalls, duplicateLookup{title: title, text: description, since: since, excludeID: excludeID})
	return f.post, f.err
}

func (f *fakeDuplicateFinder) FindReply(_ context.Context, postID, body string, since time.Time, excludeID string) (*models.ContentDuplicate, error) {
	f.replyCalls = append(f.replyCalls, duplicateLookup{postID: postID, text: body, since: since, excludeID: excludeID})
	return f.reply, f.err
}

func assertDuplicateWindow(t *testing.T, since time.Time) {
	t.Helper()
	want := time.Now().Add(-DefaultDuplicateMaxAge)
	if since.Before(want.Add(-time.Minute)) || since.After(want.Add(time.Minute)) {
		t.Errorf("lookup window starts at %v, want about %v", since, want)
	}
}

func TestModerationService_AutoFlag_DuplicatePostFromCanonicalPosts(t *testing.T) {
	flags := &MockFlagCreator{}
	original := uuid.New()
	finder := &fakeDuplicateFinder{post: &models.ContentDuplicate{
		TargetType: "post", TargetID: original.String(), PostID: original.String(),
	}}
	svc := NewModerationService(flags, nil, nil)
	svc.SetDuplicateFinder(finder)

	newPost := uuid.New()
	content := ModerationContent{
		Title:       "How to test Go code",
		Description: "I want to learn about testing in Go applications. What is the best approach?",
	}
	if err := svc.AutoFlagIfNeeded(context.Background(), newPost, "post", content); err != nil {
		t.Fatalf("AutoFlagIfNeeded: %v", err)
	}

	if len(finder.postCalls) != 1 || len(finder.replyCalls) != 0 {
		t.Fatalf("want 1 post lookup and 0 reply lookups, got %d and %d", len(finder.postCalls), len(finder.replyCalls))
	}
	call := finder.postCalls[0]
	if call.title != content.Title || call.text != content.Description || call.excludeID != newPost.String() {
		t.Errorf("post lookup %+v does not carry the post's content and exclude the post itself", call)
	}
	assertDuplicateWindow(t, call.since)

	if len(flags.CreatedFlags) != 1 {
		t.Fatalf("want 1 duplicate flag, got %d", len(flags.CreatedFlags))
	}
	flag := flags.CreatedFlags[0]
	if flag.TargetType != "post" || flag.TargetID != newPost || flag.Reason != "duplicate" ||
		flag.Details != "duplicate of post "+original.String() || flag.ReporterType != "system" {
		t.Errorf("unexpected flag %+v", flag)
	}
}

func TestModerationService_AutoFlag_DuplicateReplyOnTheSamePost(t *testing.T) {
	flags := &MockFlagCreator{}
	postID, original := uuid.New(), uuid.New()
	finder := &fakeDuplicateFinder{reply: &models.ContentDuplicate{
		TargetType: "reply", TargetID: original.String(), PostID: postID.String(),
	}}
	svc := NewModerationService(flags, nil, nil)
	svc.SetDuplicateFinder(finder)

	newReply := uuid.New()
	content := ModerationContent{Description: "Same fix, retried.", PostID: postID.String()}
	if err := svc.AutoFlagIfNeeded(context.Background(), newReply, "reply", content); err != nil {
		t.Fatalf("AutoFlagIfNeeded: %v", err)
	}

	if len(finder.replyCalls) != 1 || len(finder.postCalls) != 0 {
		t.Fatalf("want 1 reply lookup and 0 post lookups, got %d and %d", len(finder.replyCalls), len(finder.postCalls))
	}
	call := finder.replyCalls[0]
	if call.postID != postID.String() || call.text != content.Description || call.excludeID != newReply.String() {
		t.Errorf("reply lookup %+v does not carry the reply's post and body and exclude the reply itself", call)
	}
	assertDuplicateWindow(t, call.since)

	if len(flags.CreatedFlags) != 1 {
		t.Fatalf("want exactly the duplicate flag, got %d flags", len(flags.CreatedFlags))
	}
	flag := flags.CreatedFlags[0]
	want := "duplicate of reply " + original.String() + " on post " + postID.String()
	if flag.TargetType != "reply" || flag.TargetID != newReply || flag.Reason != "duplicate" || flag.Details != want {
		t.Errorf("unexpected flag %+v, want details %q", flag, want)
	}
}

// A reply has no title: the post-shaped spam rules (title length and caps, minimum
// description) do not apply to it, so a short unique reply raises no flag.
func TestModerationService_AutoFlag_UniqueReplyIsNotFlagged(t *testing.T) {
	flags := &MockFlagCreator{}
	finder := &fakeDuplicateFinder{}
	svc := NewModerationService(flags, nil, nil)
	svc.SetDuplicateFinder(finder)

	content := ModerationContent{Description: "Thanks, that worked.", PostID: uuid.NewString()}
	if err := svc.AutoFlagIfNeeded(context.Background(), uuid.New(), "reply", content); err != nil {
		t.Fatalf("AutoFlagIfNeeded: %v", err)
	}
	if len(finder.replyCalls) != 1 {
		t.Errorf("want the reply looked up once, got %d", len(finder.replyCalls))
	}
	if len(flags.CreatedFlags) != 0 {
		t.Errorf("want no flag for a unique reply, got %+v", flags.CreatedFlags[0])
	}
}

func TestModerationService_CheckContentDuplicate_ReturnsTheOriginal(t *testing.T) {
	postID, original := uuid.New(), uuid.New()
	svc := NewModerationService(nil, nil, nil)
	svc.SetDuplicateFinder(&fakeDuplicateFinder{reply: &models.ContentDuplicate{
		TargetType: "reply", TargetID: original.String(), PostID: postID.String(),
	}})

	got, err := svc.CheckContentDuplicate(context.Background(), uuid.New(), "reply",
		ModerationContent{Description: "body", PostID: postID.String()})
	if err != nil {
		t.Fatalf("CheckContentDuplicate: %v", err)
	}
	if !got.IsDuplicate || got.OriginalTargetType != "reply" || got.OriginalTargetID != original.String() ||
		got.OriginalPostID != postID || got.Similarity != 1.0 {
		t.Errorf("unexpected result %+v", got)
	}
}

// Legacy contribution target types are not canonical: they are never looked up.
func TestModerationService_CheckContentDuplicate_SkipsLegacyTargetTypes(t *testing.T) {
	finder := &fakeDuplicateFinder{post: &models.ContentDuplicate{TargetType: "post", TargetID: uuid.NewString(), PostID: uuid.NewString()}}
	svc := NewModerationService(nil, nil, nil)
	svc.SetDuplicateFinder(finder)

	for _, legacy := range []string{"answer", "approach", "response", "comment"} {
		got, err := svc.CheckContentDuplicate(context.Background(), uuid.New(), legacy, ModerationContent{Description: "body"})
		if err != nil {
			t.Fatalf("%s: %v", legacy, err)
		}
		if got.IsDuplicate {
			t.Errorf("%s: a legacy target must not be reported as a duplicate", legacy)
		}
	}
	if len(finder.postCalls)+len(finder.replyCalls) != 0 {
		t.Errorf("legacy targets must not be looked up, got %d lookups", len(finder.postCalls)+len(finder.replyCalls))
	}
}

func TestModerationService_AutoFlag_DuplicateFinderErrorIsReturned(t *testing.T) {
	flags := &MockFlagCreator{}
	boom := errors.New("database unavailable")
	svc := NewModerationService(flags, nil, nil)
	svc.SetDuplicateFinder(&fakeDuplicateFinder{err: boom})

	err := svc.AutoFlagIfNeeded(context.Background(), uuid.New(), "reply",
		ModerationContent{Description: "body", PostID: uuid.NewString()})
	if !errors.Is(err, boom) {
		t.Fatalf("want the finder error, got %v", err)
	}
	if len(flags.CreatedFlags) != 0 {
		t.Errorf("no flag may be created when the lookup failed")
	}
}
