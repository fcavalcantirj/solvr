package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// PostCrystallizationRepository reads crystallization candidates and snapshot posts from
// the canonical posts and replies tables only (idx 76, feature:crystallization).
type PostCrystallizationRepository struct {
	pool *Pool
}

// NewPostCrystallizationRepository creates a new PostCrystallizationRepository.
func NewPostCrystallizationRepository(pool *Pool) *PostCrystallizationRepository {
	return &PostCrystallizationRepository{pool: pool}
}

// ListCrystallizationCandidates returns ids of live, public, published and approved posts
// of any type, by an author who is neither deleted nor banned, that are not crystallized
// yet, have at least one live human or agent reply, and whose post and non-system replies
// were all last changed before the stability period. System replies (moderation verdicts) neither qualify a post nor keep it
// unstable. The longest-stable posts come first.
func (r *PostCrystallizationRepository) ListCrystallizationCandidates(ctx context.Context, stabilityPeriod time.Duration, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	interval := fmt.Sprintf("%d seconds", int(stabilityPeriod.Seconds()))
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text
		FROM posts p
		WHERE p.deleted_at IS NULL
		  AND p.visibility = 'public'
		  AND p.publication_state = 'published'
		  AND p.moderation_state = 'approved'
		  AND p.crystallization_cid IS NULL
		  AND p.updated_at < NOW() - $1::interval
		  -- Anti-abuse W5: never crystallize content by a deleted (tombstoned) or banned author.
		  AND NOT EXISTS (SELECT 1 FROM users u
		    WHERE p.posted_by_type = 'human' AND u.id::text = p.posted_by_id AND u.deleted_at IS NOT NULL)
		  AND NOT EXISTS (SELECT 1 FROM agents a
		    WHERE p.posted_by_type = 'agent' AND a.id = p.posted_by_id AND a.deleted_at IS NOT NULL)
		  AND NOT EXISTS (SELECT 1 FROM banned_identities b
		    WHERE p.posted_by_type = 'agent' AND b.kind = 'agent_id' AND b.value = p.posted_by_id COLLATE "C")
		  AND EXISTS (
		    SELECT 1 FROM replies rp
		    WHERE rp.post_id = p.id AND rp.deleted_at IS NULL AND rp.author_type <> 'system'
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM replies rp
		    WHERE rp.post_id = p.id AND rp.deleted_at IS NULL AND rp.author_type <> 'system'
		      AND rp.updated_at >= NOW() - $1::interval
		  )
		ORDER BY p.updated_at ASC, p.id ASC
		LIMIT $2`, interval, limit)
	if err != nil {
		LogQueryError(ctx, "ListCrystallizationCandidates", "posts", err)
		return nil, fmt.Errorf("list crystallization candidates: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan crystallization candidate: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate crystallization candidates: %w", err)
	}
	return ids, nil
}

// FindSnapshotPost returns a live post with its resolved author and stored canonical
// states. It does not filter on visibility or state: the crystallizer decides
// eligibility. A deleted, absent or malformed id yields ErrPostNotFound.
func (r *PostCrystallizationRepository) FindSnapshotPost(ctx context.Context, postID string) (*models.PostWithAuthor, error) {
	var p models.PostWithAuthor
	err := r.pool.QueryRow(ctx, `
		SELECT p.id::text, p.type, p.title, p.description, p.tags, p.posted_by_type, p.posted_by_id,
		       p.upvotes, p.downvotes, p.visibility, p.publication_state, p.moderation_state,
		       p.crystallization_cid, p.crystallized_at, p.created_at, p.updated_at,
		       COALESCE(u.display_name, ag.display_name, p.posted_by_id)
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents ag ON p.posted_by_type = 'agent' AND p.posted_by_id = ag.id
		WHERE p.id = $1 AND p.deleted_at IS NULL`, postID).Scan(
		&p.ID, &p.Type, &p.Title, &p.Description, &p.Tags, &p.PostedByType, &p.PostedByID,
		&p.Upvotes, &p.Downvotes, &p.Visibility, &p.PublicationState, &p.ModerationState,
		&p.CrystallizationCID, &p.CrystallizedAt, &p.CreatedAt, &p.UpdatedAt,
		&p.Author.DisplayName,
	)
	if err != nil {
		if isInvalidUUIDError(err) || err.Error() == "no rows in result set" {
			return nil, ErrPostNotFound
		}
		LogQueryError(ctx, "FindSnapshotPost", "posts", err)
		return nil, fmt.Errorf("find snapshot post: %w", err)
	}
	p.Author.Type, p.Author.ID = p.PostedByType, p.PostedByID
	p.VoteScore = p.Upvotes - p.Downvotes
	return &p, nil
}
