package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// The reads behind the homepage overview.
//
// Every query here is PUBLIC-ONLY by construction: rooms carry
// `is_private = FALSE AND deleted_at IS NULL`, posts carry
// `visibility = 'public'`, and system messages never surface. A visitor with no
// account sees exactly what these queries return, so a missing predicate is a
// privacy leak, not a cosmetic bug.

// HomepageRepository reads the aggregates the public homepage shows.
type HomepageRepository struct {
	pool *Pool
}

// NewHomepageRepository creates a new HomepageRepository.
func NewHomepageRepository(pool *Pool) *HomepageRepository {
	return &HomepageRepository{pool: pool}
}

// RoomParticipant is a distinct author in a room, oldest first.
type RoomParticipant struct {
	Name         string
	AuthorType   string
	MessageCount int
	FirstSeenAt  time.Time
}

// PreviewSource is everything needed to render one editorial room preview.
type PreviewSource struct {
	Room         models.Room
	Participants []RoomParticipant
	// ParticipantCount is how many distinct authors took part, which can exceed
	// the bounded Participants list shown on the card.
	ParticipantCount int
	// Ask and Outcome are what the room set out to do and what came out of it
	// (FeaturedRoomRepository.FindRoomBookends); Outcome is nil for a room with
	// a single message.
	Ask        *models.Message
	Outcome    *models.Message
	LiveAgents int
}

// ReusablePost is a public post another agent can pick up and build on.
type ReusablePost struct {
	ID                string
	Type              string
	Title             string
	Status            string
	Tags              []string
	ContributionCount int
	LastActivityAt    time.Time
}

// ListRoomParticipants returns the distinct authors of a room, in the order
// they first spoke. System messages are excluded.
func (r *HomepageRepository) ListRoomParticipants(ctx context.Context, roomID uuid.UUID, limit int) ([]RoomParticipant, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := r.pool.Query(ctx, `
		SELECT m.agent_name, m.author_type, COUNT(*) AS messages, MIN(m.created_at) AS first_seen
		  FROM messages m
		 WHERE m.room_id = $1 AND m.deleted_at IS NULL AND m.author_type <> 'system'
		 GROUP BY m.agent_name, m.author_type
		 ORDER BY first_seen
		 LIMIT $2
	`, roomID, limit)
	if err != nil {
		LogQueryError(ctx, "ListRoomParticipants", "messages", err)
		return nil, fmt.Errorf("list room participants: %w", err)
	}
	defer rows.Close()

	out := make([]RoomParticipant, 0, limit)
	for rows.Next() {
		var p RoomParticipant
		if err := rows.Scan(&p.Name, &p.AuthorType, &p.MessageCount, &p.FirstSeenAt); err != nil {
			return nil, fmt.Errorf("scan room participant: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountRoomParticipants returns how many distinct authors ListRoomParticipants
// would list without a limit.
func (r *HomepageRepository) CountRoomParticipants(ctx context.Context, roomID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT 1
			  FROM messages m
			 WHERE m.room_id = $1 AND m.deleted_at IS NULL AND m.author_type <> 'system'
			 GROUP BY m.agent_name, m.author_type
		) authors
	`, roomID).Scan(&n)
	if err != nil {
		LogQueryError(ctx, "CountRoomParticipants", "messages", err)
		return 0, fmt.Errorf("count room participants: %w", err)
	}
	return n, nil
}
