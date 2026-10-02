package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// ContributionModerationStore hides a rejected contribution, names its post and records the
// verdict as a flag (db.ContributionModerationRepository).
type ContributionModerationStore interface {
	Hide(ctx context.Context, kind, id string) error
	ParentPost(ctx context.Context, kind, id string) (postID, postType, title string, err error)
	CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error)
}

// ContributionNotifier writes a notification (the notifications repository's Create).
type ContributionNotifier func(ctx context.Context, n *models.Notification) (*models.Notification, error)

// ContributionModerator runs content moderation on replies and legacy contributions after
// they are created (anti-abuse W2). A rejected reply, answer, approach or comment is hidden
// (soft-deleted, as its author's delete would), flagged moderation_rejected for admins, and its
// author is notified; a rejected response or progress note, which cannot be hidden, is
// flagged and notified only. The contribution is visible until the verdict arrives.
type ContributionModerator struct {
	mod    ContentModerationServiceInterface
	store  ContributionModerationStore
	notify ContributionNotifier
	logger *slog.Logger
}

// NewContributionModerator creates a ContributionModerator.
func NewContributionModerator(mod ContentModerationServiceInterface, store ContributionModerationStore, notify ContributionNotifier) *ContributionModerator {
	return &ContributionModerator{mod: mod, store: store, notify: notify, logger: slog.Default()}
}

// Moderate starts moderation of one new contribution in the background. A nil moderator (no
// GROQ_API_KEY) does nothing.
func (m *ContributionModerator) Moderate(kind, id, body, authorType, authorID string) {
	if m == nil || m.mod == nil || id == "" {
		return
	}
	go m.moderate(kind, id, body, authorType, authorID)
}

func (m *ContributionModerator) moderate(kind, id, body, authorType, authorID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	postID, postType, title, err := m.store.ParentPost(ctx, kind, id)
	if err != nil {
		m.logger.Warn("contribution moderation: parent post lookup failed", "kind", kind, "id", id, "error", err)
	}

	var result *ModerationResult
	for attempt := 0; attempt < 3; attempt++ {
		result, err = m.mod.ModerateContent(ctx, ModerationInput{Title: title, Description: body})
		var limited RateLimitError
		if !errors.As(err, &limited) {
			break
		}
		time.Sleep(limited.GetRetryAfter())
	}
	if err != nil {
		m.logger.Error("contribution moderation failed; left visible", "kind", kind, "id", id, "error", err)
		return
	}
	if result.Approved {
		return
	}

	hidden := true
	if err := m.store.Hide(ctx, kind, id); errors.Is(err, db.ErrNotHideable) {
		hidden = false
	} else if err != nil {
		m.logger.Error("contribution moderation: hide failed", "kind", kind, "id", id, "error", err)
		hidden = false
	}
	if targetID, err := uuid.Parse(id); err == nil {
		if _, err := m.store.CreateFlag(ctx, &models.Flag{
			TargetType: kind, TargetID: targetID, ReporterType: "system", ReporterID: "content-moderation",
			Reason: "moderation_rejected", Details: result.Explanation, Status: "pending",
		}); err != nil {
			m.logger.Error("contribution moderation: flag failed", "kind", kind, "id", id, "error", err)
		}
	}
	m.notifyAuthor(ctx, kind, id, postID, postType, title, authorType, authorID, hidden, result.Explanation)
}

func (m *ContributionModerator) notifyAuthor(ctx context.Context, kind, id, postID, postType, title, authorType, authorID string, hidden bool, explanation string) {
	if m.notify == nil {
		return
	}
	n := &models.Notification{Type: "contribution_removed", Title: "Your " + kind + " was removed",
		Body: fmt.Sprintf("Your %s on %q did not pass moderation: %s", kind, title, explanation)}
	if !hidden {
		n.Type, n.Title = "contribution_flagged", "Your "+kind+" was flagged for review"
	}
	if postID != "" {
		n.Link = fmt.Sprintf("/%ss/%s", postType, postID)
	}
	// A reply is the canonical contribution: its verdict is the reply.removed / reply.flagged
	// event, naming the reply and its post. A retired legacy kind stays outside the contract.
	if kind == "reply" && postID != "" {
		n.Type = models.NotificationReplyRemoved
		if !hidden {
			n.Type = models.NotificationReplyFlagged
		}
		n.SchemaVersion = models.NotificationSchemaVersion
		n.Subject = models.NotificationSubject{PostID: &postID, ReplyID: &id}
	}
	if authorType == string(models.AuthorTypeHuman) {
		n.UserID = &authorID
	} else {
		n.AgentID = &authorID
	}
	if _, err := m.notify(ctx, n); err != nil {
		m.logger.Error("contribution moderation: notify failed", "kind", kind, "error", err)
	}
}

// SetContributionModerator moderates replies after they are created.
func (h *RepliesHandler) SetContributionModerator(m *ContributionModerator) { h.contribModerator = m }

// SetContributionModerator moderates approaches and progress notes after they are created.
func (h *ProblemsHandler) SetContributionModerator(m *ContributionModerator) { h.contribModerator = m }

// SetContributionModerator moderates answers after they are created.
func (h *QuestionsHandler) SetContributionModerator(m *ContributionModerator) { h.contribModerator = m }

// SetContributionModerator moderates responses after they are created.
func (h *IdeasHandler) SetContributionModerator(m *ContributionModerator) { h.contribModerator = m }

// SetContributionModerator moderates comments after they are created.
func (h *CommentsHandler) SetContributionModerator(m *ContributionModerator) { h.contribModerator = m }
