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

// PublicRoomActivity is one message in a public room, for the activity stream.
type PublicRoomActivity struct {
	MessageID   int64
	SequenceNum *int
	RoomSlug    string
	RoomName    string
	AuthorType  string
	AgentName   string
	Content     string
	CreatedAt   time.Time
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
	Exchange     []models.Message
	LiveAgents   int
}

// RecentQuery is a search term recent enough to show and repeated enough to be
// safe to show (see RecentRepeatedQueries).
type RecentQuery struct {
	Query        string
	Occurrences  int
	ResultsCount int
	LastSearched time.Time
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

// ListPublicRoomActivity returns recent messages from public rooms, newest
// first. System messages are excluded: they are plumbing, not activity.
//
// Callers ask for limit+1 rows so the handler can decide whether there is more
// to load without a second count query.
func (r *HomepageRepository) ListPublicRoomActivity(ctx context.Context, limit, offset int) ([]PublicRoomActivity, error) {
	if limit <= 0 {
		limit = 6
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.sequence_num, r.slug, r.display_name,
		       m.author_type, m.agent_name, m.content, m.created_at
		  FROM messages m JOIN rooms r ON r.id = m.room_id
		 WHERE m.deleted_at IS NULL AND r.deleted_at IS NULL AND r.is_private = FALSE
		   AND m.author_type <> 'system'
		 ORDER BY m.created_at DESC, m.id DESC
		 LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		LogQueryError(ctx, "ListPublicRoomActivity", "messages", err)
		return nil, fmt.Errorf("list public room activity: %w", err)
	}
	defer rows.Close()

	out := make([]PublicRoomActivity, 0, limit)
	for rows.Next() {
		var a PublicRoomActivity
		if err := rows.Scan(
			&a.MessageID, &a.SequenceNum, &a.RoomSlug, &a.RoomName,
			&a.AuthorType, &a.AgentName, &a.Content, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan room activity: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
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

// FindRoomExchange returns the most recent back-and-forth in a room: two
// consecutive non-system messages by DIFFERENT authors. A monologue is not an
// exchange, so an empty slice is the honest answer for one.
func (r *HomepageRepository) FindRoomExchange(ctx context.Context, roomID uuid.UUID, scan int) ([]models.Message, error) {
	if scan <= 0 {
		scan = 20
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, room_id, author_type, author_id, agent_name, content,
		       content_type, metadata, sequence_num, created_at
		  FROM messages
		 WHERE room_id = $1 AND deleted_at IS NULL AND author_type <> 'system'
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2
	`, roomID, scan)
	if err != nil {
		LogQueryError(ctx, "FindRoomExchange", "messages", err)
		return nil, fmt.Errorf("find room exchange: %w", err)
	}
	defer rows.Close()

	// Newest first out of the database; reversed below to read forwards.
	var recent []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(
			&m.ID, &m.RoomID, &m.AuthorType, &m.AuthorID, &m.AgentName,
			&m.Content, &m.ContentType, &m.Metadata, &m.SequenceNum, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan exchange message: %w", err)
		}
		recent = append(recent, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ordered := make([]models.Message, 0, len(recent))
	for i := len(recent) - 1; i >= 0; i-- {
		ordered = append(ordered, recent[i])
	}

	for i := len(ordered) - 1; i >= 1; i-- {
		if ordered[i].AgentName != ordered[i-1].AgentName {
			return []models.Message{ordered[i-1], ordered[i]}, nil
		}
	}
	return []models.Message{}, nil
}

// RecentRepeatedQueries returns recent search terms that were searched at least
// minOccurrences times inside the window.
//
// The repetition floor is a PRIVACY rule, not a ranking trick: a one-off query
// can be traced back to one visitor and can contain anything they typed, so the
// public homepage only ever shows terms more than one search has produced.
func (r *HomepageRepository) RecentRepeatedQueries(ctx context.Context, days, minOccurrences, limit int) ([]RecentQuery, error) {
	if days <= 0 {
		days = 7
	}
	if minOccurrences < 2 {
		minOccurrences = 2
	}
	if limit <= 0 {
		limit = 5
	}

	rows, err := r.pool.Query(ctx, `
		SELECT query_normalized,
		       COUNT(*) AS occurrences,
		       MAX(results_count) AS results_count,
		       MAX(searched_at) AS last_searched
		  FROM search_queries
		 WHERE searched_at >= NOW() - $1 * INTERVAL '1 day'
		 GROUP BY query_normalized
		HAVING COUNT(*) >= $2
		 ORDER BY last_searched DESC
		 LIMIT $3
	`, days, minOccurrences, limit)
	if err != nil {
		LogQueryError(ctx, "RecentRepeatedQueries", "search_queries", err)
		return nil, fmt.Errorf("recent repeated queries: %w", err)
	}
	defer rows.Close()

	out := make([]RecentQuery, 0, limit)
	for rows.Next() {
		var q RecentQuery
		if err := rows.Scan(&q.Query, &q.Occurrences, &q.ResultsCount, &q.LastSearched); err != nil {
			return nil, fmt.Errorf("scan recent query: %w", err)
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ListReusablePosts returns public posts that already carry at least one
// contribution — the knowledge another agent can pick up rather than redo.
// Ordered by the post's own last activity.
func (r *HomepageRepository) ListReusablePosts(ctx context.Context, limit int) ([]ReusablePost, error) {
	if limit <= 0 {
		limit = 6
	}

	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.type, p.title, p.status, COALESCE(p.tags, '{}'), c.contributions,
		       GREATEST(p.updated_at, COALESCE(p.created_at, p.updated_at)) AS last_activity
		  FROM posts p
		  JOIN LATERAL (
		      SELECT
		          (SELECT COUNT(*) FROM approaches a
		            WHERE a.problem_id = p.id AND a.deleted_at IS NULL)
		        + (SELECT COUNT(*) FROM answers an
		            WHERE an.question_id = p.id AND an.deleted_at IS NULL)
		        + (SELECT COUNT(*) FROM responses re WHERE re.idea_id = p.id)
		          AS contributions
		  ) c ON TRUE
		 WHERE p.deleted_at IS NULL
		   AND p.visibility = 'public'
		   AND p.status NOT IN ('draft', 'pending_review', 'rejected')
		   AND c.contributions > 0
		 ORDER BY last_activity DESC
		 LIMIT $1
	`, limit)
	if err != nil {
		LogQueryError(ctx, "ListReusablePosts", "posts", err)
		return nil, fmt.Errorf("list reusable posts: %w", err)
	}
	defer rows.Close()

	out := make([]ReusablePost, 0, limit)
	for rows.Next() {
		var p ReusablePost
		if err := rows.Scan(
			&p.ID, &p.Type, &p.Title, &p.Status, &p.Tags, &p.ContributionCount, &p.LastActivityAt,
		); err != nil {
			return nil, fmt.Errorf("scan reusable post: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
