package db

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ErrModerationNoteTarget is returned when a moderation verdict does not target a post.
var ErrModerationNoteTarget = errors.New("moderation verdicts are recorded on posts only")

// ModerationReplyWriter records a moderation verdict as a top-level system reply on the
// moderated post. It satisfies the comment-creator interfaces the moderation paths already
// use (post moderation, post-translation re-moderation, cmd/moderate-existing), so the
// verdict text and author stay the same while the storage moves from the legacy comments
// table to canonical replies.
type ModerationReplyWriter struct {
	replies *ReplyRepository
}

// NewModerationReplyWriter creates a ModerationReplyWriter.
func NewModerationReplyWriter(pool *Pool) *ModerationReplyWriter {
	return &ModerationReplyWriter{replies: NewReplyRepository(pool)}
}

// Create writes the verdict as a reply and returns it with the reply's identity. Only
// post-targeted verdicts exist; any other target is refused rather than guessed.
func (w *ModerationReplyWriter) Create(ctx context.Context, verdict *models.Comment) (*models.Comment, error) {
	if verdict.TargetType != models.CommentTargetPost {
		return nil, ErrModerationNoteTarget
	}
	reply, err := w.replies.Create(ctx, &models.Reply{
		PostID:     verdict.TargetID,
		AuthorType: verdict.AuthorType,
		AuthorID:   verdict.AuthorID,
		Body:       verdict.Content,
	})
	if err != nil {
		return nil, err
	}
	written := *verdict
	written.ID = reply.ID
	written.CreatedAt = reply.CreatedAt
	return &written, nil
}
