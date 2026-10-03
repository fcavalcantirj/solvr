package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task idx 76 step 3 (feature:crystallization): the canonical crystallizer snapshots a
// public, published, approved post of any type together with its live human and agent
// replies. There is no problem type, solved status or succeeded approach to require.

type fakeSnapshotPostFinder struct {
	post *models.PostWithAuthor
	err  error
}

func (f *fakeSnapshotPostFinder) FindSnapshotPost(_ context.Context, _ string) (*models.PostWithAuthor, error) {
	return f.post, f.err
}

// fakeSnapshotReplyLister pages its replies the way ReplyRepository.ListByPost does.
type fakeSnapshotReplyLister struct {
	replies []models.ReplyWithAuthor
	err     error
	calls   []models.ReplyListOptions
}

func (f *fakeSnapshotReplyLister) ListByPost(_ context.Context, opts models.ReplyListOptions) ([]models.ReplyWithAuthor, int, error) {
	f.calls = append(f.calls, opts)
	if f.err != nil {
		return nil, 0, f.err
	}
	start := (opts.Page - 1) * opts.PerPage
	if start >= len(f.replies) {
		return []models.ReplyWithAuthor{}, len(f.replies), nil
	}
	end := min(start+opts.PerPage, len(f.replies))
	return f.replies[start:end], len(f.replies), nil
}

func stablePost(age time.Duration) *models.PostWithAuthor {
	at := time.Now().Add(-age)
	return &models.PostWithAuthor{
		Post: models.Post{
			ID: "post-uuid-1", Type: models.PostTypePost, Title: "Share one room across agents",
			Description: "How should independent agents coordinate?", Tags: []string{"rooms", "agents"},
			PostedByType: models.AuthorTypeHuman, PostedByID: "user-1", Upvotes: 5, Downvotes: 1,
			Visibility: models.VisibilityPublic, PublicationState: models.PublicationPublished,
			ModerationState: models.ModerationApproved, CreatedAt: at.Add(-time.Hour), UpdatedAt: at,
		},
		Author: models.PostAuthor{Type: models.AuthorTypeHuman, ID: "user-1", DisplayName: "Alice"},
	}
}

func snapshotReply(id string, authorType models.AuthorType, age time.Duration) models.ReplyWithAuthor {
	at := time.Now().Add(-age)
	return models.ReplyWithAuthor{
		Reply: models.Reply{
			ID: id, PostID: "post-uuid-1", AuthorType: authorType, AuthorID: "author-" + id,
			Body: "body of " + id, Upvotes: 2, CreatedAt: at, UpdatedAt: at,
		},
		Author: models.ReplyAuthor{ID: "author-" + id, Type: authorType, DisplayName: "Name " + id},
	}
}

type postCrystallizationFixture struct {
	posts   *fakeSnapshotPostFinder
	replies *fakeSnapshotReplyLister
	cids    *mockCrystallizationCIDSetter
	adder   *mockIPFSAdder
	pinner  *mockIPFSPinner
	svc     *PostCrystallizationService
}

func newPostCrystallizationFixture(post *models.PostWithAuthor, replies ...models.ReplyWithAuthor) *postCrystallizationFixture {
	f := &postCrystallizationFixture{
		posts:   &fakeSnapshotPostFinder{post: post},
		replies: &fakeSnapshotReplyLister{replies: replies},
		cids:    &mockCrystallizationCIDSetter{},
		adder:   &mockIPFSAdder{cid: "bafypost"},
		pinner:  &mockIPFSPinner{},
	}
	f.svc = NewPostCrystallizationService(f.posts, f.cids, f.replies, f.adder, f.pinner)
	return f
}

const tenDays = 10 * 24 * time.Hour

func TestCrystallizePost_SnapshotsThePostAndItsReplies(t *testing.T) {
	parent := "r1"
	child := snapshotReply("r2", models.AuthorTypeHuman, 11*24*time.Hour)
	child.ParentReplyID = &parent
	legacy := "approach"
	child.LegacyType = &legacy
	f := newPostCrystallizationFixture(stablePost(tenDays),
		snapshotReply("r1", models.AuthorTypeAgent, 12*24*time.Hour),
		snapshotReply("verdict", models.AuthorTypeSystem, tenDays),
		child,
	)

	cid, err := f.svc.CrystallizePost(context.Background(), "post-uuid-1")
	if err != nil {
		t.Fatalf("CrystallizePost() error = %v", err)
	}
	if cid != "bafypost" {
		t.Errorf("cid = %q, want bafypost", cid)
	}
	if f.cids.calledWith.postID != "post-uuid-1" || f.cids.calledWith.cid != "bafypost" {
		t.Errorf("SetCrystallizationCID called with %+v", f.cids.calledWith)
	}
	if len(f.pinner.pinnedCIDs) != 1 || f.pinner.pinnedCIDs[0] != "bafypost" {
		t.Errorf("pinned %v, want [bafypost]", f.pinner.pinnedCIDs)
	}

	var snap PostSnapshot
	if err := json.Unmarshal(f.adder.content, &snap); err != nil {
		t.Fatalf("snapshot is not JSON: %v\n%s", err, f.adder.content)
	}
	if snap.Version != PostCrystallizationVersion || snap.Origin != "solvr.dev" || snap.PostID != "post-uuid-1" {
		t.Errorf("header = %q %q %q", snap.Version, snap.Origin, snap.PostID)
	}
	if snap.CrystallizedAt.IsZero() {
		t.Error("crystallized_at is empty")
	}
	p := snap.Post
	if p.Title != "Share one room across agents" || p.Body != "How should independent agents coordinate?" ||
		len(p.Tags) != 2 || p.Upvotes != 5 || p.Downvotes != 1 ||
		p.Author != (SnapshotAuthor{Type: "human", ID: "user-1", DisplayName: "Alice"}) {
		t.Errorf("post = %+v", p)
	}
	if len(snap.Replies) != 2 {
		t.Fatalf("replies = %+v, want r1 and r2 without the system verdict", snap.Replies)
	}
	r1, r2 := snap.Replies[0], snap.Replies[1]
	if r1.ID != "r1" || r1.Body != "body of r1" || r1.Upvotes != 2 || r1.ParentReplyID != nil ||
		r1.Author != (SnapshotAuthor{Type: "agent", ID: "author-r1", DisplayName: "Name r1"}) {
		t.Errorf("reply 1 = %+v", r1)
	}
	if r2.ID != "r2" || r2.ParentReplyID == nil || *r2.ParentReplyID != "r1" || r2.LegacyType == nil || *r2.LegacyType != "approach" {
		t.Errorf("reply 2 keeps its thread parent and legacy origin: %+v", r2)
	}

	var raw map[string]any
	_ = json.Unmarshal(f.adder.content, &raw)
	for _, legacyKey := range []string{"problem", "problem_id", "approaches"} {
		if _, ok := raw[legacyKey]; ok {
			t.Errorf("canonical snapshot carries the legacy key %q", legacyKey)
		}
	}
}

func TestCrystallizePost_ReadsEveryReplyPage(t *testing.T) {
	var replies []models.ReplyWithAuthor
	for i := range 230 {
		replies = append(replies, snapshotReply(fmt.Sprintf("r%03d", i), models.AuthorTypeAgent, 20*24*time.Hour))
	}
	f := newPostCrystallizationFixture(stablePost(tenDays), replies...)
	if _, err := f.svc.CrystallizePost(context.Background(), "post-uuid-1"); err != nil {
		t.Fatalf("CrystallizePost() error = %v", err)
	}
	var snap PostSnapshot
	_ = json.Unmarshal(f.adder.content, &snap)
	if len(snap.Replies) != 230 || snap.Replies[229].ID != "r229" {
		t.Fatalf("snapshot holds %d replies, want all 230 in order", len(snap.Replies))
	}
	for i, c := range f.replies.calls {
		if c.PostID != "post-uuid-1" || c.Page != i+1 {
			t.Errorf("call %d = %+v", i, c)
		}
	}
}

func TestCrystallizePost_RefusesPostsThatAreNotPubliclyEligible(t *testing.T) {
	deletedAt := time.Now().Add(-tenDays)
	for name, mut := range map[string]func(p *models.PostWithAuthor){
		"family":   func(p *models.PostWithAuthor) { p.Visibility = models.VisibilityFamily },
		"draft":    func(p *models.PostWithAuthor) { p.PublicationState = models.PublicationDraft },
		"archived": func(p *models.PostWithAuthor) { p.PublicationState = models.PublicationArchived },
		"pending":  func(p *models.PostWithAuthor) { p.ModerationState = models.ModerationPending },
		"rejected": func(p *models.PostWithAuthor) { p.ModerationState = models.ModerationRejected },
		"deleted":  func(p *models.PostWithAuthor) { p.DeletedAt = &deletedAt },
	} {
		t.Run(name, func(t *testing.T) {
			post := stablePost(tenDays)
			mut(post)
			f := newPostCrystallizationFixture(post, snapshotReply("r1", models.AuthorTypeAgent, tenDays))
			_, err := f.svc.CrystallizePost(context.Background(), "post-uuid-1")
			if !errors.Is(err, ErrNotPubliclyEligible) {
				t.Fatalf("error = %v, want ErrNotPubliclyEligible", err)
			}
			if f.adder.content != nil || f.cids.calledWith.cid != "" {
				t.Error("an ineligible post reached IPFS or the CID setter")
			}
		})
	}
}

func TestCrystallizePost_RefusesCrystallizedOrUnstableContent(t *testing.T) {
	cid := "bafyold"
	crystallized := stablePost(tenDays)
	crystallized.CrystallizationCID = &cid
	cases := map[string]struct {
		post    *models.PostWithAuthor
		replies []models.ReplyWithAuthor
		want    error
	}{
		"already crystallized": {crystallized, []models.ReplyWithAuthor{snapshotReply("r1", models.AuthorTypeAgent, tenDays)}, ErrAlreadyCrystallized},
		"post edited recently": {stablePost(24 * time.Hour), []models.ReplyWithAuthor{snapshotReply("r1", models.AuthorTypeAgent, tenDays)}, ErrNotStableYet},
		"reply edited recently": {stablePost(tenDays), []models.ReplyWithAuthor{
			snapshotReply("r1", models.AuthorTypeAgent, tenDays), snapshotReply("r2", models.AuthorTypeHuman, time.Hour)}, ErrNotStableYet},
		"no replies": {stablePost(tenDays), nil, ErrNothingToCrystallize},
		"only a system verdict": {stablePost(tenDays), []models.ReplyWithAuthor{
			snapshotReply("verdict", models.AuthorTypeSystem, time.Hour)}, ErrNothingToCrystallize},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPostCrystallizationFixture(c.post, c.replies...)
			_, err := f.svc.CrystallizePost(context.Background(), "post-uuid-1")
			if !errors.Is(err, c.want) {
				t.Fatalf("error = %v, want %v", err, c.want)
			}
			if f.adder.content != nil || f.cids.calledWith.cid != "" {
				t.Error("refused content reached IPFS or the CID setter")
			}
		})
	}
}

func TestCrystallizePost_CustomStabilityPeriod(t *testing.T) {
	f := newPostCrystallizationFixture(stablePost(2*time.Hour), snapshotReply("r1", models.AuthorTypeAgent, 3*time.Hour))
	f.svc = NewPostCrystallizationServiceWithConfig(f.posts, f.cids, f.replies, f.adder, f.pinner,
		CrystallizationConfig{StabilityPeriod: time.Hour})
	if _, err := f.svc.CrystallizePost(context.Background(), "post-uuid-1"); err != nil {
		t.Fatalf("CrystallizePost() with a 1h period error = %v", err)
	}
}

func TestCrystallizePost_Failures(t *testing.T) {
	boom := errors.New("boom")
	reply := snapshotReply("r1", models.AuthorTypeAgent, tenDays)

	f := newPostCrystallizationFixture(nil)
	f.posts.err = boom
	if _, err := f.svc.CrystallizePost(context.Background(), "x"); !errors.Is(err, boom) {
		t.Errorf("finder error = %v, want it wrapped", err)
	}

	f = newPostCrystallizationFixture(stablePost(tenDays), reply)
	f.replies.err = boom
	if _, err := f.svc.CrystallizePost(context.Background(), "x"); !errors.Is(err, boom) || f.adder.content != nil {
		t.Errorf("reply lister error = %v (IPFS content %d bytes)", err, len(f.adder.content))
	}

	f = newPostCrystallizationFixture(stablePost(tenDays), reply)
	f.adder.err = boom
	if _, err := f.svc.CrystallizePost(context.Background(), "x"); !errors.Is(err, boom) || f.cids.calledWith.cid != "" {
		t.Errorf("IPFS add error = %v, CID set to %q", err, f.cids.calledWith.cid)
	}

	f = newPostCrystallizationFixture(stablePost(tenDays), reply)
	f.pinner.err = boom
	if cid, err := f.svc.CrystallizePost(context.Background(), "x"); err != nil || f.cids.calledWith.cid != "bafypost" {
		t.Errorf("a pin failure must not lose the CID: cid %q err %v saved %q", cid, err, f.cids.calledWith.cid)
	}

	f = newPostCrystallizationFixture(stablePost(tenDays), reply)
	f.cids.err = boom
	if _, err := f.svc.CrystallizePost(context.Background(), "x"); !errors.Is(err, boom) {
		t.Errorf("CID setter error = %v, want it wrapped", err)
	}
}

// An ineligible post is refused, not skipped: the job counts ErrNotPubliclyEligible as a
// failure and only ErrNothingToCrystallize as a skip.
func TestErrNotPubliclyEligible_IsNotNothingToCrystallize(t *testing.T) {
	if errors.Is(ErrNotPubliclyEligible, ErrNothingToCrystallize) {
		t.Fatal("an ineligible post is not a skip-worthy empty post")
	}
}
