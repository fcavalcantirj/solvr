package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// PostCrystallizationVersion is the format version of canonical post snapshots. Version
// 1.0 snapshots (problem + approaches) stay valid on IPFS; new ones use this format.
const PostCrystallizationVersion = "2.0"

// snapshotReplyPageSize is the reply page size the crystallizer reads with (the reply
// repository's maximum).
const snapshotReplyPageSize = 100

// Canonical crystallization errors.
var (
	// ErrNothingToCrystallize: the post has no live human or agent reply to snapshot. The
	// job skips such a post instead of counting a failure.
	ErrNothingToCrystallize = errors.New("nothing to crystallize")
	// ErrNotPubliclyEligible: only public, published, approved posts reach public IPFS.
	ErrNotPubliclyEligible = errors.New("only public, published, approved posts can be crystallized")
)

// SnapshotPostFinder reads a post, its author and its stored canonical states.
type SnapshotPostFinder interface {
	FindSnapshotPost(ctx context.Context, postID string) (*models.PostWithAuthor, error)
}

// SnapshotReplyLister pages a post's live replies, oldest first.
type SnapshotReplyLister interface {
	ListByPost(ctx context.Context, opts models.ReplyListOptions) ([]models.ReplyWithAuthor, int, error)
}

// PostCrystallizationService snapshots a canonical post and its replies to IPFS (idx 76,
// feature:crystallization). Any post type qualifies; there is no solved status or
// succeeded approach to require.
type PostCrystallizationService struct {
	posts      SnapshotPostFinder
	cidSetter  CrystallizationCIDSetter
	replies    SnapshotReplyLister
	ipfsAdder  IPFSContentAdder
	ipfsPinner IPFSContentPinner
	config     CrystallizationConfig
}

// NewPostCrystallizationService creates a PostCrystallizationService with default config.
func NewPostCrystallizationService(
	posts SnapshotPostFinder,
	cidSetter CrystallizationCIDSetter,
	replies SnapshotReplyLister,
	ipfsAdder IPFSContentAdder,
	ipfsPinner IPFSContentPinner,
) *PostCrystallizationService {
	return NewPostCrystallizationServiceWithConfig(posts, cidSetter, replies, ipfsAdder, ipfsPinner,
		DefaultCrystallizationConfig())
}

// NewPostCrystallizationServiceWithConfig creates a PostCrystallizationService with custom config.
func NewPostCrystallizationServiceWithConfig(
	posts SnapshotPostFinder,
	cidSetter CrystallizationCIDSetter,
	replies SnapshotReplyLister,
	ipfsAdder IPFSContentAdder,
	ipfsPinner IPFSContentPinner,
	config CrystallizationConfig,
) *PostCrystallizationService {
	return &PostCrystallizationService{
		posts:      posts,
		cidSetter:  cidSetter,
		replies:    replies,
		ipfsAdder:  ipfsAdder,
		ipfsPinner: ipfsPinner,
		config:     config,
	}
}

// CrystallizePost snapshots a publicly eligible, stable post with its live human and
// agent replies to IPFS and records the CID. System replies (moderation verdicts) are
// left out of the snapshot.
func (s *PostCrystallizationService) CrystallizePost(ctx context.Context, postID string) (string, error) {
	post, err := s.posts.FindSnapshotPost(ctx, postID)
	if err != nil {
		return "", fmt.Errorf("crystallize: find post: %w", err)
	}
	if !post.PublicEligible() || post.Visibility != models.VisibilityPublic {
		return "", ErrNotPubliclyEligible
	}
	if post.CrystallizationCID != nil {
		return "", ErrAlreadyCrystallized
	}
	if time.Since(post.UpdatedAt) < s.config.StabilityPeriod {
		return "", ErrNotStableYet
	}

	replies, err := s.snapshotReplies(ctx, postID)
	if err != nil {
		return "", err
	}
	for _, r := range replies {
		if time.Since(r.UpdatedAt) < s.config.StabilityPeriod {
			return "", ErrNotStableYet
		}
	}
	if len(replies) == 0 {
		return "", ErrNothingToCrystallize
	}

	data, err := json.MarshalIndent(s.BuildPostSnapshot(post, replies), "", "  ")
	if err != nil {
		return "", fmt.Errorf("crystallize: marshal snapshot: %w", err)
	}
	cid, err := s.ipfsAdder.Add(ctx, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("crystallize: IPFS add: %w", err)
	}
	// Non-fatal: the content is already on IPFS.
	if pinErr := s.ipfsPinner.Pin(ctx, cid); pinErr != nil {
		slog.Warn("crystallize: pin failed (non-fatal)", "cid", cid, "error", pinErr)
	}
	if err := s.cidSetter.SetCrystallizationCID(ctx, postID, cid); err != nil {
		return "", fmt.Errorf("crystallize: save CID: %w", err)
	}
	slog.Info("post crystallized", "post_id", postID, "cid", cid, "replies", len(replies))
	return cid, nil
}

// snapshotReplies reads every page of the post's live replies and keeps the human and
// agent ones.
func (s *PostCrystallizationService) snapshotReplies(ctx context.Context, postID string) ([]models.ReplyWithAuthor, error) {
	var kept []models.ReplyWithAuthor
	seen := 0
	for page := 1; ; page++ {
		batch, total, err := s.replies.ListByPost(ctx, models.ReplyListOptions{
			PostID: postID, Page: page, PerPage: snapshotReplyPageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("crystallize: list replies: %w", err)
		}
		for _, r := range batch {
			if r.AuthorType != models.AuthorTypeSystem {
				kept = append(kept, r)
			}
		}
		seen += len(batch)
		if len(batch) < snapshotReplyPageSize || seen >= total {
			return kept, nil
		}
	}
}

// PostSnapshot is the immutable canonical document stored on IPFS.
type PostSnapshot struct {
	Version        string          `json:"version"`
	Origin         string          `json:"origin"`
	PostID         string          `json:"post_id"`
	CrystallizedAt time.Time       `json:"crystallized_at"`
	Post           SnapshotPost    `json:"post"`
	Replies        []SnapshotReply `json:"replies"`
}

// SnapshotPost is the post within a canonical snapshot.
type SnapshotPost struct {
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Tags      []string       `json:"tags,omitempty"`
	Upvotes   int            `json:"upvotes"`
	Downvotes int            `json:"downvotes"`
	Author    SnapshotAuthor `json:"author"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// SnapshotReply is one reply within a canonical snapshot. LegacyType names the legacy
// contribution a migrated reply came from (approach, answer, ...).
type SnapshotReply struct {
	ID            string         `json:"id"`
	ParentReplyID *string        `json:"parent_reply_id,omitempty"`
	Body          string         `json:"body"`
	Upvotes       int            `json:"upvotes"`
	Downvotes     int            `json:"downvotes"`
	LegacyType    *string        `json:"legacy_type,omitempty"`
	Author        SnapshotAuthor `json:"author"`
	CreatedAt     time.Time      `json:"created_at"`
}

// BuildPostSnapshot creates the canonical snapshot of a post and its replies.
func (s *PostCrystallizationService) BuildPostSnapshot(post *models.PostWithAuthor, replies []models.ReplyWithAuthor) PostSnapshot {
	out := make([]SnapshotReply, 0, len(replies))
	for _, r := range replies {
		out = append(out, SnapshotReply{
			ID:            r.ID,
			ParentReplyID: r.ParentReplyID,
			Body:          r.Body,
			Upvotes:       r.Upvotes,
			Downvotes:     r.Downvotes,
			LegacyType:    r.LegacyType,
			Author:        SnapshotAuthor{Type: string(r.Author.Type), ID: r.Author.ID, DisplayName: r.Author.DisplayName},
			CreatedAt:     r.CreatedAt,
		})
	}
	return PostSnapshot{
		Version:        PostCrystallizationVersion,
		Origin:         "solvr.dev",
		PostID:         post.ID,
		CrystallizedAt: time.Now(),
		Post: SnapshotPost{
			Title:     post.Title,
			Body:      post.Description,
			Tags:      post.Tags,
			Upvotes:   post.Upvotes,
			Downvotes: post.Downvotes,
			Author:    SnapshotAuthor{Type: string(post.Author.Type), ID: post.Author.ID, DisplayName: post.Author.DisplayName},
			CreatedAt: post.CreatedAt,
			UpdatedAt: post.UpdatedAt,
		},
		Replies: out,
	}
}
